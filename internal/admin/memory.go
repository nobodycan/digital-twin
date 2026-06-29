package admin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrMemoryRecordNotFound = errors.New("memory record not found")

type MemoryStatus string

const (
	MemoryActive   MemoryStatus = "active"
	MemoryDisabled MemoryStatus = "disabled"
)

type MemoryRecord struct {
	ID             string       `json:"id"`
	TenantID       string       `json:"tenant_id"`
	UserID         string       `json:"user_id"`
	ConversationID string       `json:"conversation_id,omitempty"`
	Summary        string       `json:"summary"`
	Status         MemoryStatus `json:"status"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type MemoryStore interface {
	Save(MemoryRecord) (MemoryRecord, error)
	List(tenantID string) ([]MemoryRecord, error)
	Get(tenantID, memoryID string) (MemoryRecord, error)
}

type MemoryService struct {
	store MemoryStore
	now   func() time.Time
}

func NewMemoryService(store MemoryStore) MemoryService {
	return MemoryService{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s MemoryService) Save(tenantID string, record MemoryRecord) (MemoryRecord, error) {
	now := s.now()
	record.TenantID = tenantID
	if record.Status == "" {
		record.Status = MemoryActive
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	return s.store.Save(record)
}

func (s MemoryService) Disable(tenantID, memoryID string) (MemoryRecord, error) {
	record, err := s.store.Get(tenantID, memoryID)
	if err != nil {
		return MemoryRecord{}, err
	}
	record.Status = MemoryDisabled
	record.UpdatedAt = s.now()
	return s.store.Save(record)
}

func (s MemoryService) List(tenantID string) ([]MemoryRecord, error) {
	return s.store.List(tenantID)
}

func (s MemoryService) ActiveForRecall(tenantID, userID string) ([]MemoryRecord, error) {
	records, err := s.store.List(tenantID)
	if err != nil {
		return nil, err
	}
	active := make([]MemoryRecord, 0, len(records))
	for _, record := range records {
		if record.UserID == userID && record.Status == MemoryActive {
			active = append(active, record)
		}
	}
	return active, nil
}

type InMemoryMemoryStore struct {
	mu      sync.Mutex
	records map[string]map[string]MemoryRecord
}

func NewInMemoryMemoryStore() *InMemoryMemoryStore {
	return &InMemoryMemoryStore{records: make(map[string]map[string]MemoryRecord)}
}

func (s *InMemoryMemoryStore) Save(record MemoryRecord) (MemoryRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[record.TenantID]; !ok {
		s.records[record.TenantID] = make(map[string]MemoryRecord)
	}
	s.records[record.TenantID][record.ID] = record
	return record, nil
}

func (s *InMemoryMemoryStore) List(tenantID string) ([]MemoryRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := s.records[tenantID]
	out := make([]MemoryRecord, 0, len(records))
	for _, record := range records {
		out = append(out, record)
	}
	return out, nil
}

func (s *InMemoryMemoryStore) Get(tenantID, memoryID string) (MemoryRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[tenantID][memoryID]
	if !ok {
		return MemoryRecord{}, ErrMemoryRecordNotFound
	}
	return record, nil
}

type FileMemoryStore struct {
	dir string
}

func NewFileMemoryStore(dir string) *FileMemoryStore {
	return &FileMemoryStore{dir: dir}
}

func (s *FileMemoryStore) Save(record MemoryRecord) (MemoryRecord, error) {
	records, err := s.load()
	if err != nil {
		return MemoryRecord{}, err
	}
	replaced := false
	for index, existing := range records {
		if existing.TenantID == record.TenantID && existing.ID == record.ID {
			records[index] = record
			replaced = true
			break
		}
	}
	if !replaced {
		records = append(records, record)
	}
	if err := s.save(records); err != nil {
		return MemoryRecord{}, err
	}
	return record, nil
}

func (s *FileMemoryStore) List(tenantID string) ([]MemoryRecord, error) {
	records, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]MemoryRecord, 0, len(records))
	for _, record := range records {
		if record.TenantID == tenantID {
			out = append(out, record)
		}
	}
	return out, nil
}

func (s *FileMemoryStore) Get(tenantID, memoryID string) (MemoryRecord, error) {
	records, err := s.load()
	if err != nil {
		return MemoryRecord{}, err
	}
	for _, record := range records {
		if record.TenantID == tenantID && record.ID == memoryID {
			return record, nil
		}
	}
	return MemoryRecord{}, ErrMemoryRecordNotFound
}

func (s *FileMemoryStore) load() ([]MemoryRecord, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []MemoryRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *FileMemoryStore) save(records []MemoryRecord) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), data, 0o600)
}

func (s *FileMemoryStore) path() string {
	return filepath.Join(s.dir, "memories.json")
}
