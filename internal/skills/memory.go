package skills

import (
	"context"
	"strings"
	"time"

	"github.com/nobodycan/digital-twin/internal/memory"
	"github.com/nobodycan/digital-twin/pkg/types"
)

// MemStoreSkill stores a memory-like conversation fragment.
type MemStoreSkill struct {
	mem memory.Memory
}

// NewMemStoreSkill creates a memory store skill.
func NewMemStoreSkill(mem memory.Memory) MemStoreSkill {
	return MemStoreSkill{mem: mem}
}

func (s MemStoreSkill) Name() string { return "mem_store" }

// Run stores content as a small conversation.
func (s MemStoreSkill) Run(ctx context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{
		{Name: "conversation_id", Type: String, Required: true},
		{Name: "content", Type: String, Required: true},
	}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	conversation := types.Conversation{
		ID:        valid["conversation_id"].(string),
		Messages:  []types.Message{{Role: types.RoleUser, Content: valid["content"].(string), CreatedAt: time.Now().UTC()}},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if s.mem != nil {
		if err := s.mem.Remember(ctx, conversation); err != nil {
			return types.SkillResult{}, err
		}
	}
	return types.SkillResult{SkillName: s.Name(), Output: "stored"}, nil
}

// MemRecallSkill recalls memories through the memory abstraction.
type MemRecallSkill struct {
	mem memory.Memory
}

// NewMemRecallSkill creates a memory recall skill.
func NewMemRecallSkill(mem memory.Memory) MemRecallSkill {
	return MemRecallSkill{mem: mem}
}

func (s MemRecallSkill) Name() string { return "mem_recall" }

// Run recalls memories for a query.
func (s MemRecallSkill) Run(ctx context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{
		{Name: "query", Type: String, Required: true},
		{Name: "limit", Type: Number, Default: 3.0},
	}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	limit := int(valid["limit"].(float64))
	var records []memory.Record
	if s.mem != nil {
		records, err = s.mem.Recall(ctx, "", "", valid["query"].(string), limit)
		if err != nil {
			return types.SkillResult{}, err
		}
	}
	return types.SkillResult{SkillName: s.Name(), Output: records}, nil
}

// SummarizeSkill returns a deterministic first-N-words summary.
type SummarizeSkill struct{}

// NewSummarizeSkill creates a local summarize skill.
func NewSummarizeSkill() SummarizeSkill { return SummarizeSkill{} }

func (s SummarizeSkill) Name() string { return "summarize" }

// Run summarizes content locally.
func (s SummarizeSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{
		{Name: "content", Type: String, Required: true},
		{Name: "max_words", Type: Number, Default: 32.0},
	}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	words := strings.Fields(valid["content"].(string))
	maxWords := int(valid["max_words"].(float64))
	if maxWords < len(words) {
		words = words[:maxWords]
	}
	return types.SkillResult{SkillName: s.Name(), Output: strings.Join(words, " ")}, nil
}
