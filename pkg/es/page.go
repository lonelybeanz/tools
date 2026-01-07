package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/lonelybeanz/tools/pkg/log"
)

type PageDirection int

const (
	PageFirst PageDirection = iota
	PageNext
	PagePrev
	PageLast
)

type PagerRequest struct {
	Index      string        `json:"index"`
	Size       int           `json:"size"`
	Filters    []interface{} `json:"filters"`   // 筛选结构
	SortTime   string        `json:"sort_time"` // 排序结构
	Direction  PageDirection `json:"direction"`
	AfterValue []interface{} `json:"after_value"` // 只有 Next 和 Prev 需要

}

type PagerResponse struct {
	Total     int64         `json:"total"`
	List      []interface{} `json:"list"`
	FirstSort []interface{} `json:"first_sort"`
	LastSort  []interface{} `json:"last_sort"`
	HasMore   bool          `json:"has_more"`
}

// QueryFlow 封装了类似 BscScan 的分页逻辑
func QueryFlow(ctx context.Context, req PagerRequest) (*PagerResponse, error) {
	// 1. 确定排序方向
	// 默认业务逻辑是倒序（最新在最前）
	mainOrder := "desc"
	if req.Direction == PagePrev || req.Direction == PageLast {
		mainOrder = "asc"
	}

	// 2. 构建 Search Body
	query := map[string]interface{}{
		"size":             req.Size,
		"track_total_hits": true, // 获取总数
		"query":            req.Filters,
		"sort": []map[string]interface{}{
			{req.SortTime: mainOrder},
			{"_id": mainOrder}, // 必须加 _id 保证排序唯一
		},
	}

	// 3. 处理 search_after
	if (req.Direction == PageNext || req.Direction == PagePrev) && len(req.AfterValue) > 0 {
		query["search_after"] = req.AfterValue
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("encoding query failed: %w", err)
	}

	log.Debugf("QueryFlow: %s", buf.String())

	// 4. 执行请求
	resByte, err := DoESRequest(ctx, func(ctx context.Context, client *elasticsearch.Client) (*esapi.Response, error) {
		return client.Search(
			client.Search.WithContext(ctx),
			client.Search.WithIndex(req.Index),
			client.Search.WithBody(&buf),
		)
	})
	if err != nil {
		return nil, err
	}

	// 5. 解析结果
	var esRes struct {
		Hits struct {
			Total struct{ Value int64 } `json:"total"`
			Hits  []struct {
				Source interface{}   `json:"_source"`
				Sort   []interface{} `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(resByte, &esRes); err != nil {
		return nil, err
	}

	// 6. 处理内存反转
	// 如果是向上翻页或最后一页，ES 返回的是升序，我们需要转回降序给前端
	hits := esRes.Hits.Hits
	if req.Direction == PagePrev || req.Direction == PageLast {
		for i, j := 0, len(hits)-1; i < j; i, j = i+1, j-1 {
			hits[i], hits[j] = hits[j], hits[i]
		}
	}

	// 7. 组装返回结果
	resp := &PagerResponse{
		Total: esRes.Hits.Total.Value,
		List:  make([]interface{}, 0),
	}

	if len(hits) > 0 {
		for _, h := range hits {
			resp.List = append(resp.List, h.Source)
		}
		resp.FirstSort = hits[0].Sort
		resp.LastSort = hits[len(hits)-1].Sort
		resp.HasMore = len(hits) >= req.Size
	}

	return resp, nil
}
