package admin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type FileKnowledgeStore struct {
	dir string
}

type knowledgeEnvelope struct {
	Spaces    []KnowledgeSpace    `json:"spaces"`
	Documents []KnowledgeDocument `json:"documents"`
}

func NewFileKnowledgeStore(dir string) *FileKnowledgeStore {
	return &FileKnowledgeStore{dir: dir}
}

func (s *FileKnowledgeStore) SaveKnowledge(document KnowledgeDocument) (KnowledgeDocument, error) {
	if err := validateKnowledgeID(document.ID); err != nil {
		return KnowledgeDocument{}, err
	}
	envelope, err := s.load()
	if err != nil {
		return KnowledgeDocument{}, err
	}
	document.SpaceID = normalizeDocumentSpaceID(document.SpaceID)
	envelope.ensureDefaultSpace(document.TenantID)
	replaced := false
	for index, existing := range envelope.Documents {
		if existing.TenantID == document.TenantID && existing.ID == document.ID {
			envelope.Documents[index] = document
			replaced = true
			break
		}
	}
	if !replaced {
		envelope.Documents = append(envelope.Documents, document)
	}
	if err := s.save(envelope); err != nil {
		return KnowledgeDocument{}, err
	}
	return document, nil
}

func (s *FileKnowledgeStore) ListKnowledge(tenantID string) ([]KnowledgeDocument, error) {
	envelope, err := s.load()
	if err != nil {
		return nil, err
	}
	envelope.ensureDefaultSpace(tenantID)
	out := make([]KnowledgeDocument, 0, len(envelope.Documents))
	for _, document := range envelope.Documents {
		if document.TenantID == tenantID {
			out = append(out, document)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *FileKnowledgeStore) GetKnowledge(tenantID, documentID string) (KnowledgeDocument, error) {
	if err := validateKnowledgeID(documentID); err != nil {
		return KnowledgeDocument{}, err
	}
	envelope, err := s.load()
	if err != nil {
		return KnowledgeDocument{}, err
	}
	envelope.ensureDefaultSpace(tenantID)
	for _, document := range envelope.Documents {
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
	envelope, err := s.load()
	if err != nil {
		return err
	}
	envelope.ensureDefaultSpace(tenantID)
	filtered := make([]KnowledgeDocument, 0, len(envelope.Documents))
	found := false
	for _, document := range envelope.Documents {
		if document.TenantID == tenantID && document.ID == documentID {
			found = true
			continue
		}
		filtered = append(filtered, document)
	}
	if !found {
		return ErrKnowledgeDocumentNotFound
	}
	envelope.Documents = filtered
	return s.save(envelope)
}

func (s *FileKnowledgeStore) SaveKnowledgeSpace(space KnowledgeSpace) (KnowledgeSpace, error) {
	if err := validateKnowledgeID(space.ID); err != nil {
		return KnowledgeSpace{}, err
	}
	envelope, err := s.load()
	if err != nil {
		return KnowledgeSpace{}, err
	}
	envelope.ensureDefaultSpace(space.TenantID)
	replaced := false
	for index, existing := range envelope.Spaces {
		if existing.TenantID == space.TenantID && existing.ID == space.ID {
			envelope.Spaces[index] = space
			replaced = true
			break
		}
	}
	if !replaced {
		envelope.Spaces = append(envelope.Spaces, space)
	}
	sortKnowledgeSpaces(envelope.Spaces)
	if err := s.save(envelope); err != nil {
		return KnowledgeSpace{}, err
	}
	return space, nil
}

func (s *FileKnowledgeStore) ListKnowledgeSpaces(tenantID string) ([]KnowledgeSpace, error) {
	envelope, err := s.load()
	if err != nil {
		return nil, err
	}
	envelope.ensureDefaultSpace(tenantID)
	out := make([]KnowledgeSpace, 0, len(envelope.Spaces))
	for _, space := range envelope.Spaces {
		if space.TenantID == tenantID {
			out = append(out, space)
		}
	}
	sortKnowledgeSpaces(out)
	return out, nil
}

func (s *FileKnowledgeStore) GetKnowledgeSpace(tenantID, spaceID string) (KnowledgeSpace, error) {
	if err := validateKnowledgeID(normalizeDocumentSpaceID(spaceID)); err != nil {
		return KnowledgeSpace{}, err
	}
	envelope, err := s.load()
	if err != nil {
		return KnowledgeSpace{}, err
	}
	envelope.ensureDefaultSpace(tenantID)
	for _, space := range envelope.Spaces {
		if space.TenantID == tenantID && space.ID == normalizeDocumentSpaceID(spaceID) {
			return space, nil
		}
	}
	return KnowledgeSpace{}, ErrKnowledgeSpaceNotFound
}

func (s *FileKnowledgeStore) load() (knowledgeEnvelope, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return knowledgeEnvelope{}, nil
	}
	if err != nil {
		return knowledgeEnvelope{}, err
	}
	var envelope knowledgeEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil && (envelope.Spaces != nil || envelope.Documents != nil) {
		envelope.normalize()
		return envelope, nil
	}
	var documents []KnowledgeDocument
	if err := json.Unmarshal(data, &documents); err != nil {
		return knowledgeEnvelope{}, err
	}
	envelope.Documents = documents
	envelope.normalize()
	return envelope, nil
}

func (s *FileKnowledgeStore) save(envelope knowledgeEnvelope) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	envelope.normalize()
	data, err := json.MarshalIndent(envelope, "", "  ")
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

func (e *knowledgeEnvelope) normalize() {
	for index := range e.Documents {
		e.Documents[index].SpaceID = normalizeDocumentSpaceID(e.Documents[index].SpaceID)
	}
	for _, document := range e.Documents {
		e.ensureDefaultSpace(document.TenantID)
	}
	for _, space := range e.Spaces {
		e.ensureDefaultSpace(space.TenantID)
	}
	sortKnowledgeSpaces(e.Spaces)
	sort.Slice(e.Documents, func(i, j int) bool {
		if e.Documents[i].CreatedAt.Equal(e.Documents[j].CreatedAt) {
			return e.Documents[i].ID < e.Documents[j].ID
		}
		return e.Documents[i].CreatedAt.Before(e.Documents[j].CreatedAt)
	})
}

func (e *knowledgeEnvelope) ensureDefaultSpace(tenantID string) {
	if tenantID == "" {
		return
	}
	for index := range e.Documents {
		if e.Documents[index].TenantID == tenantID && e.Documents[index].SpaceID == "" {
			e.Documents[index].SpaceID = DefaultKnowledgeSpaceID
		}
	}
	for _, space := range e.Spaces {
		if space.TenantID == tenantID && space.ID == DefaultKnowledgeSpaceID {
			return
		}
	}
	now := time.Now().UTC()
	e.Spaces = append(e.Spaces, defaultKnowledgeSpace(tenantID, now))
}

func sortKnowledgeSpaces(spaces []KnowledgeSpace) {
	sort.Slice(spaces, func(i, j int) bool {
		if spaces[i].CreatedAt.Equal(spaces[j].CreatedAt) {
			return spaces[i].ID < spaces[j].ID
		}
		return spaces[i].CreatedAt.Before(spaces[j].CreatedAt)
	})
}
