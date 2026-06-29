package presentation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/avatar"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/voice"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestAdapterConvertsAgentResultToPresentationEvents(t *testing.T) {
	machine, err := avatar.NewStateMachine(avatar.Manifest{
		Supported:     []avatar.State{avatar.StateIdle, avatar.StateSpeaking},
		FallbackState: avatar.StateIdle,
	})
	if err != nil {
		t.Fatalf("NewStateMachine returned error: %v", err)
	}
	adapter := Adapter{
		TTS:    voice.MockTTSClient{},
		Avatar: machine,
		Clock:  fixedClock(time.Date(2026, 6, 17, 17, 0, 0, 0, time.UTC)),
	}

	events, err := adapter.Adapt(AdaptRequest{
		Context: EventContext{
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			RequestID:      "req-1",
		},
		Result: types.AgentResult{
			AgentName: "persona-agent",
			Message: types.Message{
				Role:    types.RoleAssistant,
				Content: "Hello there.",
			},
			Confidence: 0.9,
			Metadata:   types.Metadata{"tool": "none"},
		},
	})
	if err != nil {
		t.Fatalf("Adapt returned error: %v", err)
	}

	names := eventNames(events)
	want := []EventName{EventConversationStarted, EventAssistantTextDelta, EventSubtitle, EventAudioChunk, EventAvatarState, EventDone}
	if len(names) != len(want) {
		t.Fatalf("event names = %#v, want %#v", names, want)
	}
	for index := range want {
		if names[index] != want[index] {
			t.Fatalf("event names = %#v, want %#v", names, want)
		}
		if events[index].Sequence != index+1 {
			t.Fatalf("event %d sequence = %d, want %d", index, events[index].Sequence, index+1)
		}
	}
	if events[1].Payload["text"] != "Hello there." {
		t.Fatalf("assistant text payload = %#v", events[1].Payload)
	}
	if events[4].Payload["state"] != string(avatar.StateSpeaking) {
		t.Fatalf("avatar payload = %#v", events[4].Payload)
	}
}

func TestAdapterEmitsErrorEventWhenTTSFailsAndKeepsTextEvents(t *testing.T) {
	machine, err := avatar.NewStateMachine(avatar.Manifest{
		Supported:     []avatar.State{avatar.StateIdle, avatar.StateSpeaking},
		FallbackState: avatar.StateIdle,
	})
	if err != nil {
		t.Fatalf("NewStateMachine returned error: %v", err)
	}
	adapter := Adapter{
		TTS:    failingTTS{},
		Avatar: machine,
		Clock:  fixedClock(time.Date(2026, 6, 17, 17, 30, 0, 0, time.UTC)),
	}

	events, err := adapter.Adapt(AdaptRequest{
		Context: EventContext{TenantID: "tenant-1", UserID: "user-1", ConversationID: "conv-1", RequestID: "req-1"},
		Result:  types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "Hello there."}},
	})
	if err != nil {
		t.Fatalf("Adapt returned error: %v", err)
	}

	names := eventNames(events)
	if names[1] != EventAssistantTextDelta || names[2] != EventSubtitle {
		t.Fatalf("expected text and subtitle events before TTS failure, got %#v", names)
	}
	if names[3] != EventError {
		t.Fatalf("event names = %#v, want error event at index 3", names)
	}
	if events[3].Payload["problem"] != "tts unavailable" {
		t.Fatalf("error payload = %#v", events[3].Payload)
	}
	if names[len(names)-1] != EventDone {
		t.Fatalf("last event = %q, want done", names[len(names)-1])
	}
}

func TestAdapterStreamSinkMapsRuntimeDeltasAndTerminalSuccess(t *testing.T) {
	machine, err := avatar.NewStateMachine(avatar.Manifest{
		Supported:     []avatar.State{avatar.StateIdle, avatar.StateThinking, avatar.StateSpeaking},
		FallbackState: avatar.StateIdle,
	})
	if err != nil {
		t.Fatalf("NewStateMachine returned error: %v", err)
	}
	adapter := Adapter{
		TTS:    voice.MockTTSClient{},
		Avatar: machine,
		Clock:  fixedClock(time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)),
	}
	sink := &recordingPresentationSink{}
	streamSink := adapter.NewStreamSink(sink)
	ctx := context.Background()

	for _, event := range []types.StreamEvent{
		{
			Name:           types.StreamEventRequestStarted,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       1,
			Timestamp:      time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
		},
		{
			Name:           types.StreamEventAssistantDelta,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       2,
			Timestamp:      time.Date(2026, 6, 27, 10, 0, 1, 0, time.UTC),
			Payload:        types.Metadata{"content": "Hello there."},
		},
		{
			Name:           types.StreamEventMessageCompleted,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       3,
			Timestamp:      time.Date(2026, 6, 27, 10, 0, 2, 0, time.UTC),
			Payload:        types.Metadata{"content": "Hello there.", "agent_name": "persona-agent"},
		},
		{
			Name:           types.StreamEventDone,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       4,
			Timestamp:      time.Date(2026, 6, 27, 10, 0, 3, 0, time.UTC),
			Payload:        types.Metadata{"status": "completed"},
		},
	} {
		if err := streamSink.Emit(ctx, event); err != nil {
			t.Fatalf("Emit(%s) error = %v", event.Name, err)
		}
	}

	names := eventNames(sink.events)
	want := []EventName{
		EventConversationStarted,
		EventAvatarState,
		EventAssistantTextDelta,
		EventAvatarState,
		EventSubtitle,
		EventAudioChunk,
		EventDone,
		EventAvatarState,
	}
	if len(names) != len(want) {
		t.Fatalf("event names = %#v, want %#v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("event names = %#v, want %#v", names, want)
		}
	}
	if sink.events[1].Payload["state"] != string(avatar.StateThinking) {
		t.Fatalf("thinking payload = %#v", sink.events[1].Payload)
	}
	if sink.events[2].Payload["text"] != "Hello there." {
		t.Fatalf("delta payload = %#v", sink.events[2].Payload)
	}
	if sink.events[3].Payload["state"] != string(avatar.StateSpeaking) {
		t.Fatalf("speaking payload = %#v", sink.events[3].Payload)
	}
	if sink.events[4].Name != EventSubtitle || sink.events[5].Name != EventAudioChunk {
		t.Fatalf("completion events = %#v", names)
	}
	if sink.events[7].Payload["state"] != string(avatar.StateIdle) {
		t.Fatalf("idle payload = %#v", sink.events[7].Payload)
	}
}

func TestAdapterStreamSinkMapsCancellationWithoutTTS(t *testing.T) {
	machine, err := avatar.NewStateMachine(avatar.Manifest{
		Supported:     []avatar.State{avatar.StateIdle, avatar.StateThinking, avatar.StateInterrupted},
		FallbackState: avatar.StateIdle,
	})
	if err != nil {
		t.Fatalf("NewStateMachine returned error: %v", err)
	}
	adapter := Adapter{
		TTS:    voice.MockTTSClient{},
		Avatar: machine,
		Clock:  fixedClock(time.Date(2026, 6, 27, 11, 0, 0, 0, time.UTC)),
	}
	sink := &recordingPresentationSink{}
	streamSink := adapter.NewStreamSink(sink)

	for _, event := range []types.StreamEvent{
		{
			Name:           types.StreamEventRequestStarted,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       1,
			Timestamp:      time.Date(2026, 6, 27, 11, 0, 0, 0, time.UTC),
		},
		{
			Name:           types.StreamEventCanceled,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       2,
			Timestamp:      time.Date(2026, 6, 27, 11, 0, 1, 0, time.UTC),
			Payload:        types.Metadata{"status": "canceled"},
		},
		{
			Name:           types.StreamEventDone,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       3,
			Timestamp:      time.Date(2026, 6, 27, 11, 0, 2, 0, time.UTC),
			Payload:        types.Metadata{"status": "canceled"},
		},
	} {
		if err := streamSink.Emit(context.Background(), event); err != nil {
			t.Fatalf("Emit(%s) error = %v", event.Name, err)
		}
	}

	names := eventNames(sink.events)
	for _, forbidden := range []EventName{EventSubtitle, EventAudioChunk} {
		for _, got := range names {
			if got == forbidden {
				t.Fatalf("event names = %#v, want no %q on cancel", names, forbidden)
			}
		}
	}
	if !containsEventName(names, EventInterrupted) || !containsEventName(names, EventDone) {
		t.Fatalf("event names = %#v, want interrupted + done", names)
	}
}

func TestAdapterStreamSinkCarriesCompletionGenerationMetadataIntoDoneEvent(t *testing.T) {
	machine, err := avatar.NewStateMachine(avatar.Manifest{
		Supported:     []avatar.State{avatar.StateIdle, avatar.StateThinking, avatar.StateSpeaking},
		FallbackState: avatar.StateIdle,
	})
	if err != nil {
		t.Fatalf("NewStateMachine returned error: %v", err)
	}
	adapter := Adapter{
		TTS:    voice.MockTTSClient{},
		Avatar: machine,
		Clock:  fixedClock(time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)),
	}
	sink := &recordingPresentationSink{}
	streamSink := adapter.NewStreamSink(sink)

	for _, event := range []types.StreamEvent{
		{
			Name:           types.StreamEventRequestStarted,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       1,
			Timestamp:      time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
		},
		{
			Name:           types.StreamEventMessageCompleted,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       2,
			Timestamp:      time.Date(2026, 6, 27, 12, 0, 1, 0, time.UTC),
			Payload:        types.Metadata{"content": "Fallback reply"},
			Metadata: types.Metadata{
				"generation_mode":   "fallback",
				"fallback_category": "provider_status",
				"llm_provider":      "deepseek",
				"llm_model":         "deepseek-v4-pro",
			},
		},
		{
			Name:           types.StreamEventDone,
			RequestID:      "req-1",
			TenantID:       "tenant-1",
			UserID:         "user-1",
			ConversationID: "conv-1",
			TurnID:         "turn-1",
			AttemptID:      "attempt-1",
			Sequence:       3,
			Timestamp:      time.Date(2026, 6, 27, 12, 0, 2, 0, time.UTC),
			Payload:        types.Metadata{"status": "completed"},
		},
	} {
		if err := streamSink.Emit(context.Background(), event); err != nil {
			t.Fatalf("Emit(%s) error = %v", event.Name, err)
		}
	}

	done := sink.events[len(sink.events)-2]
	if done.Name != EventDone {
		t.Fatalf("event = %#v, want done before idle avatar", done)
	}
	for key, want := range map[string]any{
		"generation_mode":   "fallback",
		"fallback_category": "provider_status",
		"llm_provider":      "deepseek",
		"llm_model":         "deepseek-v4-pro",
	} {
		if done.Metadata[key] != want {
			t.Fatalf("done metadata[%q] = %v, want %v", key, done.Metadata[key], want)
		}
	}
}

func eventNames(events []Event) []EventName {
	names := make([]EventName, len(events))
	for index, event := range events {
		names[index] = event.Name
	}
	return names
}

type fixedClock time.Time

func (c fixedClock) Now() time.Time {
	return time.Time(c)
}

type failingTTS struct{}

func (failingTTS) Synthesize(context.Context, voice.TTSRequest) (voice.TTSResult, error) {
	return voice.TTSResult{}, errors.New("mock tts failed")
}

type recordingPresentationSink struct {
	events []Event
}

func (s *recordingPresentationSink) Emit(_ context.Context, event Event) error {
	s.events = append(s.events, event)
	return nil
}

func containsEventName(names []EventName, want EventName) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

var _ core.StreamSink = (*AdapterStreamSink)(nil)
