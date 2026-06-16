package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestExpertAgentsCanHandleAndRegister(t *testing.T) {
	registry := core.NewAgentRegistry()
	skills := skillRegistryWithDefaults(t, nil)
	agents := []core.Agent{
		NewPersonaAgent(skills),
		NewMemoryAgent(skills),
		NewKnowledgeAgent(skills),
		NewTaskAgent(skills),
		NewToolAgent(skills),
		NewSafetyAgent(skills),
	}

	for _, agent := range agents {
		if err := registry.Register(agent); err != nil {
			t.Fatalf("Register(%s) error = %v", agent.Name(), err)
		}
	}

	tests := []struct {
		intent types.Intent
		want   string
	}{
		{types.Intent{Name: types.IntentPersonaChat}, "persona-agent"},
		{types.Intent{Name: types.IntentMemoryRecall}, "memory-agent"},
		{types.Intent{Name: types.IntentKnowledgeQuery}, "knowledge-agent"},
		{types.Intent{Name: types.IntentTaskExecution}, "task-agent"},
		{types.Intent{Name: types.IntentToolCall}, "tool-agent"},
		{types.Intent{Name: types.IntentSafetyCheck}, "safety-agent"},
	}

	for _, tt := range tests {
		got, err := registry.Find(tt.intent)
		if err != nil {
			t.Fatalf("Find(%s) error = %v", tt.intent.Name, err)
		}
		if got.Name() != tt.want {
			t.Fatalf("Find(%s) = %s, want %s", tt.intent.Name, got.Name(), tt.want)
		}
	}
}

func TestExpertAgentsRun(t *testing.T) {
	skills := skillRegistryWithDefaults(t, nil)
	conversation := agentConversation("remember the launch plan")

	tests := []struct {
		name   string
		agent  core.Agent
		intent types.Intent
	}{
		{"persona", NewPersonaAgent(skills), types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.9}},
		{"memory", NewMemoryAgent(skills), types.Intent{Name: types.IntentMemoryRecall, Query: "remember", Confidence: 0.9}},
		{"knowledge", NewKnowledgeAgent(skills), types.Intent{Name: types.IntentKnowledgeQuery, Query: "knowledge", Confidence: 0.9}},
		{"task", NewTaskAgent(skills), types.Intent{Name: types.IntentTaskExecution, Query: "plan", Confidence: 0.9}},
		{"tool", NewToolAgent(skills), types.Intent{Name: types.IntentToolCall, Query: "call", Confidence: 0.9}},
		{"safety", NewSafetyAgent(skills), types.Intent{Name: types.IntentSafetyCheck, Query: "check private data", Confidence: 0.9}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.agent.Run(context.Background(), conversation, tt.intent)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.AgentName != tt.agent.Name() {
				t.Fatalf("Run() agent = %q, want %q", result.AgentName, tt.agent.Name())
			}
			if result.Message.Role != types.RoleAssistant || result.Message.Content == "" {
				t.Fatalf("Run() message = %#v", result.Message)
			}
		})
	}
}

func TestExpertAgentsReturnSkillDependencyErrors(t *testing.T) {
	dependencyErr := errors.New("skill down")
	skills := skillRegistryWithDefaults(t, dependencyErr)
	agent := NewMemoryAgent(skills)

	_, err := agent.Run(context.Background(), agentConversation("remember this"), types.Intent{Name: types.IntentMemoryRecall, Query: "remember"})
	if !errors.Is(err, dependencyErr) {
		t.Fatalf("Run() error = %v, want dependency error", err)
	}
}

func skillRegistryWithDefaults(t *testing.T, err error) *core.SkillRegistry {
	t.Helper()
	registry := core.NewSkillRegistry()
	for _, skill := range []core.Skill{
		stubSkill{name: "persona_check", output: "ok", err: err},
		stubSkill{name: "mem_recall", output: []string{"memory"}, err: err},
		stubSkill{name: "vector_search", output: []string{"knowledge"}, err: err},
		stubSkill{name: "task_decompose", output: []string{"step"}, err: err},
		stubSkill{name: "http_call", output: "allowed", err: err},
		stubSkill{name: "risk_classify", output: "low", err: err},
	} {
		if registerErr := registry.Register(skill); registerErr != nil {
			t.Fatalf("Register(%s) error = %v", skill.Name(), registerErr)
		}
	}
	return registry
}

func agentConversation(text string) types.Conversation {
	now := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)
	return types.Conversation{
		ID:       "conv-agent",
		TenantID: "tenant-1",
		UserID:   "user-1",
		Messages: []types.Message{
			{ID: "msg-1", Role: types.RoleUser, Content: text, CreatedAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}
