package admin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type FileAuditStore struct {
	dir string
}

func NewFileAuditStore(dir string) *FileAuditStore {
	return &FileAuditStore{dir: dir}
}

func (s *FileAuditStore) SaveAudit(record AuditRecord) (AuditRecord, error) {
	records, err := s.load()
	if err != nil {
		return AuditRecord{}, err
	}
	records = append(records, record)
	if err := s.save(records); err != nil {
		return AuditRecord{}, err
	}
	return record, nil
}

func (s *FileAuditStore) ListAudit(tenantID string) ([]AuditRecord, error) {
	records, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]AuditRecord, 0, len(records))
	for _, record := range records {
		if record.TenantID == tenantID {
			out = append(out, record)
		}
	}
	return out, nil
}

func (s *FileAuditStore) load() ([]AuditRecord, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []AuditRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *FileAuditStore) save(records []AuditRecord) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), data, 0o600)
}

func (s *FileAuditStore) path() string {
	return filepath.Join(s.dir, "audit.json")
}
