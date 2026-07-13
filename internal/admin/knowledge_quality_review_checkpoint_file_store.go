package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	qualityReviewMaxRecords     = 10000
	qualityReviewMaxFileBytes   = 32 << 20
	qualityReviewMaxRecordBytes = 64 << 10
)

type qualityReviewCheckpointFileEnvelope struct {
	SchemaVersion int               `json:"schema_version"`
	Records       []json.RawMessage `json:"records"`
}

type FileQualityReviewCheckpointStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileQualityReviewCheckpointStore(dir string) *FileQualityReviewCheckpointStore {
	return &FileQualityReviewCheckpointStore{dir: dir}
}

func (s *FileQualityReviewCheckpointStore) FindQualityReviewCheckpointByIdempotency(tenantID, keyHash string) (QualityReviewCheckpoint, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, _, err := s.loadRecords()
	if err != nil {
		return QualityReviewCheckpoint{}, false, err
	}
	for _, record := range records {
		if record.TenantID == tenantID && record.IdempotencyKeyHash == keyHash {
			return cloneQualityReviewCheckpoint(record), true, nil
		}
	}
	return QualityReviewCheckpoint{}, false, nil
}

func (s *FileQualityReviewCheckpointStore) AppendQualityReviewCheckpoint(record QualityReviewCheckpoint) (QualityReviewCheckpoint, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, excluded, err := s.loadRecords()
	if err != nil {
		return QualityReviewCheckpoint{}, false, err
	}
	if excluded > 0 {
		return QualityReviewCheckpoint{}, false, fmt.Errorf("%w: invalid historical record", ErrQualityReviewUnavailable)
	}
	for _, existing := range records {
		if existing.TenantID != record.TenantID || existing.IdempotencyKeyHash != record.IdempotencyKeyHash {
			continue
		}
		if existing.RequestFingerprint != record.RequestFingerprint {
			return QualityReviewCheckpoint{}, false, ErrQualityReviewIdempotencyConflict
		}
		return cloneQualityReviewCheckpoint(existing), false, nil
	}
	if len(records) >= qualityReviewMaxRecords {
		return QualityReviewCheckpoint{}, false, ErrQualityReviewCapacity
	}
	records = append(records, cloneQualityReviewCheckpoint(record))
	if err := s.saveRecords(records); err != nil {
		return QualityReviewCheckpoint{}, false, err
	}
	return cloneQualityReviewCheckpoint(record), true, nil
}

func (s *FileQualityReviewCheckpointStore) ListQualityReviewCheckpoints(tenantID string) ([]QualityReviewCheckpoint, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, excluded, err := s.loadRecords()
	if err != nil {
		return nil, 0, err
	}
	items := make([]QualityReviewCheckpoint, 0, len(records))
	for _, record := range records {
		if record.TenantID == tenantID {
			items = append(items, cloneQualityReviewCheckpoint(record))
		}
	}
	return items, excluded, nil
}

func (s *FileQualityReviewCheckpointStore) loadRecords() ([]QualityReviewCheckpoint, int, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if len(data) > qualityReviewMaxFileBytes {
		return nil, 0, fmt.Errorf("quality review checkpoint file exceeds %d bytes", qualityReviewMaxFileBytes)
	}
	var envelope qualityReviewCheckpointFileEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, 0, err
	}
	if envelope.SchemaVersion != qualityReviewSchemaVersion || envelope.Records == nil {
		return nil, 0, fmt.Errorf("unsupported quality review checkpoint envelope")
	}
	records := make([]QualityReviewCheckpoint, 0, len(envelope.Records))
	excluded := 0
	for _, raw := range envelope.Records {
		var record QualityReviewCheckpoint
		if len(raw) > qualityReviewMaxRecordBytes || json.Unmarshal(raw, &record) != nil || !validStoredQualityReviewCheckpoint(record) {
			excluded++
			continue
		}
		records = append(records, record)
	}
	return records, excluded, nil
}

func (s *FileQualityReviewCheckpointStore) saveRecords(records []QualityReviewCheckpoint) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	rawRecords := make([]json.RawMessage, 0, len(records))
	for _, record := range records {
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		rawRecords = append(rawRecords, encoded)
	}
	encoded, err := json.MarshalIndent(qualityReviewCheckpointFileEnvelope{SchemaVersion: qualityReviewSchemaVersion, Records: rawRecords}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "quality_review_checkpoints.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(append(encoded, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path())
}

func (s *FileQualityReviewCheckpointStore) path() string {
	return filepath.Join(s.dir, "quality_review_checkpoints.json")
}

func validStoredQualityReviewCheckpoint(record QualityReviewCheckpoint) bool {
	return record.SchemaVersion == qualityReviewSchemaVersion &&
		record.ID != "" && record.TenantID != "" && !record.CreatedAt.IsZero() &&
		record.Filter.From != "" && record.Filter.To != "" && record.Filter.Timezone != "" &&
		record.Filter.WindowDays > 0 && !record.Filter.Start.IsZero() && !record.Filter.End.IsZero() &&
		record.IdempotencyKeyHash != "" && record.RequestFingerprint != ""
}
