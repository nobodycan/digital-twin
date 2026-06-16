package skills

import (
	"context"
	"errors"
	"testing"

	"github.com/nobodycan/digital-twin/internal/memory"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestMemorySkillsRunWithValidParams(t *testing.T) {
	mem := &fakeMemory{}

	store := NewMemStoreSkill(mem)
	storeResult, err := store.Run(context.Background(), map[string]any{"conversation_id": "conv-1", "content": "remember this"})
	if err != nil {
		t.Fatalf("mem_store Run() error = %v", err)
	}
	if storeResult.SkillName != "mem_store" || mem.remembered != "remember this" {
		t.Fatalf("mem_store result = %#v remembered=%q", storeResult, mem.remembered)
	}

	recall := NewMemRecallSkill(mem)
	recallResult, err := recall.Run(context.Background(), map[string]any{"query": "remember", "limit": 2})
	if err != nil {
		t.Fatalf("mem_recall Run() error = %v", err)
	}
	if recallResult.SkillName != "mem_recall" {
		t.Fatalf("mem_recall skill = %q", recallResult.SkillName)
	}

	summarize := NewSummarizeSkill()
	summaryResult, err := summarize.Run(context.Background(), map[string]any{"content": "alpha beta gamma delta", "max_words": 2})
	if err != nil {
		t.Fatalf("summarize Run() error = %v", err)
	}
	if summaryResult.Output != "alpha beta" {
		t.Fatalf("summarize output = %v, want alpha beta", summaryResult.Output)
	}
}

func TestMemorySkillsRejectInvalidParams(t *testing.T) {
	_, err := NewMemStoreSkill(&fakeMemory{}).Run(context.Background(), map[string]any{"content": "missing conversation"})
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("mem_store error = %v, want ErrInvalidParams", err)
	}

	_, err = NewMemRecallSkill(&fakeMemory{}).Run(context.Background(), map[string]any{"query": 42})
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("mem_recall error = %v, want ErrInvalidParams", err)
	}
}

func TestMemorySkillsReturnDependencyErrors(t *testing.T) {
	mem := &fakeMemory{err: errors.New("memory unavailable")}

	_, err := NewMemStoreSkill(mem).Run(context.Background(), map[string]any{"conversation_id": "conv-1", "content": "remember this"})
	if err == nil || !errors.Is(err, mem.err) {
		t.Fatalf("mem_store error = %v, want dependency error", err)
	}
}

type fakeMemory struct {
	remembered string
	err        error
}

func (f *fakeMemory) Remember(_ context.Context, conversation types.Conversation) error {
	if f.err != nil {
		return f.err
	}
	if len(conversation.Messages) > 0 {
		f.remembered = conversation.Messages[len(conversation.Messages)-1].Content
	}
	return nil
}

func (f *fakeMemory) Recall(context.Context, string, string, string, int) ([]memory.Record, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []memory.Record{{ID: "rec-1", Content: "remember this", Score: 0.9}}, nil
}
