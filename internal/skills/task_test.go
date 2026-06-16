package skills

import (
	"context"
	"errors"
	"testing"
)

func TestTaskSkillsRunWithValidParams(t *testing.T) {
	decompose, err := NewTaskDecomposeSkill().Run(context.Background(), map[string]any{"request": "design, build, verify"})
	if err != nil {
		t.Fatalf("task_decompose Run() error = %v", err)
	}
	steps := decompose.Output.([]string)
	if len(steps) != 3 || steps[0] != "design" {
		t.Fatalf("task_decompose output = %#v", decompose.Output)
	}

	plan, err := NewPlanSkill().Run(context.Background(), map[string]any{"goal": "ship phase 2", "steps": []string{"test", "build"}})
	if err != nil {
		t.Fatalf("plan Run() error = %v", err)
	}
	if plan.SkillName != "plan" {
		t.Fatalf("plan skill = %q", plan.SkillName)
	}

	track, err := NewTrackSkill().Run(context.Background(), map[string]any{"task_id": "P2-01", "status": "done"})
	if err != nil {
		t.Fatalf("track Run() error = %v", err)
	}
	if track.Output != "P2-01: done" {
		t.Fatalf("track output = %v, want status", track.Output)
	}
}

func TestTaskSkillsRejectInvalidParams(t *testing.T) {
	_, err := NewTaskDecomposeSkill().Run(context.Background(), map[string]any{"request": 1})
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("task_decompose error = %v, want ErrInvalidParams", err)
	}
}
