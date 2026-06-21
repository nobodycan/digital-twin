package presentation

import (
	"context"
	"testing"
	"time"
)

func TestInterruptionControllerCancelsActiveRequestAndEmitsEvent(t *testing.T) {
	controller := NewInterruptionController(fixedClock(time.Date(2026, 6, 17, 18, 0, 0, 0, time.UTC)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	active := controller.Begin(ctx, EventContext{
		TenantID:       "tenant-1",
		UserID:         "user-1",
		ConversationID: "conv-1",
		RequestID:      "req-old",
	})

	event, ok := controller.Interrupt("conv-1", "user sent a new message")
	if !ok {
		t.Fatalf("expected active request to be interrupted")
	}
	if active.Err() == nil {
		t.Fatalf("expected active context to be cancelled")
	}
	if event.Name != EventInterrupted {
		t.Fatalf("event name = %q, want %q", event.Name, EventInterrupted)
	}
	if event.RequestID != "req-old" || event.Sequence != 1 {
		t.Fatalf("event identity = %#v", event)
	}
	if event.Payload["reason"] != "user sent a new message" {
		t.Fatalf("payload = %#v", event.Payload)
	}
}

func TestInterruptionControllerAllowsNewRequestAfterInterrupt(t *testing.T) {
	controller := NewInterruptionController(nil)
	first, firstCancel := context.WithCancel(context.Background())
	defer firstCancel()
	controller.Begin(first, EventContext{ConversationID: "conv-1", RequestID: "req-old"})
	if _, ok := controller.Interrupt("conv-1", "new input"); !ok {
		t.Fatalf("expected first request to be interrupted")
	}

	second, secondCancel := context.WithCancel(context.Background())
	defer secondCancel()
	active := controller.Begin(second, EventContext{ConversationID: "conv-1", RequestID: "req-new"})

	select {
	case <-active.Done():
		t.Fatalf("new request should not inherit cancellation from old request")
	default:
	}
}
