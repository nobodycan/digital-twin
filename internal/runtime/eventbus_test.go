package runtime

import (
	"testing"
	"time"
)

func TestEventBusPublishesToSubscribers(t *testing.T) {
	bus := NewEventBus()
	events, cancel := bus.Subscribe("conversation.started")
	defer cancel()

	if err := bus.Publish(t.Context(), Event{Topic: "conversation.started", Payload: "conv-1"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case event := <-events:
		if event.Payload != "conv-1" {
			t.Fatalf("unexpected payload %#v", event.Payload)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for event")
	}
}

func TestEventBusDoesNotDeliverOtherTopics(t *testing.T) {
	bus := NewEventBus()
	events, cancel := bus.Subscribe("wanted")
	defer cancel()

	if err := bus.Publish(t.Context(), Event{Topic: "other", Payload: "ignored"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case event := <-events:
		t.Fatalf("unexpected event %#v", event)
	case <-time.After(10 * time.Millisecond):
	}
}
