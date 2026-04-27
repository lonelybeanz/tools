package es

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckBulkResponseReturnsVersionConflict(t *testing.T) {
	body := []byte(`{"errors":true,"items":[{"index":{"status":409,"error":{"type":"version_conflict_engine_exception","reason":"exists"}}}]}`)

	err := checkBulkResponse(body)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

func TestCheckBulkResponseSummarizesRetriableItemErrors(t *testing.T) {
	body := []byte(`{"errors":true,"items":[{"index":{"status":429,"error":{"type":"es_rejected_execution_exception","reason":"queue full"}}},{"index":{"status":503,"error":{"type":"unavailable","reason":"overloaded"}}}]}`)

	err := checkBulkResponse(body)
	if err == nil {
		t.Fatal("expected bulk response error")
	}
	if !isRetriableError(err) {
		t.Fatalf("expected retriable bulk error, got %v", err)
	}
	if !strings.Contains(err.Error(), "retriable=2") {
		t.Fatalf("expected summarized retriable count, got %v", err)
	}
}
