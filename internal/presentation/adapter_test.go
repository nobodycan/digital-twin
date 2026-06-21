package presentation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/avatar"
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
