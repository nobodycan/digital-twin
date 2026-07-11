package admin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var errTestRecurrenceStore = errors.New("test recurrence store failure")

func TestInMemoryRepairRecurrenceStoreDeduplicatesObservationsAndConsolidatesPending(t *testing.T) {
	store := NewInMemoryRepairRecurrenceStore()
	first := recurrenceObservation("tenant-a", "gap-1", "audit-1")
	record := recurrenceRecord("recurrence-1", "tenant-a", "gap-1", first.AuditID)
	created, applied, err := store.ObserveRepairRecurrence(first, record)
	if err != nil || !applied || created.ID != record.ID {
		t.Fatalf("first observe = %#v, applied=%v, err=%v", created, applied, err)
	}
	duplicate, applied, err := store.ObserveRepairRecurrence(first, record)
	if err != nil || applied || duplicate.OccurrenceCount != 1 {
		t.Fatalf("duplicate observe = %#v, applied=%v, err=%v", duplicate, applied, err)
	}
	second := recurrenceObservation("tenant-a", "gap-1", "audit-2")
	record.LatestAuditID = second.AuditID
	record.OccurrenceCount = 2
	consolidated, applied, err := store.ObserveRepairRecurrence(second, record)
	if err != nil || !applied || consolidated.ID != record.ID || consolidated.OccurrenceCount != 2 || consolidated.LatestAuditID != second.AuditID {
		t.Fatalf("consolidated observe = %#v, applied=%v, err=%v", consolidated, applied, err)
	}
	items, err := store.ListRepairRecurrences("tenant-a", "gap-1", "", 20)
	if err != nil || len(items) != 1 || items[0].OccurrenceCount != 2 {
		t.Fatalf("items = %#v, err=%v", items, err)
	}
}

func TestInMemoryRepairRecurrenceStoreIsTenantScoped(t *testing.T) {
	store := NewInMemoryRepairRecurrenceStore()
	for _, tenant := range []string{"tenant-a", "tenant-b"} {
		record := recurrenceRecord("recurrence-"+tenant, tenant, "gap-1", "audit-1")
		if _, _, err := store.ObserveRepairRecurrence(recurrenceObservation(tenant, "gap-1", "audit-1"), record); err != nil {
			t.Fatalf("observe %s returned error: %v", tenant, err)
		}
	}
	items, err := store.ListRepairRecurrences("tenant-a", "gap-1", "", 20)
	if err != nil || len(items) != 1 || items[0].TenantID != "tenant-a" {
		t.Fatalf("items = %#v, err=%v", items, err)
	}
}

func TestFileRepairRecurrenceStoreSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	store := NewFileRepairRecurrenceStore(dir)
	record := recurrenceRecord("recurrence-1", "tenant-a", "gap-1", "audit-1")
	if _, _, err := store.ObserveRepairRecurrence(recurrenceObservation("tenant-a", "gap-1", "audit-1"), record); err != nil {
		t.Fatalf("observe returned error: %v", err)
	}
	reopened := NewFileRepairRecurrenceStore(dir)
	items, err := reopened.ListRepairRecurrences("tenant-a", "gap-1", "", 20)
	if err != nil || len(items) != 1 || items[0].ID != record.ID {
		t.Fatalf("reopened items = %#v, err=%v", items, err)
	}
	if _, applied, err := reopened.ObserveRepairRecurrence(recurrenceObservation("tenant-a", "gap-1", "audit-1"), record); err != nil || applied {
		t.Fatalf("replayed observation applied=%v, err=%v", applied, err)
	}
}

func TestFileRepairRecurrenceStoreKeepsConcurrentObservations(t *testing.T) {
	store := NewFileRepairRecurrenceStore(t.TempDir())
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			id := fmt.Sprintf("audit-%d", index)
			_, _, _ = store.ObserveRepairRecurrence(recurrenceObservation("tenant-a", "gap-1", id), recurrenceRecord("recurrence-1", "tenant-a", "gap-1", id))
		}(i)
	}
	group.Wait()
	items, err := store.ListRepairRecurrences("tenant-a", "gap-1", "", 20)
	if err != nil || len(items) != 1 || items[0].OccurrenceCount != 8 {
		t.Fatalf("items = %#v, err=%v", items, err)
	}
}

func recurrenceObservation(tenantID, gapID, auditID string) RepairRecurrenceObservation {
	return RepairRecurrenceObservation{TenantID: tenantID, GapID: gapID, AuditID: auditID, RecordedAt: time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)}
}

func recurrenceRecord(id, tenantID, gapID, auditID string) RepairRecurrence {
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	return RepairRecurrence{ID: id, TenantID: tenantID, GapID: gapID, SpaceID: DefaultKnowledgeSpaceID, Status: RepairRecurrenceSuspected, MatchedBy: RepairRecurrenceMatchExact, FirstAuditID: auditID, LatestAuditID: auditID, OccurrenceCount: 1, AnswerState: "unsupported", NoSourceReason: "no_matching_chunks", CreatedAt: now, UpdatedAt: now}
}

func TestRepairRecurrenceServiceDetectsExactVerifiedResolvedRepair(t *testing.T) {
	gapService, verificationService, gap := recurrenceServiceFixture(t, true)
	store := NewInMemoryRepairRecurrenceStore()
	service := NewRepairRecurrenceService(store, gapService, verificationService)
	audit := weakRecurrenceAudit(gap.Question, gap.SpaceID, gap.NoSourceReason, "audit-1")
	record, handled, err := service.Detect("tenant-a", audit)
	if err != nil || !handled || record.Status != RepairRecurrenceSuspected || record.GapID != gap.ID || record.OccurrenceCount != 1 {
		t.Fatalf("record = %#v, handled=%v, err=%v", record, handled, err)
	}
}

func TestRepairRecurrenceServiceUsesEligibleExplicitGapID(t *testing.T) {
	gapService, verificationService, gap := recurrenceServiceFixture(t, true)
	service := NewRepairRecurrenceService(NewInMemoryRepairRecurrenceStore(), gapService, verificationService)
	audit := weakRecurrenceAudit("different wording", gap.SpaceID, "different_reason", "audit-explicit")
	audit.KnowledgeEvidence = map[string]any{"gaps": []map[string]any{{"gap_id": gap.ID}}}
	record, handled, err := service.Detect("tenant-a", audit)
	if err != nil || !handled || record.MatchedBy != RepairRecurrenceMatchExplicit || record.GapID != gap.ID {
		t.Fatalf("record = %#v, handled=%v, err=%v", record, handled, err)
	}
}

func TestRepairRecurrenceServiceRejectsStaleAndAmbiguousRepairs(t *testing.T) {
	gapService, verificationService, gap := recurrenceServiceFixture(t, false)
	store := NewInMemoryRepairRecurrenceStore()
	service := NewRepairRecurrenceService(store, gapService, verificationService)
	audit := weakRecurrenceAudit(gap.Question, gap.SpaceID, gap.NoSourceReason, "audit-stale")
	if _, handled, err := service.Detect("tenant-a", audit); err != nil || handled {
		t.Fatalf("stale detect handled=%v, err=%v", handled, err)
	}
	ambiguousGapService, ambiguousVerificationService, secondBase := recurrenceServiceFixture(t, true)
	second := KnowledgeGap{ID: "gap-secondary", TenantID: "tenant-a", SpaceID: secondBase.SpaceID, Question: secondBase.Question, NoSourceReason: secondBase.NoSourceReason, Status: KnowledgeGapOpen, CreatedAt: secondBase.CreatedAt.Add(time.Second), UpdatedAt: secondBase.UpdatedAt.Add(time.Second)}
	if _, err := ambiguousGapService.store.SaveKnowledgeGap(second); err != nil {
		t.Fatalf("save second gap returned error: %v", err)
	}
	if _, err := ambiguousGapService.UpdateStatus("tenant-a", second.ID, KnowledgeGapResolved, "doc-a", "covered"); err != nil {
		t.Fatalf("resolve second gap returned error: %v", err)
	}
	if _, err := ambiguousVerificationService.Verify(context.Background(), "tenant-a", second.ID); err != nil {
		t.Fatalf("verify second gap returned error: %v", err)
	}
	ambiguousService := NewRepairRecurrenceService(store, ambiguousGapService, ambiguousVerificationService)
	allGaps, err := ambiguousGapService.List("tenant-a", secondBase.SpaceID)
	if err != nil || len(allGaps) != 2 {
		t.Fatalf("ambiguous candidates = %#v, err=%v", allGaps, err)
	}
	if _, handled, err := ambiguousService.Detect("tenant-a", weakRecurrenceAudit(secondBase.Question, secondBase.SpaceID, secondBase.NoSourceReason, "audit-ambiguous")); err != nil || handled {
		t.Fatalf("ambiguous detect handled=%v, err=%v", handled, err)
	}
}

func TestRepairRecurrenceServiceConfirmReopensGapAndDismissLeavesGapUnchanged(t *testing.T) {
	gapService, verificationService, gap := recurrenceServiceFixture(t, true)
	store := NewInMemoryRepairRecurrenceStore()
	service := NewRepairRecurrenceService(store, gapService, verificationService)
	record, _, err := service.Detect("tenant-a", weakRecurrenceAudit(gap.Question, gap.SpaceID, gap.NoSourceReason, "audit-confirm"))
	if err != nil {
		t.Fatalf("detect returned error: %v", err)
	}
	if _, err := service.Dismiss("tenant-a", record.ID, "known transient issue"); err != nil {
		t.Fatalf("dismiss returned error: %v", err)
	}
	unchanged, err := gapService.Get("tenant-a", gap.ID)
	if err != nil || unchanged.Status != KnowledgeGapResolved {
		t.Fatalf("dismiss changed gap = %#v, err=%v", unchanged, err)
	}
	second, _, err := service.Detect("tenant-a", weakRecurrenceAudit(gap.Question, gap.SpaceID, gap.NoSourceReason, "audit-confirm-2"))
	if err != nil {
		t.Fatalf("second detect returned error: %v", err)
	}
	if _, err := service.Confirm("tenant-a", second.ID, "operator"); err != nil {
		t.Fatalf("confirm returned error: %v", err)
	}
	reopened, err := gapService.Get("tenant-a", gap.ID)
	if err != nil || reopened.Status != KnowledgeGapOpen || reopened.ResolvedByDocumentID != "" {
		t.Fatalf("confirmed gap = %#v, err=%v", reopened, err)
	}
}

func TestRepairRecurrenceServiceDeduplicatesSameAudit(t *testing.T) {
	gapService, verificationService, gap := recurrenceServiceFixture(t, true)
	store := NewInMemoryRepairRecurrenceStore()
	service := NewRepairRecurrenceService(store, gapService, verificationService)
	audit := weakRecurrenceAudit(gap.Question, gap.SpaceID, gap.NoSourceReason, "audit-duplicate")
	_, handled, err := service.Detect("tenant-a", audit)
	if err != nil || !handled {
		t.Fatalf("first detect handled=%v, err=%v", handled, err)
	}
	second, handled, err := service.Detect("tenant-a", audit)
	if err != nil || !handled || second.OccurrenceCount != 1 {
		t.Fatalf("second record = %#v, handled=%v, err=%v", second, handled, err)
	}
}

func TestKnowledgeRepairServiceProjectsRecurrenceStateSeparatelyFromVerification(t *testing.T) {
	knowledgeStore := NewInMemoryKnowledgeStore()
	knowledgeService := NewKnowledgeService(knowledgeStore)
	if _, err := knowledgeService.Upload("tenant-a", KnowledgeUpload{ID: "doc-a", Name: "Repair", Content: "How do I start? Run the app.", ReviewStatus: KnowledgeReviewActive}); err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	gap, err := gapService.Create("tenant-a", KnowledgeGapInput{SpaceID: DefaultKnowledgeSpaceID, Question: "How do I start?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("create gap returned error: %v", err)
	}
	if _, err := gapService.UpdateStatus("tenant-a", gap.ID, KnowledgeGapResolved, "doc-a", "covered"); err != nil {
		t.Fatalf("resolve gap returned error: %v", err)
	}
	verification := NewRepairVerificationService(NewInMemoryRepairVerificationStore(), gapService, knowledgeService, func(context.Context, string, RepairVerificationDiagnosticRequest) (RepairVerificationDiagnosticResponse, error) {
		return RepairVerificationDiagnosticResponse{Results: []RepairVerificationDiagnosticResult{{DocumentID: "doc-a", ChunkID: "doc-a-0", Rank: 1, ReviewStatus: string(KnowledgeReviewActive)}}}, nil
	})
	if _, err := verification.Verify(context.Background(), "tenant-a", gap.ID); err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	recurrence := NewRepairRecurrenceService(NewInMemoryRepairRecurrenceStore(), gapService, verification)
	if _, _, err := recurrence.Detect("tenant-a", weakRecurrenceAudit(gap.Question, gap.SpaceID, gap.NoSourceReason, "audit-projection")); err != nil {
		t.Fatalf("detect returned error: %v", err)
	}
	repairService := NewKnowledgeRepairServiceWithRecurrence(gapService, NewAuditService(NewInMemoryAuditStore()), knowledgeService, &verification, &recurrence)
	items, err := repairService.List("tenant-a", KnowledgeRepairFilter{SpaceID: DefaultKnowledgeSpaceID})
	if err != nil || len(items) != 1 || items[0].VerificationState != RepairVerificationVerified || items[0].RecurrenceState != RepairRecurrenceProjectionSuspected || items[0].RecurrenceCount != 1 {
		t.Fatalf("items = %#v, err=%v", items, err)
	}
}

func TestRepairRecurrenceServiceReturnsDetectionStoreErrorForCallerFallback(t *testing.T) {
	gapService, verificationService, gap := recurrenceServiceFixture(t, true)
	service := NewRepairRecurrenceService(failingRepairRecurrenceStore{}, gapService, verificationService)
	if _, handled, err := service.Detect("tenant-a", weakRecurrenceAudit(gap.Question, gap.SpaceID, gap.NoSourceReason, "audit-store-error")); !errors.Is(err, errTestRecurrenceStore) || handled {
		t.Fatalf("handled=%v, err=%v", handled, err)
	}
}

type failingRepairRecurrenceStore struct{}

func (failingRepairRecurrenceStore) ObserveRepairRecurrence(RepairRecurrenceObservation, RepairRecurrence) (RepairRecurrence, bool, error) {
	return RepairRecurrence{}, false, errTestRecurrenceStore
}

func (failingRepairRecurrenceStore) GetRepairRecurrence(string, string) (RepairRecurrence, error) {
	return RepairRecurrence{}, errTestRecurrenceStore
}

func (failingRepairRecurrenceStore) SaveRepairRecurrence(RepairRecurrence) (RepairRecurrence, error) {
	return RepairRecurrence{}, errTestRecurrenceStore
}

func (failingRepairRecurrenceStore) ListRepairRecurrences(string, string, string, int) ([]RepairRecurrence, error) {
	return nil, errTestRecurrenceStore
}

func recurrenceServiceFixture(t *testing.T, current bool) (KnowledgeGapService, RepairVerificationService, KnowledgeGap) {
	t.Helper()
	knowledgeStore := NewInMemoryKnowledgeStore()
	knowledgeService := NewKnowledgeService(knowledgeStore)
	if _, err := knowledgeService.Upload("tenant-a", KnowledgeUpload{ID: "doc-a", Name: "Repair", Content: "How do I start? Run the app.", ReviewStatus: KnowledgeReviewActive}); err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	gap, err := gapService.Create("tenant-a", KnowledgeGapInput{SpaceID: DefaultKnowledgeSpaceID, Question: "How do I start?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("create gap returned error: %v", err)
	}
	if _, err := gapService.UpdateStatus("tenant-a", gap.ID, KnowledgeGapResolved, "doc-a", "covered"); err != nil {
		t.Fatalf("resolve gap returned error: %v", err)
	}
	verificationStore := NewInMemoryRepairVerificationStore()
	verificationService := NewRepairVerificationService(verificationStore, gapService, knowledgeService, func(context.Context, string, RepairVerificationDiagnosticRequest) (RepairVerificationDiagnosticResponse, error) {
		return RepairVerificationDiagnosticResponse{Results: []RepairVerificationDiagnosticResult{{DocumentID: "doc-a", ChunkID: "doc-a-0", Rank: 1, ReviewStatus: string(KnowledgeReviewActive)}}}, nil
	})
	if _, err := verificationService.Verify(context.Background(), "tenant-a", gap.ID); err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	if !current {
		document, err := knowledgeService.Get("tenant-a", "doc-a")
		if err != nil {
			t.Fatalf("get document returned error: %v", err)
		}
		document.ContentHash = "changed"
		if _, err := knowledgeStore.SaveKnowledge(document); err != nil {
			t.Fatalf("save changed document returned error: %v", err)
		}
	}
	return gapService, verificationService, gap
}

func weakRecurrenceAudit(question, spaceID, reason, id string) AuditRecord {
	return AuditRecord{ID: id, ConversationID: "conversation-" + id, QuestionSummary: question, KnowledgeSpaceID: spaceID, KnowledgeAnswerState: "unsupported", KnowledgeNoSourceReason: reason, CreatedAt: time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)}
}
