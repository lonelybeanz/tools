package es

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esapi"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestSleepWithContextReturnsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := sleepWithContext(ctx, time.Second)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("sleepWithContext waited too long after cancellation: %s", elapsed)
	}
}

func TestDoESRequestDoesNotRetryAfterContextCancellation(t *testing.T) {
	esMutex.Lock()
	oldClient := EsClient
	EsClient = &elasticsearch.Client{}
	esMutex.Unlock()
	defer func() {
		esMutex.Lock()
		EsClient = oldClient
		esMutex.Unlock()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	_, err := DoESRequest(ctx, func(ctx context.Context, client *elasticsearch.Client) (*esapi.Response, error) {
		attempts++
		cancel()
		return nil, context.Canceled
	})

	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected one attempt after context cancellation, got %d", attempts)
	}
}

func TestDoESRequestLogsRecoveredRetryAsWarning(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	oldLogger := esLogger
	esLogger = zap.New(core).Sugar()
	defer func() { esLogger = oldLogger }()

	esMutex.Lock()
	oldClient := EsClient
	EsClient = &elasticsearch.Client{}
	esMutex.Unlock()
	defer func() {
		esMutex.Lock()
		EsClient = oldClient
		esMutex.Unlock()
	}()

	attempts := 0
	_, err := DoESRequest(context.Background(), func(ctx context.Context, client *elasticsearch.Client) (*esapi.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, timeoutError{}
		}
		return &esapi.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})
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
}
