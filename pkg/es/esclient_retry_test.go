package es

import (
	"context"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

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
