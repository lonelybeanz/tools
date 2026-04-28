package es

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
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

func TestDefaultSaveTimeoutExceedsTransportHeaderTimeout(t *testing.T) {
	if saveTimeout <= esResponseHeaderTimeout {
		t.Fatalf("saveTimeout %s must exceed esResponseHeaderTimeout %s", saveTimeout, esResponseHeaderTimeout)
	}
}

func TestSaveAndRetryWithLimitLogsRecoveredRetryAsWarning(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	oldLogger := esLogger
	esLogger = zap.New(core).Sugar()
	defer func() { esLogger = oldLogger }()

	oldSaveToEs := saveToEs
	attempts := 0
	saveToEs = func(indexName string, buffer bytes.Buffer, timeOut time.Duration) error {
		attempts++
		if attempts == 1 {
			return context.DeadlineExceeded
		}
		return nil
	}
	defer func() { saveToEs = oldSaveToEs }()

	err := saveAndRetryWithLimit("idx", *bytes.NewBufferString(`{"index":{}}`), 2)
	if err != nil {
		t.Fatalf("expected retry to recover, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}

	for _, entry := range observed.All() {
		if entry.Level >= zapcore.ErrorLevel {
			t.Fatalf("expected recovered retry to avoid error logs, got %s: %s", entry.Level, entry.Message)
		}
	}
	if observed.FilterMessageSnippet("retrying").Len() != 1 {
		t.Fatalf("expected one retry warning, got logs: %#v", observed.All())
	}
	if observed.FilterMessageSnippet("completed").Len() != 1 {
		t.Fatalf("expected one recovered completion warning, got logs: %#v", observed.All())
	}
}
