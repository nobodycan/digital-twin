package admin

import (
	"context"
	"errors"
	"testing"
)

func TestRepairEvalPromotionServicePromotesCurrentResolvedVerification(t *testing.T) {
	fixture := newRepairEvalPromotionServiceFixture(t)
	request := RepairEvalPromotionRequest{
		GapID:                 fixture.gap.ID,
		VerificationAttemptID: fixture.attempt.ID,
		MinimumSupportState:   RepairEvalSupportPartiallySupported,
		RequiredDocumentIDs:   []string{"doc-a"},
		PromotedBy:            "operator-a",
	}

	record, applied, err := fixture.service.Promote(context.Background(), "tenant-a", request)
	if err != nil || !applied {
		t.Fatalf("promote returned record=%#v, applied=%v, err=%v", record, applied, err)
	}
	if record.CaseID != "repair-eval-tenant-a-"+fixture.gap.ID || record.Revision != 1 || !record.Active || record.SpaceID != fixture.gap.SpaceID {
		t.Fatalf("record = %#v", record)
	}
}

func TestRepairEvalPromotionServiceFailsClosedWhenStoreUnavailable(t *testing.T) {
	fixture := newRepairEvalPromotionServiceFixture(t)
	service := NewRepairEvalPromotionService(nil, fixture.gaps, fixture.knowledge, fixture.verification, nil)
	_, _, err := service.Promote(context.Background(), "tenant-a", RepairEvalPromotionRequest{
		GapID: fixture.gap.ID, VerificationAttemptID: fixture.attempt.ID, MinimumSupportState: RepairEvalSupportGrounded, PromotedBy: "operator-a",
	})
	if !errors.Is(err, ErrRepairEvalPromotionUnavailable) {
		t.Fatalf("err = %v, want unavailable error", err)
	}
}

func TestRepairEvalPromotionServiceRejectsUnsafeActorBeforePersistence(t *testing.T) {
	fixture := newRepairEvalPromotionServiceFixture(t)
	_, _, err := fixture.service.Promote(context.Background(), "tenant-a", RepairEvalPromotionRequest{
		GapID: fixture.gap.ID, VerificationAttemptID: fixture.attempt.ID, MinimumSupportState: RepairEvalSupportGrounded, PromotedBy: "operator/unsafe",
	})
	if !errors.Is(err, ErrRepairEvalPromotionInvalidRequest) {
		t.Fatalf("err = %v, want invalid request", err)
	}
}

func TestRepairEvalPromotionServiceRejectsUnresolvedGap(t *testing.T) {
	fixture := newRepairEvalPromotionServiceFixture(t)
	request := RepairEvalPromotionRequest{GapID: fixture.gap.ID, VerificationAttemptID: fixture.attempt.ID, MinimumSupportState: RepairEvalSupportGrounded, PromotedBy: "operator-a"}

	if _, _, err := fixture.service.Promote(context.Background(), "tenant-a", request); err != nil {
		t.Fatalf("initial promote returned error: %v", err)
	}
	if _, err := fixture.gaps.UpdateStatus("tenant-a", fixture.gap.ID, KnowledgeGapOpen, "", ""); err != nil {
		t.Fatalf("reopen returned error: %v", err)
	}
	_, _, err := fixture.service.Promote(context.Background(), "tenant-a", request)
	if !errors.Is(err, ErrRepairEvalPromotionGapNotEligible) {
		t.Fatalf("err = %v, want gap eligibility error", err)
	}
}

func TestRepairEvalPromotionServiceRejectsStaleVerification(t *testing.T) {
	fixture := newRepairEvalPromotionServiceFixture(t)
	document, err := fixture.knowledge.Get("tenant-a", "doc-a")
	if err != nil {
		t.Fatalf("get document returned error: %v", err)
	}
	document.ContentHash = "changed"
	if _, err := fixture.knowledgeStore.SaveKnowledge(document); err != nil {
		t.Fatalf("save changed document returned error: %v", err)
	}

	request := RepairEvalPromotionRequest{GapID: fixture.gap.ID, VerificationAttemptID: fixture.attempt.ID, MinimumSupportState: RepairEvalSupportGrounded, PromotedBy: "operator-a"}
	_, _, err = fixture.service.Promote(context.Background(), "tenant-a", request)
	if !errors.Is(err, ErrRepairEvalPromotionVerificationStale) {
		t.Fatalf("err = %v, want stale verification error", err)
	}
}

func TestRepairEvalPromotionServiceRejectsPendingRecurrence(t *testing.T) {
	fixture := newRepairEvalPromotionServiceFixture(t)
	recurrence := NewRepairRecurrenceService(NewInMemoryRepairRecurrenceStore(), fixture.gaps, fixture.verification)
	if _, _, err := recurrence.Detect("tenant-a", weakRecurrenceAudit(fixture.gap.Question, fixture.gap.SpaceID, fixture.gap.NoSourceReason, "audit-promotion")); err != nil {
		t.Fatalf("detect returned error: %v", err)
	}
	fixture.service.recurrence = &recurrence

	request := RepairEvalPromotionRequest{GapID: fixture.gap.ID, VerificationAttemptID: fixture.attempt.ID, MinimumSupportState: RepairEvalSupportGrounded, PromotedBy: "operator-a"}
	_, _, err := fixture.service.Promote(context.Background(), "tenant-a", request)
	if !errors.Is(err, ErrRepairEvalPromotionRecurrencePending) {
		t.Fatalf("err = %v, want pending recurrence error", err)
	}
}

func TestRepairEvalPromotionServiceRejectsInactiveOrCrossSpaceDocument(t *testing.T) {
	fixture := newRepairEvalPromotionServiceFixture(t)
	request := RepairEvalPromotionRequest{GapID: fixture.gap.ID, VerificationAttemptID: fixture.attempt.ID, MinimumSupportState: RepairEvalSupportGrounded, RequiredDocumentIDs: []string{"doc-pending"}, PromotedBy: "operator-a"}
	if _, err := fixture.knowledge.Upload("tenant-a", KnowledgeUpload{ID: "doc-pending", Name: "Pending", Content: "content", SpaceID: fixture.gap.SpaceID, ReviewStatus: KnowledgeReviewPending}); err != nil {
		t.Fatalf("upload pending document returned error: %v", err)
	}
	_, _, err := fixture.service.Promote(context.Background(), "tenant-a", request)
	if !errors.Is(err, ErrRepairEvalPromotionDocumentNotEligible) {
		t.Fatalf("err = %v, want document eligibility error", err)
	}
}

func TestKnowledgeRepairServiceProjectsPromotionState(t *testing.T) {
	fixture := newRepairEvalPromotionServiceFixture(t)
	store := NewInMemoryRepairEvalPromotionStore()
	promotionService := NewRepairEvalPromotionService(store, fixture.gaps, fixture.knowledge, fixture.verification, nil)
	repairService := NewKnowledgeRepairServiceWithRecurrenceAndPromotion(fixture.gaps, NewAuditService(NewInMemoryAuditStore()), fixture.knowledge, &fixture.verification, nil, store)

	items, err := repairService.List("tenant-a", KnowledgeRepairFilter{SpaceID: DefaultKnowledgeSpaceID})
	if err != nil || len(items) != 1 || items[0].PromotionState != RepairEvalPromotionNotPromoted {
		t.Fatalf("unpromoted items = %#v, err=%v", items, err)
	}
	if _, _, err := promotionService.Promote(context.Background(), "tenant-a", RepairEvalPromotionRequest{GapID: fixture.gap.ID, VerificationAttemptID: fixture.attempt.ID, MinimumSupportState: RepairEvalSupportGrounded, PromotedBy: "operator-a"}); err != nil {
		t.Fatalf("promote returned error: %v", err)
	}
	items, err = repairService.List("tenant-a", KnowledgeRepairFilter{SpaceID: DefaultKnowledgeSpaceID})
	if err != nil || len(items) != 1 || items[0].PromotionState != RepairEvalPromotionPromoted || items[0].PromotionRevision != 1 {
		t.Fatalf("promoted items = %#v, err=%v", items, err)
	}
	document, err := fixture.knowledge.Get("tenant-a", "doc-a")
	if err != nil {
		t.Fatalf("get document returned error: %v", err)
	}
	document.ContentHash = "changed-after-promotion"
	if _, err := fixture.knowledgeStore.SaveKnowledge(document); err != nil {
		t.Fatalf("save changed document returned error: %v", err)
	}
	items, err = repairService.List("tenant-a", KnowledgeRepairFilter{SpaceID: DefaultKnowledgeSpaceID})
	if err != nil || len(items) != 1 || items[0].PromotionState != RepairEvalPromotionStale {
		t.Fatalf("stale items = %#v, err=%v", items, err)
	}
}

type repairEvalPromotionServiceFixture struct {
	service        *RepairEvalPromotionService
	gaps           KnowledgeGapService
	knowledge      KnowledgeService
	knowledgeStore *InMemoryKnowledgeStore
	verification   RepairVerificationService
	gap            KnowledgeGap
	attempt        RepairVerificationAttempt
}

func newRepairEvalPromotionServiceFixture(t *testing.T) repairEvalPromotionServiceFixture {
	t.Helper()
	knowledgeStore := NewInMemoryKnowledgeStore()
	knowledge := NewKnowledgeService(knowledgeStore)
	if _, err := knowledge.Upload("tenant-a", KnowledgeUpload{ID: "doc-a", Name: "Start", Content: "How do I start? Run the app.", ReviewStatus: KnowledgeReviewActive}); err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	gaps := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	gap, err := gaps.Create("tenant-a", KnowledgeGapInput{SpaceID: DefaultKnowledgeSpaceID, Question: "How do I start?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("create gap returned error: %v", err)
	}
	if _, err := gaps.UpdateStatus("tenant-a", gap.ID, KnowledgeGapResolved, "doc-a", "covered"); err != nil {
		t.Fatalf("resolve gap returned error: %v", err)
	}
	verification := NewRepairVerificationService(NewInMemoryRepairVerificationStore(), gaps, knowledge, func(context.Context, string, RepairVerificationDiagnosticRequest) (RepairVerificationDiagnosticResponse, error) {
		return RepairVerificationDiagnosticResponse{Results: []RepairVerificationDiagnosticResult{{DocumentID: "doc-a", ChunkID: "doc-a-0", Rank: 1, ReviewStatus: string(KnowledgeReviewActive)}}}, nil
	})
	attempt, err := verification.Verify(context.Background(), "tenant-a", gap.ID)
	if err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	service := NewRepairEvalPromotionService(NewInMemoryRepairEvalPromotionStore(), gaps, knowledge, verification, nil)
	return repairEvalPromotionServiceFixture{service: &service, gaps: gaps, knowledge: knowledge, knowledgeStore: knowledgeStore, verification: verification, gap: gap, attempt: attempt}
}
