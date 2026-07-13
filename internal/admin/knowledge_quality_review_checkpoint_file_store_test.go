package admin

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFileQualityReviewCheckpointStoreExcludesOversizedHistoricalRecordAndBlocksAppend(t *testing.T) {
	dir := t.TempDir()
	store := NewFileQualityReviewCheckpointStore(dir)
	record := qualityReviewCheckpointForTest("oversized", "2026-07-10", "2026-07-12", time.Now().UTC())
	record.SchemaVersion = qualityReviewSchemaVersion
	record.IdempotencyKeyHash = "key"
	record.RequestFingerprint = "fingerprint"
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	raw = append(raw[:len(raw)-1], []byte(`,"padding":"`+strings.Repeat("x", qualityReviewMaxRecordBytes)+`"}`)...)
	envelope, err := json.Marshal(qualityReviewCheckpointFileEnvelope{SchemaVersion: qualityReviewSchemaVersion, Records: []json.RawMessage{raw}})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := os.WriteFile(store.path(), envelope, 0o600); err != nil {
		t.Fatalf("write envelope: %v", err)
	}
	if _, excluded, err := store.ListQualityReviewCheckpoints("tenant-a"); err != nil || excluded != 1 {
		t.Fatalf("list = excluded %d, err %v", excluded, err)
	}
	_, _, err = store.AppendQualityReviewCheckpoint(qualityReviewCheckpointForTest("new", "2026-07-10", "2026-07-12", time.Now().UTC()))
	if !errors.Is(err, ErrQualityReviewUnavailable) {
		t.Fatalf("append error = %v, want unavailable", err)
	}
}

func TestFileQualityReviewCheckpointStoreReopensPersistedRecords(t *testing.T) {
	dir := t.TempDir()
	store := NewFileQualityReviewCheckpointStore(dir)
	record := qualityReviewCheckpointForTest("checkpoint-1", "2026-07-10", "2026-07-12", time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC))
	record.SchemaVersion = qualityReviewSchemaVersion
	record.IdempotencyKeyHash = "key-hash"
	record.RequestFingerprint = "request-fingerprint"
	record.Snapshot.ProjectedAt = record.CreatedAt
	if _, created, err := store.AppendQualityReviewCheckpoint(record); err != nil || !created {
		t.Fatalf("append = created:%v err:%v", created, err)
	}

	reopened := NewFileQualityReviewCheckpointStore(dir)
	items, excluded, err := reopened.ListQualityReviewCheckpoints("tenant-a")
	if err != nil {
		t.Fatalf("list after reopen returned error: %v", err)
	}
	if excluded != 0 || len(items) != 1 || items[0].ID != "checkpoint-1" || items[0].Snapshot.ProjectedAt.IsZero() {
		t.Fatalf("items = %#v, excluded=%d", items, excluded)
	}
}

func TestFileQualityReviewCheckpointStoreRejectsRecordCapacityBeforeRewrite(t *testing.T) {
	dir := t.TempDir()
	store := NewFileQualityReviewCheckpointStore(dir)
	records := make([]QualityReviewCheckpoint, 0, qualityReviewMaxRecords)
	for index := 0; index < qualityReviewMaxRecords; index++ {
		record := qualityReviewCheckpointForTest("checkpoint-capacity-"+time.Unix(int64(index), 0).UTC().Format("150405.000000000"), "2026-07-10", "2026-07-12", time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC))
		record.SchemaVersion = qualityReviewSchemaVersion
		record.IdempotencyKeyHash = "key-hash-" + time.Unix(int64(index), 0).UTC().Format("150405.000000000")
		record.RequestFingerprint = "request-" + record.IdempotencyKeyHash
		record.Snapshot.ProjectedAt = record.CreatedAt
		records = append(records, record)
	}
	if err := store.saveRecords(records); err != nil {
		t.Fatalf("seed save records: %v", err)
	}
	before, _, err := store.ListQualityReviewCheckpoints("tenant-a")
	if err != nil {
		t.Fatalf("list before capacity append: %v", err)
	}
	overflow := qualityReviewCheckpointForTest("checkpoint-overflow", "2026-07-10", "2026-07-12", time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC))
	overflow.SchemaVersion = qualityReviewSchemaVersion
	overflow.IdempotencyKeyHash = "key-hash-overflow"
	overflow.RequestFingerprint = "request-overflow"
	overflow.Snapshot.ProjectedAt = overflow.CreatedAt
	if _, _, err := store.AppendQualityReviewCheckpoint(overflow); !errors.Is(err, ErrQualityReviewCapacity) {
		t.Fatalf("overflow append error = %v, want capacity error", err)
	}
	after, _, err := store.ListQualityReviewCheckpoints("tenant-a")
	if err != nil {
		t.Fatalf("list after capacity append: %v", err)
	}
	if len(before) != qualityReviewMaxRecords || len(after) != len(before) {
		t.Fatalf("records before=%d after=%d", len(before), len(after))
	}
}
