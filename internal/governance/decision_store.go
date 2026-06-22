package governance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type DecisionType string

const (
	DecisionEval     DecisionType = "eval"
	DecisionPolicy   DecisionType = "policy"
	DecisionRelease  DecisionType = "release"
	DecisionRollback DecisionType = "rollback"
	DecisionFeedback DecisionType = "feedback"
)

type DecisionRecord struct {
	ID        string         `json:"id"`
	TenantID  string         `json:"tenant_id"`
	Type      DecisionType   `json:"type"`
	ActorID   string         `json:"actor_id"`
	CreatedAt time.Time      `json:"created_at"`
	Evidence  types.Metadata `json:"evidence,omitempty"`
}

type DecisionStore interface {
	SaveDecision(DecisionRecord) error
	GetDecision(tenantID, decisionID string) (DecisionRecord, error)
	ListDecisions(tenantID string) ([]DecisionRecord, error)
}

type InMemoryDecisionStore struct {
	mu      sync.Mutex
	records map[string]DecisionRecord
}

func NewInMemoryDecisionStore() *InMemoryDecisionStore {
	return &InMemoryDecisionStore{records: make(map[string]DecisionRecord)}
}

func (s *InMemoryDecisionStore) SaveDecision(record DecisionRecord) error {
	if err := validateDecisionRecord(record); err != nil {
		return err
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[decisionKey(record.TenantID, record.ID)] = record
	return nil
}

func (s *InMemoryDecisionStore) GetDecision(tenantID, decisionID string) (DecisionRecord, error) {
	if err := validateSafeID("tenant_id", tenantID); err != nil {
		return DecisionRecord{}, err
	}
	if err := validateSafeID("decision_id", decisionID); err != nil {
		return DecisionRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[decisionKey(tenantID, decisionID)]
	if !ok {
		return DecisionRecord{}, core.WrapError(core.ErrStoreFailure, "decision missing")
	}
	return record, nil
}

func (s *InMemoryDecisionStore) ListDecisions(tenantID string) ([]DecisionRecord, error) {
	if err := validateSafeID("tenant_id", tenantID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]DecisionRecord, 0)
	for _, record := range s.records {
		if record.TenantID == tenantID {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID < records[j].ID
	})
	return records, nil
}

type FileDecisionStore struct {
	root string
	mu   sync.Mutex
}

func NewFileDecisionStore(root string) *FileDecisionStore {
	return &FileDecisionStore{root: root}
}

func (s *FileDecisionStore) SaveDecision(record DecisionRecord) error {
	if err := validateDecisionRecord(record); err != nil {
		return err
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSONAtomic(s.path(record.TenantID, record.ID), record)
}

func (s *FileDecisionStore) GetDecision(tenantID, decisionID string) (DecisionRecord, error) {
	if err := validateSafeID("tenant_id", tenantID); err != nil {
		return DecisionRecord{}, err
	}
	if err := validateSafeID("decision_id", decisionID); err != nil {
		return DecisionRecord{}, err
	}
	data, err := os.ReadFile(s.path(tenantID, decisionID))
	if err != nil {
		return DecisionRecord{}, core.WrapError(core.ErrStoreFailure, "decision missing")
	}
	var record DecisionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return DecisionRecord{}, core.WrapError(core.ErrStoreFailure, "decode decision")
	}
	return record, nil
}

func (s *FileDecisionStore) ListDecisions(tenantID string) ([]DecisionRecord, error) {
	if err := validateSafeID("tenant_id", tenantID); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.root, "tenants", tenantID, "governance", "decisions")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []DecisionRecord{}, nil
	}
	if err != nil {
		return nil, core.WrapError(core.ErrStoreFailure, "list decisions")
	}
	records := make([]DecisionRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		record, err := s.GetDecision(tenantID, entry.Name()[:len(entry.Name())-len(".json")])
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID < records[j].ID
	})
	return records, nil
}

func (s *FileDecisionStore) path(tenantID, decisionID string) string {
	return filepath.Join(s.root, "tenants", tenantID, "governance", "decisions", decisionID+".json")
}

func validateDecisionRecord(record DecisionRecord) error {
	if err := validateSafeID("decision_id", record.ID); err != nil {
		return err
	}
	if err := validateSafeID("tenant_id", record.TenantID); err != nil {
		return err
	}
	if !validDecisionType(record.Type) {
		return core.NewNamedError(core.ErrInvalidInput, "decision_type", string(record.Type))
	}
	if record.ActorID == "" {
		return core.NewNamedError(core.ErrInvalidInput, "actor_id", record.ActorID)
	}
	return nil
}

func validDecisionType(value DecisionType) bool {
	switch value {
	case DecisionEval, DecisionPolicy, DecisionRelease, DecisionRollback, DecisionFeedback:
		return true
	default:
		return false
	}
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return core.WrapError(core.ErrStoreFailure, "create governance directory")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return core.WrapError(core.ErrStoreFailure, "encode governance record")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return core.WrapError(core.ErrStoreFailure, "write governance record")
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return core.WrapError(core.ErrStoreFailure, "replace governance record")
	}
	return nil
}

func decisionKey(tenantID, decisionID string) string {
	return tenantID + "/" + decisionID
}

var _ DecisionStore = (*InMemoryDecisionStore)(nil)
var _ DecisionStore = (*FileDecisionStore)(nil)
