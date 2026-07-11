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

var ErrRepairEvalPromotionNotFound = errors.New("repair eval promotion not found")

type RepairEvalSupportState string

const (
	RepairEvalSupportPartiallySupported RepairEvalSupportState = "partially_supported"
	RepairEvalSupportGrounded           RepairEvalSupportState = "grounded"
)

type RepairEvalPromotionRevision struct {
	ID                              string                 `json:"id"`
	TenantID                        string                 `json:"tenant_id"`
	CaseID                          string                 `json:"case_id"`
	GapID                           string                 `json:"gap_id"`
	SpaceID                         string                 `json:"space_id"`
	Question                        string                 `json:"question"`
	VerificationAttemptID           string                 `json:"verification_attempt_id"`
	VerificationSnapshotFingerprint string                 `json:"verification_snapshot_fingerprint"`
	MinimumSupportState             RepairEvalSupportState `json:"minimum_support_state"`
	RequiredDocumentIDs             []string               `json:"required_document_ids,omitempty"`
	PolicyFingerprint               string                 `json:"policy_fingerprint"`
	Revision                        int                    `json:"revision"`
	Active                          bool                   `json:"active"`
	PromotedBy                      string                 `json:"promoted_by"`
	PromotedAt                      time.Time              `json:"promoted_at"`
}

type repairEvalPromotionFileData struct {
	Revisions []RepairEvalPromotionRevision `json:"revisions"`
}

type RepairEvalPromotionStore interface {
	PromoteRepairEval(RepairEvalPromotionRevision) (RepairEvalPromotionRevision, bool, error)
	GetActiveRepairEvalPromotion(tenantID, gapID string) (RepairEvalPromotionRevision, error)
	ListRepairEvalPromotions(tenantID, gapID string, activeOnly bool, limit int) ([]RepairEvalPromotionRevision, error)
}

type InMemoryRepairEvalPromotionStore struct {
	mu        sync.Mutex
	revisions []RepairEvalPromotionRevision
}

func NewInMemoryRepairEvalPromotionStore() *InMemoryRepairEvalPromotionStore {
	return &InMemoryRepairEvalPromotionStore{}
}

func (s *InMemoryRepairEvalPromotionStore) PromoteRepairEval(candidate RepairEvalPromotionRevision) (RepairEvalPromotionRevision, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return promoteRepairEval(&s.revisions, candidate)
}

func (s *InMemoryRepairEvalPromotionStore) GetActiveRepairEvalPromotion(tenantID, gapID string) (RepairEvalPromotionRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return getActiveRepairEvalPromotion(s.revisions, tenantID, gapID)
}

func (s *InMemoryRepairEvalPromotionStore) ListRepairEvalPromotions(tenantID, gapID string, activeOnly bool, limit int) ([]RepairEvalPromotionRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return filterRepairEvalPromotions(s.revisions, tenantID, gapID, activeOnly, limit), nil
}

type FileRepairEvalPromotionStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileRepairEvalPromotionStore(dir string) *FileRepairEvalPromotionStore {
	return &FileRepairEvalPromotionStore{dir: dir}
}

func (s *FileRepairEvalPromotionStore) PromoteRepairEval(candidate RepairEvalPromotionRevision) (RepairEvalPromotionRevision, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return RepairEvalPromotionRevision{}, false, err
	}
	record, applied, err := promoteRepairEval(&data.Revisions, candidate)
	if err != nil || !applied {
		return record, applied, err
	}
	if err := s.save(data); err != nil {
		return RepairEvalPromotionRevision{}, false, err
	}
	return record, true, nil
}

func (s *FileRepairEvalPromotionStore) GetActiveRepairEvalPromotion(tenantID, gapID string) (RepairEvalPromotionRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return RepairEvalPromotionRevision{}, err
	}
	return getActiveRepairEvalPromotion(data.Revisions, tenantID, gapID)
}

func (s *FileRepairEvalPromotionStore) ListRepairEvalPromotions(tenantID, gapID string, activeOnly bool, limit int) ([]RepairEvalPromotionRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	return filterRepairEvalPromotions(data.Revisions, tenantID, gapID, activeOnly, limit), nil
}

func (s *FileRepairEvalPromotionStore) load() (repairEvalPromotionFileData, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, "repair_eval_promotions.json"))
	if errors.Is(err, os.ErrNotExist) {
		return repairEvalPromotionFileData{}, nil
	}
	if err != nil {
		return repairEvalPromotionFileData{}, err
	}
	var fileData repairEvalPromotionFileData
	if err := json.Unmarshal(data, &fileData); err != nil {
		return repairEvalPromotionFileData{}, err
	}
	return fileData, nil
}

func (s *FileRepairEvalPromotionStore) save(data repairEvalPromotionFileData) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "repair_eval_promotions.*.tmp")
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
	return os.Rename(tmpName, filepath.Join(s.dir, "repair_eval_promotions.json"))
}

func promoteRepairEval(revisions *[]RepairEvalPromotionRevision, candidate RepairEvalPromotionRevision) (RepairEvalPromotionRevision, bool, error) {
	maxRevision := 0
	activeIndex := -1
	for i, existing := range *revisions {
		if existing.TenantID != candidate.TenantID || existing.GapID != candidate.GapID {
			continue
		}
		if existing.Revision > maxRevision {
			maxRevision = existing.Revision
		}
		if existing.Active {
			activeIndex = i
			if existing.VerificationAttemptID == candidate.VerificationAttemptID && existing.PolicyFingerprint == candidate.PolicyFingerprint {
				return cloneRepairEvalPromotion(existing), false, nil
			}
		}
	}
	if candidate.ID == "" {
		candidate.ID = fmt.Sprintf("promotion-%s-r%d", candidate.GapID, maxRevision+1)
	}
	if err := validateRepairEvalPromotion(candidate); err != nil {
		return RepairEvalPromotionRevision{}, false, err
	}
	if activeIndex >= 0 {
		(*revisions)[activeIndex].Active = false
	}
	candidate.Revision = maxRevision + 1
	candidate.Active = true
	*revisions = append(*revisions, cloneRepairEvalPromotion(candidate))
	return cloneRepairEvalPromotion(candidate), true, nil
}

func validateRepairEvalPromotion(record RepairEvalPromotionRevision) error {
	for field, value := range map[string]string{
		"promotion_id":            record.ID,
		"tenant_id":               record.TenantID,
		"case_id":                 record.CaseID,
		"gap_id":                  record.GapID,
		"space_id":                record.SpaceID,
		"verification_attempt_id": record.VerificationAttemptID,
		"policy_fingerprint":      record.PolicyFingerprint,
		"promoted_by":             record.PromotedBy,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", field)
		}
		if err := validateKnowledgeID(value); err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
	}
	if strings.TrimSpace(record.Question) == "" {
		return errors.New("promotion question is required")
	}
	if record.MinimumSupportState != RepairEvalSupportPartiallySupported && record.MinimumSupportState != RepairEvalSupportGrounded {
		return errors.New("invalid promotion support state")
	}
	if record.PromotedAt.IsZero() {
		return errors.New("promotion timestamp is required")
	}
	seen := make(map[string]struct{}, len(record.RequiredDocumentIDs))
	for _, documentID := range record.RequiredDocumentIDs {
		if err := validateKnowledgeID(documentID); err != nil {
			return fmt.Errorf("required document: %w", err)
		}
		if _, ok := seen[documentID]; ok {
			return errors.New("duplicate required document")
		}
		seen[documentID] = struct{}{}
	}
	return nil
}

func getActiveRepairEvalPromotion(revisions []RepairEvalPromotionRevision, tenantID, gapID string) (RepairEvalPromotionRevision, error) {
	for _, revision := range revisions {
		if revision.Active && revision.TenantID == tenantID && revision.GapID == gapID {
			return cloneRepairEvalPromotion(revision), nil
		}
	}
	return RepairEvalPromotionRevision{}, ErrRepairEvalPromotionNotFound
}

func filterRepairEvalPromotions(revisions []RepairEvalPromotionRevision, tenantID, gapID string, activeOnly bool, limit int) []RepairEvalPromotionRevision {
	items := make([]RepairEvalPromotionRevision, 0)
	for _, revision := range revisions {
		if revision.TenantID != tenantID || (gapID != "" && revision.GapID != gapID) || (activeOnly && !revision.Active) {
			continue
		}
		items = append(items, cloneRepairEvalPromotion(revision))
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Revision > items[j].Revision })
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func cloneRepairEvalPromotion(record RepairEvalPromotionRevision) RepairEvalPromotionRevision {
	record.RequiredDocumentIDs = append([]string(nil), record.RequiredDocumentIDs...)
	return record
}
