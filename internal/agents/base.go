package agents

import (
	"context"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

// BaseAgent contains shared helpers for expert agents.
type BaseAgent struct {
	NameValue       string
	TenantID        string
	PersonaID       string
	Skills          *core.SkillRegistry
	SkillAuthorizer SkillAuthorizer
}

type SkillAuthorizer interface {
	AuthorizeSkill(context.Context, SkillCall) error
}

type SkillCall struct {
	TenantID  string
	PersonaID string
	AgentName string
	SkillName string
	Params    map[string]any
}

// Name returns the stable agent name.
func (a BaseAgent) Name() string {
	return a.NameValue
}

// Result builds a structured assistant result.
func (a BaseAgent) Result(content string, confidence types.Confidence, metadata types.Metadata) types.AgentResult {
	return types.AgentResult{
		AgentName: a.Name(),
		Message: types.Message{
			Role:      types.RoleAssistant,
			Content:   content,
			CreatedAt: time.Now().UTC(),
		},
		Confidence: confidence,
		Metadata:   metadata,
	}
}

// RunSkill finds and runs a skill from the registry.
func (a BaseAgent) RunSkill(ctx context.Context, name string, params map[string]any) (types.SkillResult, error) {
	if a.Skills == nil {
		return types.SkillResult{}, core.NewNamedError(core.ErrSkillNotFound, "skill", name)
	}
	skill, err := a.Skills.Get(name)
	if err != nil {
		return types.SkillResult{}, err
	}
	if a.SkillAuthorizer != nil {
		if err := a.SkillAuthorizer.AuthorizeSkill(ctx, SkillCall{
			TenantID:  a.TenantID,
			PersonaID: a.PersonaID,
			AgentName: a.Name(),
			SkillName: name,
			Params:    params,
		}); err != nil {
			return types.SkillResult{}, err
		}
	}
	return skill.Run(ctx, params)
}
