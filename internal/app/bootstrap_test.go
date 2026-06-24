package app

import (
	"context"
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

type denyAllSkills struct {
	calls int
	call  agents.SkillCall
}

func (a *denyAllSkills) AuthorizeSkill(_ context.Context, call agents.SkillCall) error {
	a.calls++
	a.call = call
	return core.WrapError(core.ErrUnauthorized, "test authorizer denied skill")
}
