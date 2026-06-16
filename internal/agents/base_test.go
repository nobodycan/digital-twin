package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestBaseAgentBuildsAssistantResult(t *testing.T) {
	base := BaseAgent{NameValue: "base"}

	result := base.Result("hello", types.Confidence(0.8), types.Metadata{"k": "v"})

	if result.AgentName != "base" {
		t.Fatalf("Result() agent = %q, want base", result.AgentName)
	}
	if result.Message.Role != types.RoleAssistant || result.Message.Content != "hello" {
		t.Fatalf("Result() message = %#v", result.Message)
	}
	if result.Metadata["k"] != "v" {
		t.Fatalf("Result() metadata = %#v", result.Metadata)
	}
}

func TestBaseAgentRequiresSkill(t *testing.T) {
	base := BaseAgent{NameValue: "base", Skills: core.NewSkillRegistry()}

	_, err := base.RunSkill(context.Background(), "missing", map[string]any{})
	if err == nil {
		t.Fatal("RunSkill() error = nil, want missing skill")
	}
	if !errors.Is(err, core.ErrSkillNotFound) {
		t.Fatalf("RunSkill() error = %v, want ErrSkillNotFound", err)
	}
}

func TestBaseAgentRunSkillUsesRegistry(t *testing.T) {
	registry := core.NewSkillRegistry()
	if err := registry.Register(stubSkill{name: "ok", output: "done"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	base := BaseAgent{NameValue: "base", Skills: registry}

	result, err := base.RunSkill(context.Background(), "ok", map[string]any{})
	if err != nil {
		t.Fatalf("RunSkill() error = %v", err)
	}
	if result.Output != "done" {
		t.Fatalf("RunSkill() output = %v, want done", result.Output)
	}
}

type stubSkill struct {
	name   string
	output any
	err    error
}

func (s stubSkill) Name() string { return s.name }

func (s stubSkill) Run(context.Context, map[string]any) (types.SkillResult, error) {
	return types.SkillResult{SkillName: s.name, Output: s.output}, s.err
}
