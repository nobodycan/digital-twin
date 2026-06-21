package presentation

import (
	"sort"
	"time"
)

type EventName string

const (
	EventConversationStarted EventName = "conversation_started"
	EventUserText            EventName = "user_text"
	EventASRPartial          EventName = "asr_partial"
	EventASRFinal            EventName = "asr_final"
	EventAssistantTextDelta  EventName = "assistant_text_delta"
	EventSubtitle            EventName = "subtitle"
	EventAudioChunk          EventName = "audio_chunk"
	EventAvatarState         EventName = "avatar_state"
	EventToolStatus          EventName = "tool_status"
	EventInterrupted         EventName = "interrupted"
	EventError               EventName = "error"
	EventDone                EventName = "done"
)

type EventContext struct {
	TenantID       string
	UserID         string
	ConversationID string
	RequestID      string
	Sequence       int
	OccurredAt     time.Time
}

type Event struct {
	Name           EventName      `json:"name"`
	TenantID       string         `json:"tenant_id"`
	UserID         string         `json:"user_id"`
	ConversationID string         `json:"conversation_id"`
	RequestID      string         `json:"request_id"`
	Sequence       int            `json:"sequence"`
	OccurredAt     time.Time      `json:"occurred_at"`
	Payload        map[string]any `json:"payload,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

func NewEvent(name EventName, context EventContext, payload, metadata map[string]any) Event {
	return Event{
		Name:           name,
		TenantID:       context.TenantID,
		UserID:         context.UserID,
		ConversationID: context.ConversationID,
		RequestID:      context.RequestID,
		Sequence:       context.Sequence,
		OccurredAt:     context.OccurredAt,
		Payload:        copyMap(payload),
		Metadata:       copyMap(metadata),
	}
}

func OrderedEvents(events []Event) []Event {
	ordered := make([]Event, len(events))
	copy(ordered, events)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Sequence < ordered[j].Sequence
	})
	return ordered
}

func NewErrorEvent(context EventContext, problem, cause, fix string) Event {
	return NewEvent(EventError, context, map[string]any{
		"problem": problem,
		"cause":   cause,
		"fix":     fix,
	}, nil)
}

func copyMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	copied := make(map[string]any, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
