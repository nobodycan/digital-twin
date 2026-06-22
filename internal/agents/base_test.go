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

func TestBaseAgentRunSkillDeniesBeforeExecutingSkill(t *testing.T) {
	registry := core.NewSkillRegistry()
	skill := &countingSkill{name: "http_call"}
	if err := registry.Register(skill); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	base := BaseAgent{
		NameValue: "base",
		Skills:    registry,
		SkillAuthorizer: denyAuthorizer{
			err: core.WrapError(core.ErrUnauthorized, "tool policy denied http_call"),
		},
	}

	_, err := base.RunSkill(context.Background(), "http_call", map[string]any{"url": "https://api.example.com/data"})
	if !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("RunSkill() error = %v, want unauthorized", err)
	}
	if skill.calls != 0 {
		t.Fatalf("denied skill was executed %d times, want 0", skill.calls)
	}
}

func TestBaseAgentRunSkillPassesGovernanceContextToAuthorizer(t *testing.T) {
	registry := core.NewSkillRegistry()
	if err := registry.Register(stubSkill{name: "calendar", output: "ok"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	authorizer := &capturingAuthorizer{}
	base := BaseAgent{
		NameValue:       "base",
		Skills:          registry,
		TenantID:        "tenant-1",
		PersonaID:       "advisor",
		SkillAuthorizer: authorizer,
	}

	_, err := base.RunSkill(context.Background(), "calendar", map[string]any{"action": "list"})
	if err != nil {
		t.Fatalf("RunSkill() error = %v", err)
	}
	if authorizer.call.TenantID != "tenant-1" || authorizer.call.PersonaID != "advisor" || authorizer.call.SkillName != "calendar" {
		t.Fatalf("authorizer call = %#v", authorizer.call)
	}
	if authorizer.call.Params["action"] != "list" {
		t.Fatalf("authorizer params = %#v", authorizer.call.Params)
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

type countingSkill struct {
	name  string
	calls int
}

func (s *countingSkill) Name() string { return s.name }

func (s *countingSkill) Run(context.Context, map[string]any) (types.SkillResult, error) {
	s.calls++
	return types.SkillResult{SkillName: s.name}, nil
}

type denyAuthorizer struct {
	err error
}

func (a denyAuthorizer) AuthorizeSkill(context.Context, SkillCall) error {
	return a.err
}

type capturingAuthorizer struct {
	call SkillCall
}

func (a *capturingAuthorizer) AuthorizeSkill(_ context.Context, call SkillCall) error {
	a.call = call
	return nil
}
