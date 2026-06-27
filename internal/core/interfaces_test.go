package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestCoreInterfaceCompileTimeShape(t *testing.T) {
	var _ Agent = agentFunc{}
	var _ Skill = skillFunc{}
	var _ Router = routerFunc{}
	var _ Orchestrator = orchestratorFunc{}
	var _ StreamSink = streamSinkFunc{}
	var _ AssistantDeltaSink = assistantDeltaSinkFunc{}
	var _ StreamingAgent = streamingAgentFunc{}
	var _ StreamingOrchestrator = streamingOrchestratorFunc{}
}

func TestNewNamedErrorMatchesSentinel(t *testing.T) {
	err := NewNamedError(ErrDuplicateName, "agent", "memory")

	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("expected duplicate sentinel match")
	}
	if got := err.Error(); got == "" || !containsAll(got, "agent", "memory") {
		t.Fatalf("expected error to include kind and name, got %q", got)
	}
}

func TestAdditionalErrorPredicates(t *testing.T) {
	tests := []struct {
		name string
		err  error
		ok   func(error) bool
	}{
		{name: "duplicate", err: ErrDuplicateName, ok: IsDuplicateName},
		{name: "skill not found", err: ErrSkillNotFound, ok: IsSkillNotFound},
		{name: "provider failure", err: ErrProviderFailure, ok: IsProviderFailure},
		{name: "store failure", err: ErrStoreFailure, ok: IsStoreFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.ok(WrapError(tt.err, "context")) {
				t.Fatalf("expected predicate to match %v", tt.err)
			}
		})
	}
}

type agentFunc struct{}

func (agentFunc) Name() string { return "agent" }

func (agentFunc) CanHandle(types.Intent) bool { return true }

func (agentFunc) Run(context.Context, types.Conversation, types.Intent) (types.AgentResult, error) {
	return types.AgentResult{}, nil
}

type skillFunc struct{}

func (skillFunc) Name() string { return "skill" }

func (skillFunc) Run(context.Context, map[string]any) (types.SkillResult, error) {
	return types.SkillResult{}, nil
}

type routerFunc struct{}

func (routerFunc) Route(context.Context, types.Conversation) (types.Intent, error) {
	return types.Intent{}, nil
}

type orchestratorFunc struct{}

func (orchestratorFunc) Handle(context.Context, types.Conversation) (types.AgentResult, error) {
	return types.AgentResult{}, nil
}

type streamSinkFunc struct{}

func (streamSinkFunc) Emit(context.Context, types.StreamEvent) error { return nil }

type assistantDeltaSinkFunc struct{}

func (assistantDeltaSinkFunc) EmitAssistantDelta(context.Context, string) error { return nil }

type streamingAgentFunc struct{ agentFunc }

func (streamingAgentFunc) Stream(context.Context, types.Conversation, types.Intent, AssistantDeltaSink) (types.AgentResult, error) {
	return types.AgentResult{}, nil
}

type streamingOrchestratorFunc struct{ orchestratorFunc }

func (streamingOrchestratorFunc) Stream(context.Context, types.TurnRequest, StreamSink) (types.AgentResult, error) {
	return types.AgentResult{}, nil
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !contains(value, part) {
			return false
		}
	}
	return true
}

func contains(value, part string) bool {
	return strings.Contains(value, part)
}
