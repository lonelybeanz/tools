package es

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
)

type TxInfo struct {
	TxHash string `json:"tx_hash"`
}

func TestTime(t *testing.T) {
	// 配置带用户名和密码的 Elasticsearch 客户端
	cfg := elasticsearch.Config{
		Addresses: []string{"http://127.0.0.1:9200"},
		Username:  "admin",
		Password:  "1@in",
		// Logger:    &elastictransport.JSONLogger{Output: os.Stdout}, // 打印请求/响应信息
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}
	res, err := client.Search(
		client.Search.WithContext(context.Background()),
		client.Search.WithIndex("tx_index"),
		client.Search.WithBody(bytes.NewReader([]byte(`{"query": {"match": {"tx_hash.keyword": "`+"0xab28d36f56195c49e454f5b3cc1211f9e4bff2360408f282a8b99d997cca6373"+`"}}}`))),
	)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()

	var esResponse struct {
		Hits struct {
			Hits []struct {
				Source TxInfo `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&esResponse); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
	}

	var txInfoList []TxInfo
	for _, hit := range esResponse.Hits.Hits {
		txInfoList = append(txInfoList, hit.Source)
	}
	for _, txInfo := range txInfoList {
		log.Println(txInfo.TxHash)
	}

}
