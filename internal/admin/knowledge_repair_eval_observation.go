package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrRepairEvalObservationInvalid = errors.New("invalid repair eval observation")

type RepairEvalObservationStatus string

const (
	RepairEvalObservationPassed      RepairEvalObservationStatus = "passed"
	RepairEvalObservationFailed      RepairEvalObservationStatus = "failed"
	RepairEvalObservationUnavailable RepairEvalObservationStatus = "unavailable"
)

type RepairEvalObservation struct {
	ID                string                      `json:"id"`
	TenantID          string                      `json:"tenant_id"`
	RunID             string                      `json:"run_id"`
	CaseID            string                      `json:"case_id"`
	GapID             string                      `json:"gap_id"`
	SpaceID           string                      `json:"space_id"`
	PromotionID       string                      `json:"promotion_id"`
	PromotionRevision int                         `json:"promotion_revision"`
	ObservedAt        time.Time                   `json:"observed_at"`
	Status            RepairEvalObservationStatus `json:"status"`
	FailureReason     string                      `json:"failure_reason,omitempty"`
}

type RepairEvalObservationStore interface {
	AppendRepairEvalObservation(RepairEvalObservation) (RepairEvalObservation, bool, error)
	ListRepairEvalObservations(tenantID string, limit int) ([]RepairEvalObservation, bool, error)
}

type repairEvalObservationFileData struct {
	Records []RepairEvalObservation `json:"records"`
}

type InMemoryRepairEvalObservationStore struct {
	mu      sync.Mutex
	records []RepairEvalObservation
}

func NewInMemoryRepairEvalObservationStore() *InMemoryRepairEvalObservationStore {
	return &InMemoryRepairEvalObservationStore{}
}

func (s *InMemoryRepairEvalObservationStore) AppendRepairEvalObservation(record RepairEvalObservation) (RepairEvalObservation, bool, error) {
	if err := validateRepairEvalObservation(record); err != nil {
		return RepairEvalObservation{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return appendRepairEvalObservation(&s.records, record)
}

func (s *InMemoryRepairEvalObservationStore) ListRepairEvalObservations(tenantID string, limit int) ([]RepairEvalObservation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return filterRepairEvalObservations(s.records, tenantID, limit)
}

type FileRepairEvalObservationStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileRepairEvalObservationStore(dir string) *FileRepairEvalObservationStore {
	return &FileRepairEvalObservationStore{dir: dir}
}

func (s *FileRepairEvalObservationStore) AppendRepairEvalObservation(record RepairEvalObservation) (RepairEvalObservation, bool, error) {
	if err := validateRepairEvalObservation(record); err != nil {
		return RepairEvalObservation{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return RepairEvalObservation{}, false, err
	}
	result, applied, err := appendRepairEvalObservation(&data.Records, record)
	if err != nil || !applied {
		return result, applied, err
	}
	if err := s.save(data); err != nil {
		return RepairEvalObservation{}, false, err
	}
	return result, true, nil
}

func (s *FileRepairEvalObservationStore) ListRepairEvalObservations(tenantID string, limit int) ([]RepairEvalObservation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return nil, false, err
	}
	return filterRepairEvalObservations(data.Records, tenantID, limit)
}

func (s *FileRepairEvalObservationStore) load() (repairEvalObservationFileData, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return repairEvalObservationFileData{}, nil
	}
	if err != nil {
		return repairEvalObservationFileData{}, err
	}
	var fileData repairEvalObservationFileData
	if err := json.Unmarshal(data, &fileData); err != nil {
		return repairEvalObservationFileData{}, err
	}
	return fileData, nil
}

func (s *FileRepairEvalObservationStore) save(data repairEvalObservationFileData) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "repair_eval_observations.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(encoded, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path())
}

func (s *FileRepairEvalObservationStore) path() string {
	return filepath.Join(s.dir, "repair_eval_observations.json")
}

func appendRepairEvalObservation(records *[]RepairEvalObservation, record RepairEvalObservation) (RepairEvalObservation, bool, error) {
	key := repairEvalObservationKey(record)
	for _, existing := range *records {
		if repairEvalObservationKey(existing) == key {
			return cloneRepairEvalObservation(existing), false, nil
		}
	}
	*records = append(*records, cloneRepairEvalObservation(record))
	return cloneRepairEvalObservation(record), true, nil
}

func filterRepairEvalObservations(records []RepairEvalObservation, tenantID string, limit int) ([]RepairEvalObservation, bool, error) {
	items := make([]RepairEvalObservation, 0)
	for _, record := range records {
		if record.TenantID == tenantID {
			items = append(items, cloneRepairEvalObservation(record))
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ObservedAt.Equal(items[j].ObservedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].ObservedAt.After(items[j].ObservedAt)
	})
	if limit <= 0 {
		limit = 100
	}
	truncated := len(items) > limit
	if truncated {
		items = items[:limit]
	}
	return items, truncated, nil
}

func validateRepairEvalObservation(record RepairEvalObservation) error {
	for field, value := range map[string]string{
		"id": record.ID, "tenant_id": record.TenantID, "run_id": record.RunID,
		"case_id": record.CaseID, "gap_id": record.GapID, "space_id": record.SpaceID,
		"promotion_id": record.PromotionID,
	} {
		if err := validateKnowledgeID(value); err != nil {
			return fmt.Errorf("%s: %w: %v", field, ErrRepairEvalObservationInvalid, err)
		}
	}
	if record.PromotionRevision < 1 || record.ObservedAt.IsZero() {
		return ErrRepairEvalObservationInvalid
	}
	switch record.Status {
	case RepairEvalObservationPassed:
		if strings.TrimSpace(record.FailureReason) != "" {
			return ErrRepairEvalObservationInvalid
		}
	case RepairEvalObservationFailed, RepairEvalObservationUnavailable:
		if !validRepairEvalObservationReason(record.FailureReason) {
			return ErrRepairEvalObservationInvalid
		}
	default:
		return ErrRepairEvalObservationInvalid
	}
	return nil
}

func validRepairEvalObservationReason(reason string) bool {
	switch strings.TrimSpace(reason) {
	case "assertion_failed", "required_check_unavailable", "malformed_result", "executor_unavailable":
		return true
	default:
		return false
	}
}

func repairEvalObservationKey(record RepairEvalObservation) string {
	return strings.Join([]string{record.TenantID, record.RunID, record.CaseID, record.PromotionID, fmt.Sprint(record.PromotionRevision)}, "\x00")
}

func cloneRepairEvalObservation(record RepairEvalObservation) RepairEvalObservation {
	return record
}
