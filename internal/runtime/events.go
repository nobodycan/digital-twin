package runtime

import "github.com/nobodycan/digital-twin/pkg/types"

type EventName string

const (
	EventRequestStarted   EventName = "request_started"
	EventStateChanged     EventName = "state_changed"
	EventRouteSelected    EventName = "route_selected"
	EventAgentSelected    EventName = "agent_selected"
	EventMessageDelta     EventName = "message_delta"
	EventMessageCompleted EventName = "message_completed"
	EventRuntimeError     EventName = "runtime_error"
	EventDone             EventName = "done"
)

type RuntimeEvent struct {
	Topic          string         `json:"topic"`
	RequestID      string         `json:"request_id"`
	ConversationID string         `json:"conversation_id"`
	TenantID       string         `json:"tenant_id"`
	UserID         string         `json:"user_id"`
	Metadata       types.Metadata `json:"metadata,omitempty"`
}

func NewRuntimeEvent(name EventName, requestID string, conversation types.Conversation, metadata types.Metadata) RuntimeEvent {
	return RuntimeEvent{
		Topic:          string(name),
		RequestID:      requestID,
		ConversationID: conversation.ID,
		TenantID:       conversation.TenantID,
		UserID:         conversation.UserID,
		Metadata:       copyMetadata(metadata),
	}
}

func copyMetadata(metadata types.Metadata) types.Metadata {
	if metadata == nil {
		return nil
	}
	copied := make(types.Metadata, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}
