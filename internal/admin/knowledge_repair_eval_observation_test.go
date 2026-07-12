package admin

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestInMemoryRepairEvalObservationStoreIsIdempotentAndTenantScoped(t *testing.T) {
	store := NewInMemoryRepairEvalObservationStore()
	first := repairEvalObservation("tenant-a", "run-1", "case-1", "promotion-1")
	created, applied, err := store.AppendRepairEvalObservation(first)
	if err != nil || !applied || created.ID != first.ID {
		t.Fatalf("first append = %#v, applied=%v, err=%v", created, applied, err)
	}
	replayed, applied, err := store.AppendRepairEvalObservation(first)
	if err != nil || applied || replayed.ID != first.ID {
		t.Fatalf("replayed append = %#v, applied=%v, err=%v", replayed, applied, err)
	}
	other := repairEvalObservation("tenant-b", "run-1", "case-1", "promotion-1")
	if _, _, err := store.AppendRepairEvalObservation(other); err != nil {
		t.Fatalf("other tenant append returned error: %v", err)
	}
	items, truncated, err := store.ListRepairEvalObservations("tenant-a", 10)
	if err != nil || truncated || len(items) != 1 || items[0].TenantID != "tenant-a" {
		t.Fatalf("tenant-a items = %#v, truncated=%v, err=%v", items, truncated, err)
	}
}

func TestFileRepairEvalObservationStoreSurvivesReopenAndConcurrentAppends(t *testing.T) {
	store := NewFileRepairEvalObservationStore(t.TempDir())
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			record := repairEvalObservation("tenant-a", "run-1", fmt.Sprintf("case-%d", index), fmt.Sprintf("promotion-%d", index))
			if _, _, err := store.AppendRepairEvalObservation(record); err != nil {
				t.Errorf("append %d returned error: %v", index, err)
			}
		}(i)
	}
	group.Wait()
	reopened := NewFileRepairEvalObservationStore(store.dir)
	items, truncated, err := reopened.ListRepairEvalObservations("tenant-a", 20)
	if err != nil || truncated || len(items) != 8 {
		t.Fatalf("reopened items = %#v, truncated=%v, err=%v", items, truncated, err)
	}
}

func TestRepairEvalObservationValidationRejectsRawFailureDetails(t *testing.T) {
	store := NewInMemoryRepairEvalObservationStore()
	record := repairEvalObservation("tenant-a", "run-1", "case-1", "promotion-1")
	record.FailureReason = "provider stack trace: secret"
	if _, _, err := store.AppendRepairEvalObservation(record); !errors.Is(err, ErrRepairEvalObservationInvalid) {
		t.Fatalf("error = %v, want %v", err, ErrRepairEvalObservationInvalid)
	}
}

func repairEvalObservation(tenantID, runID, caseID, promotionID string) RepairEvalObservation {
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	return RepairEvalObservation{
		ID:                "observation-" + tenantID + "-" + caseID,
		TenantID:          tenantID,
		RunID:             runID,
		CaseID:            caseID,
		GapID:             "gap-1",
		SpaceID:           DefaultKnowledgeSpaceID,
		PromotionID:       promotionID,
		PromotionRevision: 1,
		ObservedAt:        now,
		Status:            RepairEvalObservationPassed,
	}
}
