package es

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v9"
)

func TestQueryStreamHandlesNullSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_, _ = io.WriteString(w, `{"hits":{"total":{"value":1},"hits":[{"_id":"1","_source":null,"sort":[1]}]}}`)
	}))
	defer server.Close()

	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	esMutex.Lock()
	oldClient := EsClient
	EsClient = client
	esMutex.Unlock()
	defer func() {
		esMutex.Lock()
		EsClient = oldClient
		esMutex.Unlock()
	}()

	_, err = QueryStream(context.Background(), StreamRequest{
		Index:    "idx",
		PageSize: 20,
		Filters:  map[string]interface{}{"match_all": map[string]interface{}{}},
		Sort:     []map[string]interface{}{{"_id": "asc"}},
	})
	if err == nil || !strings.Contains(err.Error(), "missing _source") {
		t.Fatalf("expected missing _source error, got %v", err)
	}
}
