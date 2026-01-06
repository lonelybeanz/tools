package es

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

var (
	EsClient *elasticsearch.Client
	esMutex  sync.RWMutex
)

var ErrVersionConflict = errors.New("version conflict detected (409)")

//	dialer := &net.Dialer{
//		Timeout:   30 * time.Second,
//		KeepAlive: 30 * time.Second,
//	}
var sharedTransport = &http.Transport{
	// DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
	// 	conn, err := dialer.DialContext(ctx, network, addr)
	// 	if err == nil {
	// 		fmt.Println("DialContext: new connection to", addr)
	// 	}
	// 	return conn, err
	// },
	MaxIdleConns:        200,
	MaxIdleConnsPerHost: 200,
	// MaxConnsPerHost:       300,
	IdleConnTimeout:       90 * time.Second,
	ResponseHeaderTimeout: 10 * time.Second, // 控制服务器响应的最大等待时间
	ExpectContinueTimeout: 1 * time.Second,
	// DisableKeepAlives: true,
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
	addresses := strings.Split(addressesStr, ",")
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
	for i := 0; i < 3; i++ {
		// ✅ 在每次循环开始时检查 context
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		res, err := req(ctx, client)

		if err != nil {
			SafeClose(res) // Even on error, res might be non-nil with a body that needs closing.
			var netErr net.Error
			// 检查是否是网络错误（包括超时）
			if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
				esLogger.Errorf("⚠️ es request network error, retrying... (%d/3): %v", i+1, err)
				lastErr = err
				time.Sleep(time.Duration(i+1) * time.Second) // 增加重试等待时间
				continue
			}
			// 检查是否是EOF错误
			if errors.Is(err, io.EOF) {
				esLogger.Errorf("⚠️ es request EOF error, retrying... (%d/3): %v", i+1, err)
				lastErr = err
				time.Sleep(time.Duration(i+1) * time.Second)
				continue
			}
			// 对于其他未知错误，直接返回
			return nil, fmt.Errorf("❌ es request failed: %w", err)
		}

		bodyBytes, readErr := io.ReadAll(res.Body)
		// It's crucial to close the body right after reading
		SafeClose(res)

		if readErr != nil {
			// 读取响应体失败，也认为是一种可重试的网络问题
			esLogger.Errorf("es response read failed, retrying... (%d/3): %v", i+1, readErr)
			lastErr = readErr
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		if res.IsError() {
			// 对于 5xx 系列的服务器错误，进行重试
			if res.StatusCode >= 500 && res.StatusCode < 600 {
				esLogger.Errorf("es response server error with status code %d, retrying... (%d/3). Body: %s", res.StatusCode, i+1, string(bodyBytes))
				lastErr = fmt.Errorf("es response error with status code %d: %s", res.StatusCode, string(bodyBytes))
				time.Sleep(time.Duration(i+1) * time.Second)
				continue
			}
			// 对于 4xx 客户端错误或其他错误，直接返回
			return nil, fmt.Errorf("es response error: %s", string(bodyBytes))
		}

		// 请求成功，返回结果
		return bodyBytes, nil
	}

	// 循环结束仍然失败，返回最后一次的错误
	if lastErr != nil {
		return nil, fmt.Errorf("es request failed after 3 retries: %w", lastErr)
	}

	return nil, fmt.Errorf("es request failed after 3 retries without a specific error")
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

type ESBulkResponse struct {
	Errors bool `json:"errors"`
	Items  []map[string]struct {
		Status int `json:"status"`
		Error  struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error,omitempty"`
	} `json:"items"`
}

func SaveAndRetry(indexName string, buffer bytes.Buffer) error {
	return saveAndRetryWithLimit(indexName, buffer, 5) // 默认最大重试5次
}

// 带重试次数限制的内部函数
func saveAndRetryWithLimit(indexName string, buffer bytes.Buffer, retriesLeft int) error {
	timeOut := time.Duration(60-retriesLeft*10) * time.Second
	err := SaveToEs(indexName, buffer, timeOut)
	if err != nil {
		esLogger.Error("保存数据到ES失败:", err)

		// 检查是否还有重试次数
		if retriesLeft <= 0 {
			esLogger.Error("保存数据到ES失败,已达到最大重试次数")
			return err
		}

		// 递归调用，减少重试次数
		return saveAndRetryWithLimit(indexName, buffer, retriesLeft-1)
	}
	return nil
}

func SaveToEs(indexName string, buffer bytes.Buffer, timeOut time.Duration) error {

	// 打印完整的POST语句供后续补偿
	WriteMsgLog(buffer.String())

	// 设置最大超时时间
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()
	bodyBytes, err := DoESRequest(ctx, func(ctx context.Context, client *elasticsearch.Client) (*esapi.Response, error) {
		return client.Bulk(
			bytes.NewReader(buffer.Bytes()),
			client.Bulk.WithContext(ctx),
			// EsDB.Bulk.WithRefresh("wait_for"), // ✅ 自动等待可见
		)
	})
	if err != nil {
		return err
	}

	var esResp ESBulkResponse
	err = json.Unmarshal(bodyBytes, &esResp)
	if err != nil {
		esLogger.Errorf("[saveToEs] 解析响应失败: %v", err)
		return fmt.Errorf("saveToEs failed: %v", err)
	}

	if esResp.Errors {
		for _, item := range esResp.Items {
			for _, result := range item {
				if result.Status == 409 && result.Error.Type == "version_conflict_engine_exception" {
					return ErrVersionConflict
				}
			}
		}
		return fmt.Errorf("saveToEs failed: %v", esResp.Items)
	} else {
		return nil
	}

}
