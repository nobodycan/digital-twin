package runtime

import (
	"testing"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestNewRuntimeEventCarriesStableTopicAndCorrelation(t *testing.T) {
	conversation := types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
	}

	event := NewRuntimeEvent(EventRouteSelected, "req-1", conversation, types.Metadata{"intent": "persona.chat"})

	if event.Topic != "route_selected" {
		t.Fatalf("Topic = %q, want route_selected", event.Topic)
	}
	if event.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want req-1", event.RequestID)
	}
	if event.ConversationID != "conv-1" || event.TenantID != "tenant-1" || event.UserID != "user-1" {
		t.Fatalf("correlation = %#v", event)
	}
	if event.Metadata["intent"] != "persona.chat" {
		t.Fatalf("Metadata[intent] = %#v, want persona.chat", event.Metadata["intent"])
	}
}

func TestRuntimeEventMetadataIsCopied(t *testing.T) {
	metadata := types.Metadata{"state": "routing"}
	event := NewRuntimeEvent(EventStateChanged, "req-1", types.Conversation{}, metadata)

	metadata["state"] = "mutated"

	if event.Metadata["state"] != "routing" {
		t.Fatalf("Metadata[state] = %#v, want copied routing", event.Metadata["state"])
	}
}
