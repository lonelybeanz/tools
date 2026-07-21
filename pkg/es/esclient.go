package es

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esapi"
)

var (
	EsClient *elasticsearch.Client
	esMutex  sync.RWMutex
)

var ErrVersionConflict = errors.New("version conflict detected (409)")

const (
	esRequestAttempts       = 3
	esResponseHeaderTimeout = 30 * time.Second
	slowESRequest           = 10 * time.Second
)

var dialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}
var sharedTransport = &http.Transport{
	DialContext:           dialer.DialContext,
	MaxIdleConns:          256,
	MaxIdleConnsPerHost:   256,
	MaxConnsPerHost:       256,
	IdleConnTimeout:       90 * time.Second,
	ResponseHeaderTimeout: esResponseHeaderTimeout, // 控制服务器响应头的最大等待时间
	ExpectContinueTimeout: 1 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true, // 跳过证书验证（⚠️ 仅限开发环境）
	},
}

func EsClientStart(addressesStr, username, password string) {
	InitEs(addressesStr, username, password)
	go func() {
		for {
			if !CheckEsHealth() {
				esLogger.Error("ES不可用,尝试重新初始化...")
				InitEs(addressesStr, username, password)
			}
			time.Sleep(30 * time.Second)
		}
	}()
}

func InitEs(addressesStr, username, password string) {
	addresses := splitAddresses(addressesStr)
	cfg := elasticsearch.Config{
		Addresses: addresses,
		Username:  username,
		Password:  password,
		Transport: sharedTransport,
	}

	newClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		esLogger.Errorf("Error creating the client: %s", err)
		return
	}

	esMutex.Lock()
	defer esMutex.Unlock()
	// 如果旧的 EsClient 存在，理论上应该先安全关闭它的连接，但这比较复杂。
	// go-elasticsearch 的 transport 是共享的，所以替换 Client 实例通常是安全的。
	EsClient = newClient

}

func splitAddresses(addressesStr string) []string {
	parts := strings.Split(addressesStr, ",")
	addresses := make([]string, 0, len(parts))
	for _, part := range parts {
		address := strings.TrimSpace(part)
		if address != "" {
			addresses = append(addresses, address)
		}
	}
	return addresses
}

func SafeClose(res *esapi.Response) {
	if res != nil && res.Body != nil {
		io.Copy(io.Discard, res.Body) // ✅ 保证总是关闭
		res.Body.Close()
	}
}

func DoESRequest(ctx context.Context, req func(ctx context.Context, client *elasticsearch.Client) (*esapi.Response, error)) ([]byte, error) {
	esMutex.RLock()
	if EsClient == nil {
		esMutex.RUnlock()
		return nil, errors.New("es client is not initialized")
	}
	client := EsClient
	esMutex.RUnlock() // 拿到 client 后即可解锁

	var lastErr error
	start := time.Now()
	for i := 0; i < esRequestAttempts; i++ {
		// ✅ 在每次循环开始时检查 context
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		res, err := req(ctx, client)

		if err != nil {
			SafeClose(res) // Even on error, res might be non-nil with a body that needs closing.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			var netErr net.Error
			// 检查是否是网络错误（包括超时）
			if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
				esLogger.Warnf("es request network error, retrying... (%d/%d): %v", i+1, esRequestAttempts, err)
				lastErr = err
				if err := sleepWithContext(ctx, retryBackoff(i)); err != nil {
					return nil, err
				}
				continue
			}
			// 检查是否是EOF错误
			if errors.Is(err, io.EOF) {
				esLogger.Warnf("es request EOF error, retrying... (%d/%d): %v", i+1, esRequestAttempts, err)
				lastErr = err
				if err := sleepWithContext(ctx, retryBackoff(i)); err != nil {
					return nil, err
				}
				continue
			}
			// 对于其他未知错误，直接返回
			return nil, fmt.Errorf("es request failed: %w", err)
		}
		if res == nil {
			return nil, errors.New("es request returned nil response")
		}
		if res.Body == nil {
			return nil, errors.New("es request returned nil response body")
		}

		bodyBytes, readErr := io.ReadAll(res.Body)
		// It's crucial to close the body right after reading
		SafeClose(res)

		if readErr != nil {
			// 读取响应体失败，也认为是一种可重试的网络问题
			esLogger.Warnf("es response read failed, retrying... (%d/%d): %v", i+1, esRequestAttempts, readErr)
			lastErr = readErr
			if err := sleepWithContext(ctx, retryBackoff(i)); err != nil {
				return nil, err
			}
			continue
		}

		if res.IsError() {
			// 对于 5xx 系列的服务器错误，进行重试
			if res.StatusCode >= 500 && res.StatusCode < 600 {
				esLogger.Warnf("es response server error with status code %d, retrying... (%d/%d). Body: %s", res.StatusCode, i+1, esRequestAttempts, string(bodyBytes))
				lastErr = fmt.Errorf("es response error with status code %d: %s", res.StatusCode, string(bodyBytes))
				if err := sleepWithContext(ctx, retryBackoff(i)); err != nil {
					return nil, err
				}
				continue
			}
			// 对于 4xx 客户端错误或其他错误，直接返回
			return nil, fmt.Errorf("es response error: %s", string(bodyBytes))
		}

		// 请求成功，返回结果
		logSlowESRequest(start, i+1)
		return bodyBytes, nil
	}

	// 循环结束仍然失败，返回最后一次的错误
	if lastErr != nil {
		return nil, fmt.Errorf("es request failed after %d retries: %w", esRequestAttempts, lastErr)
	}

	return nil, fmt.Errorf("es request failed after %d retries without a specific error", esRequestAttempts)
}

func retryBackoff(attempt int) time.Duration {
	return time.Duration(attempt+1) * time.Second
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func logSlowESRequest(start time.Time, attempts int) {
	elapsed := time.Since(start)
	if elapsed >= slowESRequest {
		esLogger.Warnf("es request completed slowly elapsed=%s attempts=%d", elapsed, attempts)
	}
}

func CheckEsHealth() bool {
	esMutex.RLock()
	if EsClient == nil {
		esMutex.RUnlock()
		return false
	}
	// 捕获客户端以在解锁后使用
	client := EsClient
	esMutex.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := client.Ping(client.Ping.WithContext(ctx))
	if err != nil {
		esLogger.Error("ES Ping 失败:", err)
		return false
	}

	defer SafeClose(res)
	if res.StatusCode >= 400 {
		esLogger.Error("ES 返回错误状态码:", res.StatusCode)
		return false
	}
	return true
}
