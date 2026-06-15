package testutil

import (
	"context"
	"testing"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestFakeAgentRecordsCallsAndReturnsConfiguredResult(t *testing.T) {
	fake := &FakeAgent{
		AgentName: "fake-agent",
		Result:    types.AgentResult{AgentName: "fake-agent"},
		Handles:   true,
	}

	var _ core.Agent = fake

	result, err := fake.Run(context.Background(), types.Conversation{ID: "conv-1"}, types.Intent{Name: types.IntentMemoryRecall})
	if err != nil {
		t.Fatalf("run fake agent: %v", err)
	}
	if result.AgentName != "fake-agent" {
		t.Fatalf("expected configured result, got %#v", result)
	}
	if len(fake.Calls()) != 1 {
		t.Fatalf("expected one call, got %d", len(fake.Calls()))
	}
	if !fake.CanHandle(types.Intent{Name: types.IntentMemoryRecall}) {
		t.Fatalf("expected fake to handle intent")
	}
}

func TestFakeSkillRouterAndOrchestratorCompile(t *testing.T) {
	var _ core.Skill = &FakeSkill{SkillName: "fake-skill"}
	var _ core.Router = &FakeRouter{}
	var _ core.Orchestrator = &FakeOrchestrator{}
}
