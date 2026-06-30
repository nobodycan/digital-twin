package knowledge

import (
	"context"
	"testing"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestServiceGroundUsesLexicalPipelineByDefault(t *testing.T) {
	store := admin.NewInMemoryKnowledgeStore()
	service := admin.NewKnowledgeService(store)
	if _, err := service.Upload("tenant-1", admin.KnowledgeUpload{
		ID:      "kb-1",
		Name:    "planning.md",
		Content: "Phase 11 should add retrieval diagnostics.\n\nGrounded answers should stay auditable.",
	}); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	grounding, err := NewService(store).Ground(context.Background(), types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
	}, "retrieval diagnostics", 3)
	if err != nil {
		t.Fatalf("Ground() error = %v", err)
	}
	if grounding.RetrievalMode != string(RetrievalModeLexical) {
		t.Fatalf("RetrievalMode = %q, want lexical", grounding.RetrievalMode)
	}
	if grounding.NoSourceReason != "" {
		t.Fatalf("NoSourceReason = %q, want empty", grounding.NoSourceReason)
	}
	if len(grounding.Citations) != 1 {
		t.Fatalf("citations len = %d, want 1", len(grounding.Citations))
	}
	if len(grounding.Explanations) != 1 {
		t.Fatalf("explanations len = %d, want 1", len(grounding.Explanations))
	}
	if grounding.Explanations[0].LexicalScore <= 0 {
		t.Fatalf("LexicalScore = %v, want positive", grounding.Explanations[0].LexicalScore)
	}
}

func TestServiceGroundReturnsNoSourceReasonWhenNothingMatches(t *testing.T) {
	store := admin.NewInMemoryKnowledgeStore()
	service := admin.NewKnowledgeService(store)
	if _, err := service.Upload("tenant-1", admin.KnowledgeUpload{
		ID:      "kb-1",
		Name:    "planning.md",
		Content: "Phase 11 should add retrieval diagnostics.",
	}); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	grounding, err := NewService(store).Ground(context.Background(), types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
	}, "calendar sync", 3)
	if err != nil {
		t.Fatalf("Ground() error = %v", err)
	}
	if len(grounding.Citations) != 0 {
		t.Fatalf("citations len = %d, want 0", len(grounding.Citations))
	}
	if grounding.NoSourceReason != "no_matching_chunks" {
		t.Fatalf("NoSourceReason = %q, want no_matching_chunks", grounding.NoSourceReason)
	}
	if len(grounding.StagesRun) != 1 || grounding.StagesRun[0] != "lexical" {
		t.Fatalf("StagesRun = %#v, want lexical only", grounding.StagesRun)
	}
}
