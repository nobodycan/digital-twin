package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/agents"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/testutil"
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

func TestNewLocalRuntimeWiresSkillAuthorizerIntoAgents(t *testing.T) {
	authorizer := &denyAllSkills{}
	local, err := NewLocalRuntime(LocalRuntimeConfig{
		TenantID:        "tenant-1",
		PersonaID:       "advisor",
		SkillAuthorizer: authorizer,
	})
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
			Content:   "call https://example.com",
			CreatedAt: time.Now().UTC(),
		}},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}
	if authorizer.calls != 1 {
		t.Fatalf("authorizer calls = %d, want 1", authorizer.calls)
	}
	if authorizer.call.TenantID != "tenant-1" || authorizer.call.PersonaID != "advisor" || authorizer.call.SkillName != "http_call" {
		t.Fatalf("authorizer call = %#v", authorizer.call)
	}
	if result.Metadata["error"] != "agent_failed" {
		t.Fatalf("result metadata = %#v, want agent_failed fallback", result.Metadata)
	}
}

func TestNewLocalRuntimeUsesConfiguredPersonaLLM(t *testing.T) {
	local, err := NewLocalRuntime(LocalRuntimeConfig{
		PersonaLLM: &testutil.FakeLLM{
			ChatResponse: llm.ChatResponse{Message: types.Message{Role: types.RoleAssistant, Content: "I think this is an LLM runtime reply."}},
		},
		PersonaLLMProvider: "openai-compatible",
		PersonaLLMModel:    "gpt-runtime",
	})
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
	if result.Message.Content != "I think this is an LLM runtime reply." {
		t.Fatalf("Message.Content = %q, want injected LLM runtime reply", result.Message.Content)
	}
	if result.Metadata["llm_model"] != "gpt-runtime" {
		t.Fatalf("llm_model = %v, want gpt-runtime", result.Metadata["llm_model"])
	}
}

func TestNewLocalRuntimeStreamPersistsConversationHistoryToConfiguredDataDir(t *testing.T) {
	dataDir := t.TempDir()
	local, err := NewLocalRuntime(LocalRuntimeConfig{
		DataDir: dataDir,
	})
	if err != nil {
		t.Fatalf("NewLocalRuntime() error = %v", err)
	}

	streaming, ok := any(local.Orchestrator).(core.StreamingOrchestrator)
	if !ok {
		t.Fatal("expected orchestrator to implement core.StreamingOrchestrator")
	}

	sink := discardStreamSink{}
	_, err = streaming.Stream(t.Context(), types.TurnRequest{
		ConversationID: "conv-1",
		TenantID:       "tenant-1",
		UserID:         "user-1",
		TurnID:         "turn-1",
		AttemptID:      "attempt-1",
		Message: types.Message{
			ID:        "msg-1",
			Role:      types.RoleUser,
			Content:   "hello",
			CreatedAt: time.Now().UTC(),
		},
	}, sink)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	path := filepath.Join(dataDir, "tenants", "tenant-1", "users", "user-1", "conversations", "conv-1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var conversation types.Conversation
	if err := json.Unmarshal(data, &conversation); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(conversation.Turns) != 1 || conversation.Turns[0].Status != types.TurnCompleted {
		t.Fatalf("conversation turns = %#v, want one completed turn", conversation.Turns)
	}
}

func TestNewLocalRuntimeKeepsConversationHistoryAcrossRebuilds(t *testing.T) {
	dataDir := t.TempDir()
	first, err := NewLocalRuntime(LocalRuntimeConfig{DataDir: dataDir})
	if err != nil {
		t.Fatalf("first NewLocalRuntime() error = %v", err)
	}
	firstStreaming := any(first.Orchestrator).(core.StreamingOrchestrator)
	if _, err := firstStreaming.Stream(t.Context(), types.TurnRequest{
		ConversationID: "conv-1",
		TenantID:       "tenant-1",
		UserID:         "user-1",
		TurnID:         "turn-1",
		AttemptID:      "attempt-1",
		Message: types.Message{
			ID:        "msg-1",
			Role:      types.RoleUser,
			Content:   "hello",
			CreatedAt: time.Now().UTC(),
		},
	}, discardStreamSink{}); err != nil {
		t.Fatalf("first Stream() error = %v", err)
	}

	second, err := NewLocalRuntime(LocalRuntimeConfig{DataDir: dataDir})
	if err != nil {
		t.Fatalf("second NewLocalRuntime() error = %v", err)
	}
	secondStreaming := any(second.Orchestrator).(core.StreamingOrchestrator)
	if _, err := secondStreaming.Stream(t.Context(), types.TurnRequest{
		ConversationID: "conv-1",
		TenantID:       "tenant-1",
		UserID:         "user-1",
		TurnID:         "turn-2",
		AttemptID:      "attempt-1",
		Message: types.Message{
			ID:        "msg-2",
			Role:      types.RoleUser,
			Content:   "hello again",
			CreatedAt: time.Now().UTC(),
		},
	}, discardStreamSink{}); err != nil {
		t.Fatalf("second Stream() error = %v", err)
	}

	path := filepath.Join(dataDir, "tenants", "tenant-1", "users", "user-1", "conversations", "conv-1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var conversation types.Conversation
	if err := json.Unmarshal(data, &conversation); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(conversation.Turns) != 2 {
		t.Fatalf("turns len = %d, want 2", len(conversation.Turns))
	}
}

type denyAllSkills struct {
	calls int
	call  agents.SkillCall
}

func (a *denyAllSkills) AuthorizeSkill(_ context.Context, call agents.SkillCall) error {
	a.calls++
	a.call = call
	return core.WrapError(core.ErrUnauthorized, "test authorizer denied skill")
}

type discardStreamSink struct{}

func (discardStreamSink) Emit(context.Context, types.StreamEvent) error {
	return nil
}
