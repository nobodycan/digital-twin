package admin

import (
	"fmt"
	"sync"
	"time"
)

type AuditStatus string

const (
	AuditStatusCompleted AuditStatus = "completed"
	AuditStatusCancelled AuditStatus = "cancelled"
	AuditStatusFailed    AuditStatus = "failed"
)

type AuditRecord struct {
	ID             string      `json:"id"`
	TenantID       string      `json:"tenant_id"`
	ConversationID string      `json:"conversation_id"`
	UserID         string      `json:"user_id"`
	Status         AuditStatus `json:"status"`
	AgentName      string      `json:"agent_name"`
	LatencyMS      int64       `json:"latency_ms"`
	EventSummary   []string    `json:"event_summary"`
	CreatedAt      time.Time   `json:"created_at"`
}

type AuditStore interface {
	SaveAudit(AuditRecord) (AuditRecord, error)
	ListAudit(tenantID string) ([]AuditRecord, error)
}

type AuditService struct {
	store AuditStore
	now   func() time.Time
}

func NewAuditService(store AuditStore) AuditService {
	return AuditService{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s AuditService) Record(tenantID string, record AuditRecord) (AuditRecord, error) {
	record.TenantID = tenantID
	if record.ID == "" {
		record.ID = fmt.Sprintf("audit-%s", record.ConversationID)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = s.now()
	}
	return s.store.SaveAudit(record)
}

func (s AuditService) Recent(tenantID string) ([]AuditRecord, error) {
	return s.store.ListAudit(tenantID)
}

type InMemoryAuditStore struct {
	mu      sync.Mutex
	records map[string][]AuditRecord
}

func NewInMemoryAuditStore() *InMemoryAuditStore {
	return &InMemoryAuditStore{records: make(map[string][]AuditRecord)}
}

func (s *InMemoryAuditStore) SaveAudit(record AuditRecord) (AuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.TenantID] = append(s.records[record.TenantID], record)
	return record, nil
}

func (s *InMemoryAuditStore) ListAudit(tenantID string) ([]AuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := s.records[tenantID]
	out := make([]AuditRecord, len(records))
	copy(out, records)
	return out, nil
}
