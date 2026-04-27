package es

import (
	"testing"
	"time"
)

func TestTryQueueMsgLogDoesNotBlockWhenQueueIsFull(t *testing.T) {
	queue := make(chan []byte)

	done := make(chan bool, 1)
	go func() {
		done <- tryQueueMsgLogBytes(queue, nil, []byte("bulk body"))
	}()

	select {
	case queued := <-done:
		if queued {
			t.Fatal("expected full queue enqueue to report false")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("tryQueueMsgLog blocked on a full queue")
	}
}

func TestTryQueueMsgLogBytesQueuesCopiedPayload(t *testing.T) {
	queue := make(chan []byte, 1)
	slots := make(chan struct{}, 1)
	slots <- struct{}{}

	payload := []byte("bulk body")
	if !tryQueueMsgLogBytes(queue, slots, payload) {
		t.Fatal("expected payload to be queued")
	}

	payload[0] = 'B'
	queued := <-queue
	if string(queued) != "bulk body" {
		t.Fatalf("expected queued payload to be copied, got %q", string(queued))
	}
}
