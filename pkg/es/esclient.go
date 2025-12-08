package es

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/zeromicro/go-zero/core/logx"
)

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

var EsClient *elasticsearch.Client

func EsClientStart(addressesStr, username, password string) {
	InitEs(addressesStr, username, password)
	logx.Info("elasticsearch connect success")
	go func() {
		for {
			if !CheckEsHealth() {
				logx.Error("ES 不可用，尝试重新初始化...")
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

	var err error
	EsClient, err = elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

}

func SafeClose(res *esapi.Response) {
	if res != nil && res.Body != nil {
		io.Copy(io.Discard, res.Body) // ✅ 保证总是关闭
		res.Body.Close()
	}
}

func DoESRequest(ctx context.Context, req func(ctx context.Context) (*esapi.Response, error)) ([]byte, error) {
	var lastErr error

	for i := 0; i < 3; i++ {
		res, err := req(ctx)

		if err != nil {
			SafeClose(res) // Even on error, res might be non-nil with a body that needs closing.
			var netErr net.Error
			// 检查是否是网络错误（包括超时）
			if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
				logx.Errorf("es request network error, retrying... (%d/3): %v", i+1, err)
				lastErr = err
				time.Sleep(time.Duration(i+1) * time.Second) // 增加重试等待时间
				continue
			}
			// 检查是否是EOF错误
			if errors.Is(err, io.EOF) {
				logx.Errorf("es request EOF error, retrying... (%d/3): %v", i+1, err)
				lastErr = err
				time.Sleep(time.Duration(i+1) * time.Second)
				continue
			}
			// 对于其他未知错误，直接返回
			return nil, fmt.Errorf("es request failed: %w", err)
		}

		bodyBytes, readErr := io.ReadAll(res.Body)
		// It's crucial to close the body right after reading
		SafeClose(res)

		if readErr != nil {
			// 读取响应体失败，也认为是一种可重试的网络问题
			logx.Errorf("es response read failed, retrying... (%d/3): %v", i+1, readErr)
			lastErr = readErr
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		if res.IsError() {
			// 对于 5xx 系列的服务器错误，进行重试
			if res.StatusCode >= 500 && res.StatusCode < 600 {
				logx.Errorf("es response server error with status code %d, retrying... (%d/3). Body: %s", res.StatusCode, i+1, string(bodyBytes))
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := EsClient.Ping(EsClient.Ping.WithContext(ctx))
	if err != nil {
		logx.Error("ES Ping 失败:", err)
		return false
	}

	defer SafeClose(res)
	_, err = io.ReadAll(res.Body)
	if err != nil {
		logx.Error("ES Ping 读取响应失败:", err)
		return false
	}

	if res.StatusCode >= 400 {
		logx.Error("ES 返回错误状态码:", res.StatusCode)
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

func SaveToEs(indexName string, buffer bytes.Buffer) error {

	// 打印完整的POST语句供后续补偿
	WriteSaveLog(buffer.String())

	// start := time.Now()

	// for attempt := 1; ; attempt++ {
	// 设置最大超时时间为 5 秒
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	bodyBytes, err := DoESRequest(ctx, func(ctx context.Context) (*esapi.Response, error) {
		return EsClient.Bulk(
			bytes.NewReader(buffer.Bytes()),
			EsClient.Bulk.WithContext(ctx),
			// EsDB.Bulk.WithRefresh("wait_for"), // ✅ 自动等待可见
		)
	})
	cancel() // ✅ 立即 cancel
	if err != nil {
		return err
	}

	var esResp ESBulkResponse
	err = json.Unmarshal(bodyBytes, &esResp)
	if err != nil {
		logx.Errorf("[saveToEs] 解析响应失败: %v", err)
		return fmt.Errorf("saveToEs failed: %v", err)
	}

	if esResp.Errors {
		for _, item := range esResp.Items {
			for _, result := range item {
				if result.Status == 409 && result.Error.Type == "version_conflict_engine_exception" {
					return errors.New("409")
				}
			}
		}
		return fmt.Errorf("saveToEs failed: %v", esResp.Items)
	} else {
		return nil
	}

}
