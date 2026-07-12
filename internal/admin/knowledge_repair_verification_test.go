package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRepairVerificationSnapshotFingerprintIsStableAcrossDocumentOrder(t *testing.T) {
	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	documents := []KnowledgeDocument{
		verificationDocument("doc-b", "hash-b", now.Add(time.Minute)),
		verificationDocument("doc-a", "hash-a", now),
	}
	reversed := []KnowledgeDocument{documents[1], documents[0]}

	first, err := repairVerificationSnapshotFingerprint(documents, DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("first fingerprint returned error: %v", err)
	}
	second, err := repairVerificationSnapshotFingerprint(reversed, DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("second fingerprint returned error: %v", err)
	}
	if first != second {
		t.Fatalf("fingerprints differ: %q != %q", first, second)
	}
}

func TestRepairVerificationSnapshotFingerprintChangesWhenReviewStateChanges(t *testing.T) {
	document := verificationDocument("doc-a", "hash-a", time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC))
	first, err := repairVerificationSnapshotFingerprint([]KnowledgeDocument{document}, DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("first fingerprint returned error: %v", err)
	}
	document.ReviewStatus = KnowledgeReviewPending
	second, err := repairVerificationSnapshotFingerprint([]KnowledgeDocument{document}, DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("second fingerprint returned error: %v", err)
	}
	if first == second {
		t.Fatalf("fingerprint did not change after review state mutation")
	}
}

func TestInMemoryRepairVerificationStoreIsAppendOnlyAndTenantScoped(t *testing.T) {
	store := NewInMemoryRepairVerificationStore()
	first := RepairVerificationAttempt{ID: "verification-1", TenantID: "tenant-a", GapID: "gap-1", CompletedAt: time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)}
	second := RepairVerificationAttempt{ID: "verification-2", TenantID: "tenant-a", GapID: "gap-1", CompletedAt: first.CompletedAt.Add(time.Minute)}
	otherTenant := RepairVerificationAttempt{ID: "verification-3", TenantID: "tenant-b", GapID: "gap-1", CompletedAt: first.CompletedAt.Add(2 * time.Minute)}
	for _, attempt := range []RepairVerificationAttempt{first, second, otherTenant} {
		if _, err := store.AppendRepairVerification(attempt); err != nil {
			t.Fatalf("append returned error: %v", err)
		}
	}
	items, err := store.ListRepairVerifications("tenant-a", "gap-1")
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if len(items) != 2 || items[0].ID != "verification-2" || items[1].ID != "verification-1" {
		t.Fatalf("items = %#v, want newest-first tenant-scoped attempts", items)
	}
}

func TestRepairVerificationServicePersistsPassedAttemptWithoutChangingGap(t *testing.T) {
	knowledgeStore := NewInMemoryKnowledgeStore()
	knowledgeService := NewKnowledgeService(knowledgeStore)
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	gap, err := gapService.Create("tenant-a", KnowledgeGapInput{SpaceID: DefaultKnowledgeSpaceID, Question: "How do I start?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("create gap returned error: %v", err)
	}
	if _, err := knowledgeService.Upload("tenant-a", KnowledgeUpload{ID: "doc-a", Name: "Start", Content: "How do I start? Run the app.", ReviewStatus: KnowledgeReviewActive}); err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	store := NewInMemoryRepairVerificationStore()
	service := NewRepairVerificationService(store, gapService, knowledgeService, func(context.Context, string, RepairVerificationDiagnosticRequest) (RepairVerificationDiagnosticResponse, error) {
		return RepairVerificationDiagnosticResponse{Results: []RepairVerificationDiagnosticResult{{DocumentID: "doc-a", DocumentName: "Start", ChunkID: "doc-a-0", Rank: 1, ReviewStatus: string(KnowledgeReviewActive), SourceType: "text"}}}, nil
	})

	attempt, err := service.Verify(context.Background(), "tenant-a", gap.ID)
	if err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	if attempt.Result != RepairVerificationPassed || attempt.AfterState != "grounded" || attempt.SourceCount != 1 {
		t.Fatalf("attempt = %#v", attempt)
	}
	unchanged, err := gapService.Get("tenant-a", gap.ID)
	if err != nil {
		t.Fatalf("get gap returned error: %v", err)
	}
	if unchanged.Status != KnowledgeGapOpen {
		t.Fatalf("gap status = %q, want open", unchanged.Status)
	}
}

func TestRepairVerificationServicePersistsUnsupportedAttempt(t *testing.T) {
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	gap, err := gapService.Create("tenant-a", KnowledgeGapInput{SpaceID: DefaultKnowledgeSpaceID, Question: "Unknown?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("create gap returned error: %v", err)
	}
	knowledgeService := NewKnowledgeService(NewInMemoryKnowledgeStore())
	store := NewInMemoryRepairVerificationStore()
	service := NewRepairVerificationService(store, gapService, knowledgeService, func(context.Context, string, RepairVerificationDiagnosticRequest) (RepairVerificationDiagnosticResponse, error) {
		return RepairVerificationDiagnosticResponse{NoSourceReason: "no_matching_chunks"}, nil
	})
	attempt, err := service.Verify(context.Background(), "tenant-a", gap.ID)
	if err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	if attempt.Result != RepairVerificationFailed || attempt.FailureReason != RepairVerificationUnsupported || attempt.AfterState != "unsupported" {
		t.Fatalf("attempt = %#v", attempt)
	}
}

func TestRepairVerificationServicePersistsRetrievalErrorAfterSnapshot(t *testing.T) {
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	gap, err := gapService.Create("tenant-a", KnowledgeGapInput{SpaceID: DefaultKnowledgeSpaceID, Question: "Retry?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("create gap returned error: %v", err)
	}
	knowledgeService := NewKnowledgeService(NewInMemoryKnowledgeStore())
	store := NewInMemoryRepairVerificationStore()
	service := NewRepairVerificationService(store, gapService, knowledgeService, func(context.Context, string, RepairVerificationDiagnosticRequest) (RepairVerificationDiagnosticResponse, error) {
		return RepairVerificationDiagnosticResponse{}, errors.New("backend detail must not be persisted")
	})
	attempt, err := service.Verify(context.Background(), "tenant-a", gap.ID)
	if err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	if attempt.FailureReason != RepairVerificationRetrievalError || attempt.AfterState != "unknown" || attempt.EvidenceFingerprint == "" {
		t.Fatalf("attempt = %#v", attempt)
	}
}

func TestFileRepairVerificationStoreSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	store := NewFileRepairVerificationStore(dir)
	attempt := RepairVerificationAttempt{ID: "verification-1", TenantID: "tenant-a", GapID: "gap-1", CompletedAt: time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)}
	if _, err := store.AppendRepairVerification(attempt); err != nil {
		t.Fatalf("append returned error: %v", err)
	}
	reopened := NewFileRepairVerificationStore(dir)
	items, err := reopened.ListRepairVerifications("tenant-a", "gap-1")
	if err != nil || len(items) != 1 || items[0].ID != attempt.ID {
		t.Fatalf("items = %#v, err = %v", items, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "repair_verifications.json")); err != nil {
		t.Fatalf("verification file missing: %v", err)
	}
}

func TestFileRepairVerificationStoreKeepsConcurrentAppends(t *testing.T) {
	store := NewFileRepairVerificationStore(t.TempDir())
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			_, _ = store.AppendRepairVerification(RepairVerificationAttempt{ID: fmt.Sprintf("verification-%d", index), TenantID: "tenant-a", GapID: "gap-1", CompletedAt: time.Date(2026, 7, 10, 8, index, 0, 0, time.UTC)})
		}(i)
	}
	group.Wait()
	items, err := store.ListRepairVerifications("tenant-a", "gap-1")
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if len(items) != 8 {
		t.Fatalf("items = %d, want 8", len(items))
	}
}

func TestRepairVerificationServiceListsAllAttemptsWithoutInboxLimit(t *testing.T) {
	store := NewInMemoryRepairVerificationStore()
	for index := 0; index < 105; index++ {
		attempt := RepairVerificationAttempt{
			ID: fmt.Sprintf("verification-%d", index), TenantID: "tenant-a", GapID: "gap-1",
			SpaceID: DefaultKnowledgeSpaceID, CompletedAt: time.Date(2026, 7, 11, 8, index%60, 0, 0, time.UTC),
		}
		if _, err := store.AppendRepairVerification(attempt); err != nil {
			t.Fatalf("append %d returned error: %v", index, err)
		}
	}
	service := RepairVerificationService{store: store}
	items, err := service.ListAll("tenant-a")
	if err != nil || len(items) != 105 {
		t.Fatalf("all verification attempts = %d, err=%v", len(items), err)
	}
}

func TestRepairVerificationProjectionBecomesStaleWhenActiveDocumentChanges(t *testing.T) {
	knowledgeStore := NewInMemoryKnowledgeStore()
	knowledgeService := NewKnowledgeService(knowledgeStore)
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	gap, err := gapService.Create("tenant-a", KnowledgeGapInput{SpaceID: DefaultKnowledgeSpaceID, Question: "How do I start?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("create gap returned error: %v", err)
	}
	if _, err := knowledgeService.Upload("tenant-a", KnowledgeUpload{ID: "doc-a", Name: "Start", Content: "How do I start? Run the app.", ReviewStatus: KnowledgeReviewActive}); err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	store := NewInMemoryRepairVerificationStore()
	service := NewRepairVerificationService(store, gapService, knowledgeService, func(context.Context, string, RepairVerificationDiagnosticRequest) (RepairVerificationDiagnosticResponse, error) {
		return RepairVerificationDiagnosticResponse{Results: []RepairVerificationDiagnosticResult{{DocumentID: "doc-a", ChunkID: "doc-a-0", Rank: 1, ReviewStatus: string(KnowledgeReviewActive)}}}, nil
	})
	if _, err := service.Verify(context.Background(), "tenant-a", gap.ID); err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	document, err := knowledgeService.Get("tenant-a", "doc-a")
	if err != nil {
		t.Fatalf("get document returned error: %v", err)
	}
	document.ContentHash = "changed"
	if _, err := knowledgeStore.SaveKnowledge(document); err != nil {
		t.Fatalf("save changed document returned error: %v", err)
	}
	projection, err := service.Project("tenant-a", gap, []KnowledgeDocument{document})
	if err != nil {
		t.Fatalf("project returned error: %v", err)
	}
	if projection.State != RepairVerificationStale {
		t.Fatalf("state = %q, want stale", projection.State)
	}
}

func verificationDocument(id, hash string, updatedAt time.Time) KnowledgeDocument {
	return KnowledgeDocument{
		ID: id, SpaceID: DefaultKnowledgeSpaceID, Status: KnowledgeReady,
		ReviewStatus: KnowledgeReviewActive, ContentHash: hash, UpdatedAt: updatedAt,
		Metadata: map[string]string{KnowledgeMetadataLexicalReady: "ready", KnowledgeMetadataVectorStatus: KnowledgeVectorMissing},
	}
}
