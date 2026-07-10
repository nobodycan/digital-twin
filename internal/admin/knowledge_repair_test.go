package admin

import (
	"testing"
	"time"
)

func TestKnowledgeRepairServiceProjectsRepeatedWeakGapWithPriority(t *testing.T) {
	knowledgeStore := NewInMemoryKnowledgeStore()
	knowledgeService := NewKnowledgeService(knowledgeStore)
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	auditService := NewAuditService(NewInMemoryAuditStore())
	base := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	gapService.now = func() time.Time { return base }
	auditService.now = func() time.Time { return base }

	gap, err := gapService.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "How should I verify DeepSeek startup?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	for i, createdAt := range []time.Time{base.Add(5 * time.Minute), base.Add(10 * time.Minute)} {
		if _, err := auditService.Record("tenant-1", AuditRecord{
			ID:                      "audit-gap-repeat-" + string(rune('a'+i)),
			ConversationID:          "conv-repeat-" + string(rune('a'+i)),
			Status:                  AuditStatusCompleted,
			QuestionSummary:         gap.Question,
			KnowledgeSpaceID:        gap.SpaceID,
			KnowledgeAnswerState:    "unsupported",
			KnowledgeNoSourceReason: gap.NoSourceReason,
			CreatedAt:               createdAt,
			KnowledgeEvidence: map[string]any{
				"answer_state": "unsupported",
				"summary":      "No supporting evidence recorded.",
				"gaps": []map[string]any{
					{"gap_id": gap.ID, "status": "open", "reason": gap.NoSourceReason},
				},
			},
		}); err != nil {
			t.Fatalf("Record returned error: %v", err)
		}
	}

	service := NewKnowledgeRepairService(gapService, auditService, knowledgeService)
	items, err := service.List("tenant-1", KnowledgeRepairFilter{SpaceID: DefaultKnowledgeSpaceID})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("repair count = %d, want 1", len(items))
	}
	item := items[0]
	if item.GapID != gap.ID || item.AnswerState != "unsupported" {
		t.Fatalf("repair item = %#v", item)
	}
	if item.OccurrenceCount != 2 {
		t.Fatalf("occurrence_count = %d, want 2", item.OccurrenceCount)
	}
	if item.Priority != KnowledgeRepairPriorityHigh {
		t.Fatalf("priority = %q, want high", item.Priority)
	}
	if len(item.LinkedDocuments) != 0 {
		t.Fatalf("linked documents = %#v, want none", item.LinkedDocuments)
	}
	if !containsRepairReason(item.PriorityReasons, "repeated_weak_answer") || !containsRepairReason(item.PriorityReasons, "no_linked_evidence") {
		t.Fatalf("priority_reasons = %#v", item.PriorityReasons)
	}
	if item.LastSeenAt.IsZero() || !item.LastSeenAt.Equal(base.Add(10*time.Minute)) {
		t.Fatalf("last_seen_at = %v, want latest audit timestamp", item.LastSeenAt)
	}
}

func TestKnowledgeRepairServiceProjectsLinkedResolvedDocumentAndFilters(t *testing.T) {
	knowledgeStore := NewInMemoryKnowledgeStore()
	knowledgeService := NewKnowledgeService(knowledgeStore)
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	auditService := NewAuditService(NewInMemoryAuditStore())
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	gapService.now = func() time.Time { return now }

	gap, err := gapService.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "Where is the smoke checklist?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := knowledgeService.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-smoke",
		Name:    "Smoke Checklist",
		Content: "Use scripts/smoke-conversation.ps1 after boot.",
		Metadata: map[string]string{
			"source_gap_id": gap.ID,
			"source_type":   "workbench_note",
		},
	}); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if _, err := gapService.UpdateStatus("tenant-1", gap.ID, KnowledgeGapResolved, "kb-smoke", "Covered by note"); err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}

	service := NewKnowledgeRepairService(gapService, auditService, knowledgeService)
	items, err := service.List("tenant-1", KnowledgeRepairFilter{
		SpaceID:        DefaultKnowledgeSpaceID,
		LinkedEvidence: true,
		UnresolvedOnly: false,
		WeakOnly:       false,
		Status:         string(KnowledgeGapResolved),
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("repair count = %d, want 1", len(items))
	}
	item := items[0]
	if item.Priority != KnowledgeRepairPriorityLow {
		t.Fatalf("priority = %q, want low", item.Priority)
	}
	if len(item.LinkedDocuments) != 1 || item.LinkedDocuments[0].DocumentID != "kb-smoke" {
		t.Fatalf("linked documents = %#v", item.LinkedDocuments)
	}
	if item.LinkedDocuments[0].Relation != "resolved_by" {
		t.Fatalf("linked relation = %#v, want resolved_by", item.LinkedDocuments[0])
	}
}

func TestKnowledgeRepairServiceSupportsUnresolvedAndWeakFilters(t *testing.T) {
	knowledgeService := NewKnowledgeService(NewInMemoryKnowledgeStore())
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	auditService := NewAuditService(NewInMemoryAuditStore())
	now := time.Date(2026, 7, 9, 13, 0, 0, 0, time.UTC)
	gapNow := now
	gapService.now = func() time.Time {
		current := gapNow
		gapNow = gapNow.Add(time.Nanosecond)
		return current
	}
	auditService.now = func() time.Time { return now }

	openGap, err := gapService.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "How do I verify startup?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("Create(open) returned error: %v", err)
	}
	resolvedGap, err := gapService.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "Where is the support runbook?",
		NoSourceReason: "review_gated_documents",
	})
	if err != nil {
		t.Fatalf("Create(resolved) returned error: %v", err)
	}
	if _, err := gapService.UpdateStatus("tenant-1", resolvedGap.ID, KnowledgeGapResolved, "", ""); err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	if _, err := auditService.Record("tenant-1", AuditRecord{
		ID:                      "audit-open-gap",
		ConversationID:          "conv-open",
		Status:                  AuditStatusCompleted,
		QuestionSummary:         openGap.Question,
		KnowledgeSpaceID:        openGap.SpaceID,
		KnowledgeAnswerState:    "unsupported",
		KnowledgeNoSourceReason: openGap.NoSourceReason,
		CreatedAt:               now.Add(5 * time.Minute),
		KnowledgeEvidence: map[string]any{
			"answer_state": "unsupported",
			"gaps": []map[string]any{
				{"gap_id": openGap.ID, "status": "open", "reason": openGap.NoSourceReason},
			},
		},
	}); err != nil {
		t.Fatalf("Record(open) returned error: %v", err)
	}
	if _, err := auditService.Record("tenant-1", AuditRecord{
		ID:                      "audit-resolved-gap",
		ConversationID:          "conv-resolved",
		Status:                  AuditStatusCompleted,
		QuestionSummary:         resolvedGap.Question,
		KnowledgeSpaceID:        resolvedGap.SpaceID,
		KnowledgeAnswerState:    "review_gated",
		KnowledgeNoSourceReason: resolvedGap.NoSourceReason,
		CreatedAt:               now.Add(10 * time.Minute),
		KnowledgeEvidence: map[string]any{
			"answer_state": "review_gated",
			"gaps": []map[string]any{
				{"gap_id": resolvedGap.ID, "status": "resolved", "reason": resolvedGap.NoSourceReason},
			},
		},
	}); err != nil {
		t.Fatalf("Record(resolved) returned error: %v", err)
	}

	service := NewKnowledgeRepairService(gapService, auditService, knowledgeService)
	items, err := service.List("tenant-1", KnowledgeRepairFilter{
		SpaceID:        DefaultKnowledgeSpaceID,
		UnresolvedOnly: true,
		WeakOnly:       true,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("repair count = %d, want 1", len(items))
	}
	if items[0].GapID != openGap.ID {
		t.Fatalf("repair item = %#v, want open gap only", items[0])
	}
}

func containsRepairReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
