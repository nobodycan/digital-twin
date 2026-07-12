package admin

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRepairEvalPromotionStoreCreatesAndIdempotentlyReplaysActiveRevision(t *testing.T) {
	store := NewInMemoryRepairEvalPromotionStore()
	candidate := promotionRevision("tenant-a", "gap-1", "verification-1", "policy-1")
	created, applied, err := store.PromoteRepairEval(candidate)
	if err != nil || !applied || created.Revision != 1 || !created.Active {
		t.Fatalf("first promotion = %#v, applied=%v, err=%v", created, applied, err)
	}
	replayed, applied, err := store.PromoteRepairEval(candidate)
	if err != nil || applied || replayed.ID != created.ID || replayed.Revision != 1 {
		t.Fatalf("replayed promotion = %#v, applied=%v, err=%v", replayed, applied, err)
	}
}

func TestRepairEvalPromotionStoreAppendsRevisionAndKeepsOneActive(t *testing.T) {
	store := NewInMemoryRepairEvalPromotionStore()
	first := promotionRevision("tenant-a", "gap-1", "verification-1", "policy-1")
	second := promotionRevision("tenant-a", "gap-1", "verification-2", "policy-1")
	if _, _, err := store.PromoteRepairEval(first); err != nil {
		t.Fatalf("first promotion returned error: %v", err)
	}
	active, applied, err := store.PromoteRepairEval(second)
	if err != nil || !applied || active.Revision != 2 || !active.Active || active.VerificationAttemptID != "verification-2" {
		t.Fatalf("second promotion = %#v, applied=%v, err=%v", active, applied, err)
	}
	items, err := store.ListRepairEvalPromotions("tenant-a", "gap-1", false, 20)
	if err != nil || len(items) != 2 || items[0].Revision != 2 || !items[0].Active || items[1].Revision != 1 || items[1].Active {
		t.Fatalf("history = %#v, err=%v", items, err)
	}
}

func TestRepairEvalPromotionStoreIsTenantScoped(t *testing.T) {
	store := NewInMemoryRepairEvalPromotionStore()
	for _, tenant := range []string{"tenant-a", "tenant-b"} {
		if _, _, err := store.PromoteRepairEval(promotionRevision(tenant, "gap-1", "verification-1", "policy-1")); err != nil {
			t.Fatalf("promotion for %s returned error: %v", tenant, err)
		}
	}
	active, err := store.GetActiveRepairEvalPromotion("tenant-a", "gap-1")
	if err != nil || active.TenantID != "tenant-a" {
		t.Fatalf("tenant-a active = %#v, err=%v", active, err)
	}
	if _, err := store.GetActiveRepairEvalPromotion("tenant-c", "gap-1"); !errors.Is(err, ErrRepairEvalPromotionNotFound) {
		t.Fatalf("cross-tenant lookup error = %v", err)
	}
}

func TestFileRepairEvalPromotionStoreSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	store := NewFileRepairEvalPromotionStore(dir)
	record := promotionRevision("tenant-a", "gap-1", "verification-1", "policy-1")
	if _, _, err := store.PromoteRepairEval(record); err != nil {
		t.Fatalf("promotion returned error: %v", err)
	}
	second := promotionRevision("tenant-a", "gap-1", "verification-2", "policy-2")
	if _, _, err := store.PromoteRepairEval(second); err != nil {
		t.Fatalf("second promotion returned error: %v", err)
	}
	reopened := NewFileRepairEvalPromotionStore(dir)
	active, err := reopened.GetActiveRepairEvalPromotion("tenant-a", "gap-1")
	if err != nil || active.ID != second.ID || active.Revision != 2 {
		t.Fatalf("reopened active = %#v, err=%v", active, err)
	}
}

func TestFileRepairEvalPromotionStoreKeepsConcurrentPromotionsConsistent(t *testing.T) {
	store := NewFileRepairEvalPromotionStore(t.TempDir())
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			candidate := promotionRevision("tenant-a", "gap-1", fmt.Sprintf("verification-%d", index), "policy-1")
			_, _, _ = store.PromoteRepairEval(candidate)
		}(i)
	}
	group.Wait()
	items, err := store.ListRepairEvalPromotions("tenant-a", "gap-1", false, 20)
	if err != nil || len(items) != 8 {
		t.Fatalf("concurrent history length = %d, items=%#v, err=%v", len(items), items, err)
	}
	active := 0
	for _, item := range items {
		if item.Active {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("active revisions = %d, want 1", active)
	}
}

func TestRepairEvalPromotionStoreAllowsExplicitTrendReadLimit(t *testing.T) {
	store := NewInMemoryRepairEvalPromotionStore()
	for index := 0; index < 105; index++ {
		record := promotionRevision("tenant-a", fmt.Sprintf("gap-%d", index), fmt.Sprintf("verification-%d", index), "policy-1")
		if _, _, err := store.PromoteRepairEval(record); err != nil {
			t.Fatalf("promotion %d returned error: %v", index, err)
		}
	}
	items, err := store.ListRepairEvalPromotions("tenant-a", "", false, 1000)
	if err != nil || len(items) != 105 {
		t.Fatalf("trend promotion items = %d, err=%v", len(items), err)
	}
}

func promotionRevision(tenantID, gapID, attemptID, policy string) RepairEvalPromotionRevision {
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	return RepairEvalPromotionRevision{
		ID: "promotion-" + gapID + "-" + attemptID, TenantID: tenantID, CaseID: "repair-eval-" + tenantID + "-" + gapID,
		GapID: gapID, SpaceID: DefaultKnowledgeSpaceID, Question: "How do I start?", VerificationAttemptID: attemptID,
		VerificationSnapshotFingerprint: "sha256:snapshot", MinimumSupportState: RepairEvalSupportPartiallySupported,
		PolicyFingerprint: policy, PromotedBy: "operator", PromotedAt: now,
	}
}
