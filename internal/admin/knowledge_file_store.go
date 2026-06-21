package admin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type FileKnowledgeStore struct {
	dir string
}

func NewFileKnowledgeStore(dir string) *FileKnowledgeStore {
	return &FileKnowledgeStore{dir: dir}
}

func (s *FileKnowledgeStore) SaveKnowledge(document KnowledgeDocument) (KnowledgeDocument, error) {
	documents, err := s.load()
	if err != nil {
		return KnowledgeDocument{}, err
	}
	documents = append(documents, document)
	if err := s.save(documents); err != nil {
		return KnowledgeDocument{}, err
	}
	return document, nil
}

func (s *FileKnowledgeStore) ListKnowledge(tenantID string) ([]KnowledgeDocument, error) {
	documents, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]KnowledgeDocument, 0, len(documents))
	for _, document := range documents {
		if document.TenantID == tenantID {
			out = append(out, document)
		}
	}
	return out, nil
}

func (s *FileKnowledgeStore) load() ([]KnowledgeDocument, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var documents []KnowledgeDocument
	if err := json.Unmarshal(data, &documents); err != nil {
		return nil, err
	}
	return documents, nil
}

func (s *FileKnowledgeStore) save(documents []KnowledgeDocument) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(documents, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), data, 0o600)
}

func (s *FileKnowledgeStore) path() string {
	return filepath.Join(s.dir, "knowledge.json")
}
