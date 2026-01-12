package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/lonelybeanz/tools/pkg/log"
)

// StreamRequest 定义了流式分页查询的参数。
type StreamRequest struct {
	Index    string                   `json:"index"`
	PageSize int                      `json:"page_size"`        // 页面大小，建议设置一个最大值，例如 1000
	Filters  map[string]interface{}   `json:"filters"`          // 筛选结构
	Sort     []map[string]interface{} `json:"sort"`             // 排序结构
	After    []interface{}            `json:"after,omitempty"`  // 用于获取下一页的游标 (search_after)
	Before   []interface{}            `json:"before,omitempty"` // 用于获取上一页的游标
}

// StreamResponse 保存了流式分页查询的结果。
type StreamResponse struct {
	Total     int64         `json:"total"`
	List      []interface{} `json:"list"`
	StartSort []interface{} `json:"start_sort,omitempty"` // 结果集中第一项的排序值
	EndSort   []interface{} `json:"end_sort,omitempty"`   // 结果集中最后一项的排序值
	HasMore   bool          `json:"has_more"`             // 指示当前方向是否还存在更多数据
}

// reverseSortOrder 反转每个排序字段的方向 (例如, "asc" 到 "desc")。
// 支持 {"field": "desc"} 和 {"field": {"order": "desc"}} 两种格式。
func reverseSortOrder(sorts []map[string]interface{}) []map[string]interface{} {
	reversed := make([]map[string]interface{}, len(sorts))
	for i, s := range sorts {
		for k, v := range s {
			newSort := make(map[string]interface{})
			// 检查详细格式: {"field": {"order": "desc"}}
			if orderMap, ok := v.(map[string]interface{}); ok {
				originalOrder := "asc" // 默认原始排序为 asc
				if order, ok := orderMap["order"].(string); ok {
					originalOrder = order
				}
				newOrder := "desc"
				if originalOrder == "desc" {
					newOrder = "asc"
				}
				// 复制其他可能的排序参数
				newOrderMap := make(map[string]interface{})
				for key, val := range orderMap {
					newOrderMap[key] = val
				}
				newOrderMap["order"] = newOrder
				newSort[k] = newOrderMap

			} else if order, ok := v.(string); ok { // 检查简化格式: {"field": "desc"}
				newOrder := "desc"
				if order == "desc" {
					newOrder = "asc"
				}
				newSort[k] = newOrder
			} else {
				// 如果格式未知，则保持原样
				newSort = s
			}
			reversed[i] = newSort
			break // 每个顶层 map 只处理一个键值对
		}
	}
	return reversed
}

// QueryStream 使用 Elasticsearch 的 search_after 实现基于游标的分页（“流式”）。
// 它支持获取下一页 (使用 'After') 和上一页 (使用 'Before')。
func QueryStream(ctx context.Context, req StreamRequest) (*StreamResponse, error) {
	if len(req.After) > 0 && len(req.Before) > 0 {
		return nil, fmt.Errorf("'After' and 'Before' cannot be used simultaneously")
	}

	// 增加 PageSize 的校验，防止一次性请求过多数据导致 ES 压力过大
	if req.PageSize <= 0 || req.PageSize > 1000 {
		req.PageSize = 20 // 设置一个合理的默认值或最大值
	}

	// 1. 根据方向确定排序顺序和 search_after 的值
	querySort := req.Sort
	searchAfter := req.After
	isPagingBackwards := len(req.Before) > 0

	if isPagingBackwards {
		// 查询上一页时，反转排序顺序
		querySort = reverseSortOrder(req.Sort)
		searchAfter = req.Before
	}

	// 2. 构建查询 Body
	query := map[string]interface{}{
		"size":             req.PageSize + 1, // 多取一条数据用于判断 HasMore
		"track_total_hits": true,
		"query":            req.Filters,
		"sort":             querySort,
	}

	if len(searchAfter) > 0 {
		query["search_after"] = searchAfter
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("encoding query failed: %w", err)
	}

	log.Debugf("QueryStream body: %s", buf.String())

	// 3. 执行 ES 请求
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

	// 4. 解析响应
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
		return nil, fmt.Errorf("unmarshaling es response failed: %w", err)
	}

	// 5. 如果是向后翻页，则在内存中反转结果集
	hits := esRes.Hits.Hits
	hasMore := len(hits) > req.PageSize

	if hasMore {
		// 如果获取到的数据大于请求的 PageSize，说明有更多数据
		// 移除额外获取的那一条
		hits = hits[:req.PageSize]
	}

	if isPagingBackwards {
		// 使用 slices.Reverse 可以使代码更简洁 (Go 1.21+)
		slices.Reverse(hits)
		// for Go 1.20 and earlier:
		// slicesReverse(hits)
	}

	// 6. 组装最终响应
	resp := &StreamResponse{
		Total: esRes.Hits.Total.Value,
		List:  make([]interface{}, 0),
	}

	if len(hits) == 0 {
		return resp, nil
	}

	for _, h := range hits {
		resp.List = append(resp.List, h.Source)
	}
	resp.StartSort = hits[0].Sort
	resp.EndSort = hits[len(hits)-1].Sort
	resp.HasMore = hasMore

	return resp, nil
}

// slicesReverse 是一个兼容旧版本 Go 的泛型反转函数
// func slicesReverse[S ~[]E, E any](s S) {
// 	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
// 		s[i], s[j] = s[j], s[i]
// 	}
// }
