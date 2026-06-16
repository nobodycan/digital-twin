package app

import (
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestNewLocalRuntimeHandlesDeterministicPersonaConversation(t *testing.T) {
	local, err := NewLocalRuntime(LocalRuntimeConfig{})
	if err != nil {
		t.Fatalf("NewLocalRuntime() error = %v", err)
	}

	result, err := local.Orchestrator.Handle(t.Context(), types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
		Messages: []types.Message{{
			ID:        "msg-1",
			Role:      types.RoleUser,
			Content:   "hello",
			CreatedAt: time.Now().UTC(),
		}},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.AgentName != "persona-agent" {
		t.Fatalf("AgentName = %q, want persona-agent", result.AgentName)
	}
	if result.Message.Content == "" {
		t.Fatal("Message.Content is empty")
	}
}
