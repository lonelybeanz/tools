package es

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

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

var ErrBulkRetriable = errors.New("bulk contains retriable item errors")

const (
	saveAttempts              = 3
	saveTimeout               = 20 * time.Second
	httpStatusRequestTimeout  = 408
	httpStatusTooManyRequests = 429
)

type BulkResponseError struct {
	Total      int
	Failed     int
	Retriable  int
	Conflicts  int
	Status     map[int]int
	SampleMsgs []string
}

func (e *BulkResponseError) Error() string {
	return fmt.Sprintf(
		"bulk response contains item errors total=%d failed=%d retriable=%d conflicts=%d status=%v samples=%s",
		e.Total,
		e.Failed,
		e.Retriable,
		e.Conflicts,
		e.Status,
		strings.Join(e.SampleMsgs, " | "),
	)
}

func (e *BulkResponseError) Unwrap() error {
	if e.Retriable > 0 {
		return ErrBulkRetriable
	}
	return nil
}

func SaveAndRetry(indexName string, buffer bytes.Buffer) error {
	return saveAndRetryWithLimit(indexName, buffer, saveAttempts)
}

// 带重试次数限制的内部函数
func saveAndRetryWithLimit(indexName string, buffer bytes.Buffer, attempts int) error {
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		start := time.Now()
		err := SaveToEs(indexName, buffer, saveTimeout)
		if err == nil {
			logSlowBulkSave(indexName, attempt, start)
			return nil
		}
		if errors.Is(err, ErrVersionConflict) || !isRetriableError(err) {
			return err
		}

		lastErr = err
		if attempt == attempts {
			break
		}
		esLogger.Errorf("saveToEs index %s failed, retrying... (%d/%d): %v", indexName, attempt, attempts, err)
	}

	esLogger.Errorf("saveToEs index %s failed after %d attempts: %v", indexName, attempts, lastErr)
	return lastErr
}

func SaveToEs(indexName string, buffer bytes.Buffer, timeOut time.Duration) error {

	// 打印完整的POST语句供后续补偿
	WriteMsgLogBytes(buffer.Bytes())

	// 设置最大超时时间
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()
	bodyBytes, err := DoESRequest(ctx, func(ctx context.Context, client *elasticsearch.Client) (*esapi.Response, error) {
		opts := []func(*esapi.BulkRequest){
			client.Bulk.WithContext(ctx),
		}
		if strings.TrimSpace(indexName) != "" {
			opts = append(opts, client.Bulk.WithIndex(indexName))
		}
		return client.Bulk(
			bytes.NewReader(buffer.Bytes()),
			opts...,
		// EsDB.Bulk.WithRefresh("wait_for"), // ✅ 自动等待可见
		)
	})
	if err != nil {
		return err
	}

	return checkBulkResponse(bodyBytes)
}

func checkBulkResponse(bodyBytes []byte) error {
	var esResp ESBulkResponse
	if err := json.Unmarshal(bodyBytes, &esResp); err != nil {
		esLogger.Errorf("[saveToEs] 解析响应失败: %v", err)
		return fmt.Errorf("saveToEs failed: %v", err)
	}

	if !esResp.Errors {
		return nil
	}

	bulkErr := &BulkResponseError{
		Total:  len(esResp.Items),
		Status: make(map[int]int),
	}
	for _, item := range esResp.Items {
		for action, result := range item {
			if result.Status >= 200 && result.Status < 300 {
				continue
			}
			bulkErr.Failed++
			bulkErr.Status[result.Status]++
			if result.Status == 409 && result.Error.Type == "version_conflict_engine_exception" {
				bulkErr.Conflicts++
			}
			if isRetriableStatus(result.Status) {
				bulkErr.Retriable++
			}
			if len(bulkErr.SampleMsgs) < 3 {
				bulkErr.SampleMsgs = append(bulkErr.SampleMsgs, fmt.Sprintf("%s status=%d type=%s reason=%s", action, result.Status, result.Error.Type, result.Error.Reason))
			}
		}
	}
	if bulkErr.Failed == 0 {
		return fmt.Errorf("bulk response reported errors but no failed items were parsed")
	}
	if bulkErr.Failed == bulkErr.Conflicts {
		return ErrVersionConflict
	}
	return bulkErr
}

func isRetriableStatus(status int) bool {
	return status == httpStatusRequestTimeout || status == httpStatusTooManyRequests || status >= 500
}

func isRetriableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrBulkRetriable) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

func logSlowBulkSave(indexName string, attempt int, start time.Time) {
	elapsed := time.Since(start)
	if elapsed >= slowESRequest || attempt > 1 {
		esLogger.Warnf("saveToEs index %s completed elapsed=%s attempts=%d", indexName, elapsed, attempt)
	}
}
