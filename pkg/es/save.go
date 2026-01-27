package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func SaveAndRetry(indexName string, buffer bytes.Buffer) error {
	return saveAndRetryWithLimit(indexName, buffer, 5) // 默认最大重试5次
}

// 带重试次数限制的内部函数
func saveAndRetryWithLimit(indexName string, buffer bytes.Buffer, retriesLeft int) error {
	timeOut := time.Duration(60-retriesLeft*10) * time.Second
	err := SaveToEs(indexName, buffer, timeOut)
	if err != nil {
		if err == ErrVersionConflict {
			esLogger.Error("版本冲突,请检查数据是否已存在")
			return err
		}
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
