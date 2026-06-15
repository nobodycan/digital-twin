package memory

import (
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestShortTermMemoryPreservesSystemAndRecentMessages(t *testing.T) {
	memory := NewShortTermMemory(3)
	conversation := types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
		Messages: []types.Message{
			msg("system", types.RoleSystem, "always preserve this system instruction"),
			msg("old", types.RoleUser, "old message words"),
			msg("new", types.RoleAssistant, "new reply"),
		},
	}

	window, err := memory.Window(t.Context(), conversation)
	if err != nil {
		t.Fatalf("window: %v", err)
	}
	if len(window.Messages) != 2 {
		t.Fatalf("expected system plus newest message, got %#v", window.Messages)
	}
	if window.Messages[0].ID != "system" || window.Messages[1].ID != "new" {
		t.Fatalf("unexpected message order %#v", window.Messages)
	}
}

func TestShortTermMemoryHandlesEmptyConversation(t *testing.T) {
	memory := NewShortTermMemory(1)
	window, err := memory.Window(t.Context(), types.Conversation{ID: "conv-1"})
	if err != nil {
		t.Fatalf("window: %v", err)
	}
	if len(window.Messages) != 0 {
		t.Fatalf("expected no messages, got %#v", window.Messages)
	}
}

func msg(id string, role types.Role, content string) types.Message {
	return types.Message{ID: id, Role: role, Content: content, CreatedAt: time.Now().UTC()}
}
