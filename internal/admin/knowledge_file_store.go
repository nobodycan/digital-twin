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
	if err := validateKnowledgeID(document.ID); err != nil {
		return KnowledgeDocument{}, err
	}
	documents, err := s.load()
	if err != nil {
		return KnowledgeDocument{}, err
	}
	replaced := false
	for index, existing := range documents {
		if existing.TenantID == document.TenantID && existing.ID == document.ID {
			documents[index] = document
			replaced = true
			break
		}
	}
	if !replaced {
		documents = append(documents, document)
	}
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

func (s *FileKnowledgeStore) GetKnowledge(tenantID, documentID string) (KnowledgeDocument, error) {
	if err := validateKnowledgeID(documentID); err != nil {
		return KnowledgeDocument{}, err
	}
	documents, err := s.load()
	if err != nil {
		return KnowledgeDocument{}, err
	}
	for _, document := range documents {
		if document.TenantID == tenantID && document.ID == documentID {
			return document, nil
		}
	}
	return KnowledgeDocument{}, ErrKnowledgeDocumentNotFound
}

func (s *FileKnowledgeStore) DeleteKnowledge(tenantID, documentID string) error {
	if err := validateKnowledgeID(documentID); err != nil {
		return err
	}
	documents, err := s.load()
	if err != nil {
		return err
	}
	filtered := make([]KnowledgeDocument, 0, len(documents))
	found := false
	for _, document := range documents {
		if document.TenantID == tenantID && document.ID == documentID {
			found = true
			continue
		}
		filtered = append(filtered, document)
	}
	if !found {
		return ErrKnowledgeDocumentNotFound
	}
	return s.save(filtered)
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
	tmpFile, err := os.CreateTemp(s.dir, "knowledge.json.*.tmp")
	if err != nil {
		return err
	}
	tmp := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, s.path()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *FileKnowledgeStore) path() string {
	return filepath.Join(s.dir, "knowledge.json")
}
