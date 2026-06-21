package presentation

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPresentationEventRoundTripsRequiredMetadataAndPayload(t *testing.T) {
	occurredAt := time.Date(2026, 6, 17, 14, 30, 0, 0, time.UTC)
	event := NewEvent(EventAssistantTextDelta, EventContext{
		TenantID:       "tenant-1",
		UserID:         "user-1",
		ConversationID: "conv-1",
		RequestID:      "req-1",
		Sequence:       7,
		OccurredAt:     occurredAt,
	}, map[string]any{
		"text": "hello",
	}, map[string]any{
		"agent": "persona-agent",
	})

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if decoded.Name != EventAssistantTextDelta {
		t.Fatalf("name = %q, want %q", decoded.Name, EventAssistantTextDelta)
	}
	if decoded.TenantID != "tenant-1" || decoded.UserID != "user-1" || decoded.ConversationID != "conv-1" || decoded.RequestID != "req-1" {
		t.Fatalf("decoded identity metadata = %#v", decoded)
	}
	if decoded.Sequence != 7 {
		t.Fatalf("sequence = %d, want 7", decoded.Sequence)
	}
	if !decoded.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", decoded.OccurredAt, occurredAt)
	}
	if decoded.Payload["text"] != "hello" {
		t.Fatalf("payload text = %v, want hello", decoded.Payload["text"])
	}
	if decoded.Metadata["agent"] != "persona-agent" {
		t.Fatalf("metadata agent = %v, want persona-agent", decoded.Metadata["agent"])
	}
}

func TestOrderedEventsReturnsCopySortedBySequence(t *testing.T) {
	events := []Event{
		{Name: EventDone, Sequence: 3},
		{Name: EventConversationStarted, Sequence: 1},
		{Name: EventSubtitle, Sequence: 2},
	}

	ordered := OrderedEvents(events)

	if got := []EventName{ordered[0].Name, ordered[1].Name, ordered[2].Name}; got[0] != EventConversationStarted || got[1] != EventSubtitle || got[2] != EventDone {
		t.Fatalf("ordered event names = %#v", got)
	}
	if events[0].Name != EventDone {
		t.Fatalf("OrderedEvents mutated input slice")
	}
}

func TestNewErrorEventCarriesActionableProblemCauseAndFix(t *testing.T) {
	event := NewErrorEvent(EventContext{
		TenantID:       "tenant-1",
		UserID:         "user-1",
		ConversationID: "conv-1",
		RequestID:      "req-1",
		Sequence:       9,
		OccurredAt:     time.Date(2026, 6, 17, 15, 0, 0, 0, time.UTC),
	}, "tts unavailable", "mock tts returned an error", "continue with text or retry voice")

	if event.Name != EventError {
		t.Fatalf("name = %q, want %q", event.Name, EventError)
	}
	if event.Payload["problem"] != "tts unavailable" {
		t.Fatalf("problem = %v", event.Payload["problem"])
	}
	if event.Payload["cause"] != "mock tts returned an error" {
		t.Fatalf("cause = %v", event.Payload["cause"])
	}
	if event.Payload["fix"] != "continue with text or retry voice" {
		t.Fatalf("fix = %v", event.Payload["fix"])
	}
}

func TestPhase4EventNamesAreStable(t *testing.T) {
	tests := map[EventName]string{
		EventConversationStarted: "conversation_started",
		EventUserText:            "user_text",
		EventASRPartial:          "asr_partial",
		EventASRFinal:            "asr_final",
		EventAssistantTextDelta:  "assistant_text_delta",
		EventSubtitle:            "subtitle",
		EventAudioChunk:          "audio_chunk",
		EventAvatarState:         "avatar_state",
		EventToolStatus:          "tool_status",
		EventInterrupted:         "interrupted",
		EventError:               "error",
		EventDone:                "done",
	}

	for got, want := range tests {
		if string(got) != want {
			t.Fatalf("event name = %q, want %q", got, want)
		}
	}
}
