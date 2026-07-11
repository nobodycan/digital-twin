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

var ErrRepairRecurrenceNotFound = errors.New("repair recurrence not found")

type RepairRecurrenceStatus string

const (
	RepairRecurrenceSuspected RepairRecurrenceStatus = "suspected"
	RepairRecurrenceDismissed RepairRecurrenceStatus = "dismissed"
	RepairRecurrenceConfirmed RepairRecurrenceStatus = "confirmed"
)

type RepairRecurrenceMatch string

const (
	RepairRecurrenceMatchExplicit RepairRecurrenceMatch = "explicit_gap_id"
	RepairRecurrenceMatchExact    RepairRecurrenceMatch = "exact_fallback"
)

type RepairRecurrence struct {
	ID                          string                 `json:"id"`
	TenantID                    string                 `json:"tenant_id"`
	GapID                       string                 `json:"gap_id"`
	SpaceID                     string                 `json:"space_id"`
	Status                      RepairRecurrenceStatus `json:"status"`
	MatchedBy                   RepairRecurrenceMatch  `json:"matched_by"`
	FirstAuditID                string                 `json:"first_audit_id"`
	LatestAuditID               string                 `json:"latest_audit_id"`
	OccurrenceCount             int                    `json:"occurrence_count"`
	AnswerState                 string                 `json:"answer_state"`
	NoSourceReason              string                 `json:"no_source_reason"`
	VerifiedAttemptID           string                 `json:"verified_attempt_id,omitempty"`
	VerifiedSnapshotFingerprint string                 `json:"verified_snapshot_fingerprint,omitempty"`
	CreatedAt                   time.Time              `json:"created_at"`
	UpdatedAt                   time.Time              `json:"updated_at"`
	DismissedAt                 *time.Time             `json:"dismissed_at,omitempty"`
	DismissReason               string                 `json:"dismiss_reason,omitempty"`
	ConfirmedAt                 *time.Time             `json:"confirmed_at,omitempty"`
	ConfirmedBy                 string                 `json:"confirmed_by,omitempty"`
}

type RepairRecurrenceObservation struct {
	TenantID     string    `json:"tenant_id"`
	GapID        string    `json:"gap_id"`
	AuditID      string    `json:"audit_id"`
	RecurrenceID string    `json:"recurrence_id"`
	RecordedAt   time.Time `json:"recorded_at"`
}

type RepairRecurrenceStore interface {
	ObserveRepairRecurrence(RepairRecurrenceObservation, RepairRecurrence) (RepairRecurrence, bool, error)
	GetRepairRecurrence(tenantID, recurrenceID string) (RepairRecurrence, error)
	SaveRepairRecurrence(RepairRecurrence) (RepairRecurrence, error)
	ListRepairRecurrences(tenantID, gapID, status string, limit int) ([]RepairRecurrence, error)
}

type repairRecurrenceFileData struct {
	Records      []RepairRecurrence            `json:"records"`
	Observations []RepairRecurrenceObservation `json:"observations"`
}

type InMemoryRepairRecurrenceStore struct {
	mu           sync.Mutex
	records      []RepairRecurrence
	observations []RepairRecurrenceObservation
}

func NewInMemoryRepairRecurrenceStore() *InMemoryRepairRecurrenceStore {
	return &InMemoryRepairRecurrenceStore{}
}

func (s *InMemoryRepairRecurrenceStore) ObserveRepairRecurrence(observation RepairRecurrenceObservation, candidate RepairRecurrence) (RepairRecurrence, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return observeRepairRecurrence(&s.records, &s.observations, observation, candidate)
}

func (s *InMemoryRepairRecurrenceStore) GetRepairRecurrence(tenantID, recurrenceID string) (RepairRecurrence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return getRepairRecurrence(s.records, tenantID, recurrenceID)
}

func (s *InMemoryRepairRecurrenceStore) SaveRepairRecurrence(record RepairRecurrence) (RepairRecurrence, error) {
	if err := validateRepairRecurrence(record); err != nil {
		return RepairRecurrence{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.records {
		if existing.TenantID == record.TenantID && existing.ID == record.ID {
			s.records[i] = cloneRepairRecurrence(record)
			return record, nil
		}
	}
	s.records = append(s.records, cloneRepairRecurrence(record))
	return record, nil
}

func (s *InMemoryRepairRecurrenceStore) ListRepairRecurrences(tenantID, gapID, status string, limit int) ([]RepairRecurrence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return filterRepairRecurrences(s.records, tenantID, gapID, status, limit), nil
}

type FileRepairRecurrenceStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileRepairRecurrenceStore(dir string) *FileRepairRecurrenceStore {
	return &FileRepairRecurrenceStore{dir: dir}
}

func (s *FileRepairRecurrenceStore) ObserveRepairRecurrence(observation RepairRecurrenceObservation, candidate RepairRecurrence) (RepairRecurrence, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return RepairRecurrence{}, false, err
	}
	record, applied, err := observeRepairRecurrence(&data.Records, &data.Observations, observation, candidate)
	if err != nil || !applied {
		return record, applied, err
	}
	if err := s.save(data); err != nil {
		return RepairRecurrence{}, false, err
	}
	return record, true, nil
}

func (s *FileRepairRecurrenceStore) GetRepairRecurrence(tenantID, recurrenceID string) (RepairRecurrence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return RepairRecurrence{}, err
	}
	return getRepairRecurrence(data.Records, tenantID, recurrenceID)
}

func (s *FileRepairRecurrenceStore) SaveRepairRecurrence(record RepairRecurrence) (RepairRecurrence, error) {
	if err := validateRepairRecurrence(record); err != nil {
		return RepairRecurrence{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return RepairRecurrence{}, err
	}
	replaced := false
	for i, existing := range data.Records {
		if existing.TenantID == record.TenantID && existing.ID == record.ID {
			data.Records[i] = cloneRepairRecurrence(record)
			replaced = true
			break
		}
	}
	if !replaced {
		data.Records = append(data.Records, cloneRepairRecurrence(record))
	}
	if err := s.save(data); err != nil {
		return RepairRecurrence{}, err
	}
	return record, nil
}

func (s *FileRepairRecurrenceStore) ListRepairRecurrences(tenantID, gapID, status string, limit int) ([]RepairRecurrence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	return filterRepairRecurrences(data.Records, tenantID, gapID, status, limit), nil
}

func (s *FileRepairRecurrenceStore) load() (repairRecurrenceFileData, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, "repair_recurrences.json"))
	if errors.Is(err, os.ErrNotExist) {
		return repairRecurrenceFileData{}, nil
	}
	if err != nil {
		return repairRecurrenceFileData{}, err
	}
	var fileData repairRecurrenceFileData
	if err := json.Unmarshal(data, &fileData); err != nil {
		return repairRecurrenceFileData{}, err
	}
	return fileData, nil
}

func (s *FileRepairRecurrenceStore) save(data repairRecurrenceFileData) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "repair_recurrences.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(s.dir, "repair_recurrences.json"))
}

func observeRepairRecurrence(records *[]RepairRecurrence, observations *[]RepairRecurrenceObservation, observation RepairRecurrenceObservation, candidate RepairRecurrence) (RepairRecurrence, bool, error) {
	if err := validateRepairRecurrenceObservation(observation); err != nil {
		return RepairRecurrence{}, false, err
	}
	if err := validateRepairRecurrence(candidate); err != nil {
		return RepairRecurrence{}, false, err
	}
	for _, existing := range *observations {
		if existing.TenantID == observation.TenantID && existing.GapID == observation.GapID && existing.AuditID == observation.AuditID {
			record, err := getRepairRecurrence(*records, observation.TenantID, existing.RecurrenceID)
			return record, false, err
		}
	}
	for i, existing := range *records {
		if existing.TenantID == candidate.TenantID && existing.GapID == candidate.GapID && existing.Status == RepairRecurrenceSuspected {
			candidate.ID = existing.ID
			candidate.FirstAuditID = existing.FirstAuditID
			candidate.CreatedAt = existing.CreatedAt
			if candidate.OccurrenceCount <= existing.OccurrenceCount {
				candidate.OccurrenceCount = existing.OccurrenceCount + 1
			}
			(*records)[i] = cloneRepairRecurrence(candidate)
			observation.RecurrenceID = existing.ID
			*observations = append(*observations, observation)
			return candidate, true, nil
		}
	}
	if candidate.OccurrenceCount < 1 {
		candidate.OccurrenceCount = 1
	}
	*records = append(*records, cloneRepairRecurrence(candidate))
	observation.RecurrenceID = candidate.ID
	*observations = append(*observations, observation)
	return candidate, true, nil
}

func validateRepairRecurrenceObservation(observation RepairRecurrenceObservation) error {
	if err := validateKnowledgeID(observation.TenantID); err != nil {
		return fmt.Errorf("recurrence observation tenant: %w", err)
	}
	if err := validateKnowledgeID(observation.GapID); err != nil {
		return fmt.Errorf("recurrence observation gap: %w", err)
	}
	if err := validateKnowledgeID(observation.AuditID); err != nil {
		return fmt.Errorf("recurrence observation audit: %w", err)
	}
	if observation.RecordedAt.IsZero() {
		return errors.New("recurrence observation recorded_at is required")
	}
	return nil
}

func validateRepairRecurrence(record RepairRecurrence) error {
	if err := validateKnowledgeID(record.ID); err != nil {
		return fmt.Errorf("recurrence id: %w", err)
	}
	if err := validateKnowledgeID(record.TenantID); err != nil {
		return fmt.Errorf("recurrence tenant: %w", err)
	}
	if err := validateKnowledgeID(record.GapID); err != nil {
		return fmt.Errorf("recurrence gap: %w", err)
	}
	if err := validateKnowledgeID(record.SpaceID); err != nil {
		return fmt.Errorf("recurrence space: %w", err)
	}
	if record.Status != RepairRecurrenceSuspected && record.Status != RepairRecurrenceDismissed && record.Status != RepairRecurrenceConfirmed {
		return errors.New("invalid recurrence status")
	}
	if record.OccurrenceCount < 1 {
		return errors.New("recurrence occurrence_count must be positive")
	}
	return nil
}

func getRepairRecurrence(records []RepairRecurrence, tenantID, recurrenceID string) (RepairRecurrence, error) {
	for _, record := range records {
		if record.TenantID == tenantID && record.ID == recurrenceID {
			return cloneRepairRecurrence(record), nil
		}
	}
	return RepairRecurrence{}, ErrRepairRecurrenceNotFound
}

func filterRepairRecurrences(records []RepairRecurrence, tenantID, gapID, status string, limit int) []RepairRecurrence {
	out := make([]RepairRecurrence, 0)
	for _, record := range records {
		if record.TenantID != tenantID || (gapID != "" && record.GapID != gapID) || (status != "" && string(record.Status) != status) {
			continue
		}
		out = append(out, cloneRepairRecurrence(record))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func cloneRepairRecurrence(record RepairRecurrence) RepairRecurrence {
	if record.DismissedAt != nil {
		value := *record.DismissedAt
		record.DismissedAt = &value
	}
	if record.ConfirmedAt != nil {
		value := *record.ConfirmedAt
		record.ConfirmedAt = &value
	}
	return record
}

func normalizeRecurrenceText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}
