package skills

import (
	"context"
	"strings"

	"github.com/nobodycan/digital-twin/pkg/types"
)

type TaskDecomposeSkill struct{}

func NewTaskDecomposeSkill() TaskDecomposeSkill { return TaskDecomposeSkill{} }
func (s TaskDecomposeSkill) Name() string       { return "task_decompose" }

func (s TaskDecomposeSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "request", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	parts := strings.Split(valid["request"].(string), ",")
	steps := make([]string, 0, len(parts))
	for _, part := range parts {
		if step := strings.TrimSpace(part); step != "" {
			steps = append(steps, step)
		}
	}
	if len(steps) == 0 {
		steps = append(steps, strings.TrimSpace(valid["request"].(string)))
	}
	return types.SkillResult{SkillName: s.Name(), Output: steps}, nil
}

type PlanSkill struct{}

func NewPlanSkill() PlanSkill { return PlanSkill{} }
func (s PlanSkill) Name() string {
	return "plan"
}

func (s PlanSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{
		{Name: "goal", Type: String, Required: true},
		{Name: "steps", Type: StringSlice, Required: true},
	}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	return types.SkillResult{SkillName: s.Name(), Output: map[string]any{
		"goal":  valid["goal"],
		"steps": valid["steps"],
	}}, nil
}

type TrackSkill struct{}

func NewTrackSkill() TrackSkill { return TrackSkill{} }
func (s TrackSkill) Name() string {
	return "track"
}

func (s TrackSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{
		{Name: "task_id", Type: String, Required: true},
		{Name: "status", Type: String, Required: true},
	}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	return types.SkillResult{SkillName: s.Name(), Output: valid["task_id"].(string) + ": " + valid["status"].(string)}, nil
}
