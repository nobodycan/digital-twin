package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrKnowledgeUploadEmpty = errors.New("knowledge upload is empty")
var ErrKnowledgeCitationMissing = errors.New("knowledge citation missing")
var ErrKnowledgeDocumentNotFound = errors.New("knowledge document not found")
var ErrKnowledgeSpaceNotFound = errors.New("knowledge space not found")
var ErrKnowledgeSpaceDisabled = errors.New("knowledge space disabled")
var ErrKnowledgeSpaceArchived = errors.New("knowledge space archived")

var safeKnowledgeIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type KnowledgeStatus string

const (
	KnowledgeReady    KnowledgeStatus = "ready"
	KnowledgeDisabled KnowledgeStatus = "disabled"
	KnowledgeIndexing KnowledgeStatus = "indexing"
	KnowledgeFailed   KnowledgeStatus = "failed"
)

type KnowledgeUpload struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Content string   `json:"content"`
	SpaceID string   `json:"space_id,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type KnowledgeSpaceStatus string

const (
	KnowledgeSpaceActive   KnowledgeSpaceStatus = "active"
	KnowledgeSpaceDisabled KnowledgeSpaceStatus = "disabled"
	KnowledgeSpaceArchived KnowledgeSpaceStatus = "archived"
)

const DefaultKnowledgeSpaceID = "default"

type KnowledgeSpace struct {
	ID                   string               `json:"id"`
	TenantID             string               `json:"tenant_id"`
	Name                 string               `json:"name"`
	Description          string               `json:"description,omitempty"`
	Status               KnowledgeSpaceStatus `json:"status"`
	DefaultRetrievalMode string               `json:"default_retrieval_mode,omitempty"`
	Tags                 []string             `json:"tags,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

type KnowledgeSpaceInput struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Description          string   `json:"description,omitempty"`
	DefaultRetrievalMode string   `json:"default_retrieval_mode,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
}

type KnowledgeSourceType string

const (
	KnowledgeSourceText     KnowledgeSourceType = "text"
	KnowledgeSourceMarkdown KnowledgeSourceType = "markdown"
)

type KnowledgeDocument struct {
	ID          string              `json:"id"`
	TenantID    string              `json:"tenant_id"`
	SpaceID     string              `json:"space_id,omitempty"`
	Name        string              `json:"name"`
	SourceType  KnowledgeSourceType `json:"source_type"`
	Status      KnowledgeStatus     `json:"status"`
	ContentHash string              `json:"content_hash"`
	ChunkCount  int                 `json:"chunk_count"`
	Chunks      []KnowledgeChunk    `json:"chunks"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Tags        []string            `json:"tags,omitempty"`
	Metadata    map[string]string   `json:"metadata,omitempty"`
}

const (
	KnowledgeMetadataLexicalReady   = "lexical_ready"
	KnowledgeMetadataVectorStatus   = "vector_status"
	KnowledgeMetadataEmbeddingModel = "embedding_model"
	KnowledgeMetadataEmbeddingVer   = "embedding_version"
	KnowledgeMetadataIndexedAt      = "indexed_at"
	KnowledgeMetadataLastErrorCode  = "last_error_code"
)

const (
	KnowledgeVectorMissing = "missing"
	KnowledgeVectorReady   = "ready"
	KnowledgeVectorFailed  = "failed"
)

type KnowledgeChunk struct {
	ID         string            `json:"id"`
	DocumentID string            `json:"document_id"`
	Ordinal    int               `json:"ordinal"`
	Text       string            `json:"text"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type KnowledgeCitation struct {
	DocumentID string `json:"document_id"`
	ChunkID    string `json:"chunk_id"`
	Text       string `json:"text"`
}

type KnowledgeStore interface {
	SaveKnowledge(KnowledgeDocument) (KnowledgeDocument, error)
	ListKnowledge(tenantID string) ([]KnowledgeDocument, error)
	GetKnowledge(tenantID, documentID string) (KnowledgeDocument, error)
	DeleteKnowledge(tenantID, documentID string) error
	SaveKnowledgeSpace(KnowledgeSpace) (KnowledgeSpace, error)
	ListKnowledgeSpaces(tenantID string) ([]KnowledgeSpace, error)
	GetKnowledgeSpace(tenantID, spaceID string) (KnowledgeSpace, error)
}

type KnowledgeService struct {
	store KnowledgeStore
	now   func() time.Time
}

func NewKnowledgeService(store KnowledgeStore) KnowledgeService {
	return KnowledgeService{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s KnowledgeService) Upload(tenantID string, upload KnowledgeUpload) (KnowledgeDocument, error) {
	if err := validateKnowledgeID(upload.ID); err != nil {
		return KnowledgeDocument{}, err
	}
	space, err := s.requireWritableSpace(tenantID, upload.SpaceID)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	chunks := chunkKnowledge(upload.ID, upload.Content)
	if len(chunks) == 0 {
		return KnowledgeDocument{}, ErrKnowledgeUploadEmpty
	}
	now := s.now()
	document := KnowledgeDocument{
		ID:          upload.ID,
		TenantID:    tenantID,
		SpaceID:     space.ID,
		Name:        upload.Name,
		SourceType:  sourceTypeFromName(upload.Name),
		Status:      KnowledgeReady,
		ContentHash: hashKnowledgeContent(upload.Content),
		ChunkCount:  len(chunks),
		Chunks:      chunks,
		CreatedAt:   now,
		UpdatedAt:   now,
		Tags:        slices.Clone(upload.Tags),
	}
	applyIndexMetadata(&document, now, KnowledgeVectorMissing, "")
	return s.store.SaveKnowledge(document)
}

func (s KnowledgeService) Get(tenantID, documentID string) (KnowledgeDocument, error) {
	return s.store.GetKnowledge(tenantID, documentID)
}

func (s KnowledgeService) List(tenantID string) ([]KnowledgeDocument, error) {
	return s.store.ListKnowledge(tenantID)
}

func (s KnowledgeService) ListBySpace(tenantID, spaceID string) ([]KnowledgeDocument, error) {
	space, err := s.GetSpace(tenantID, spaceID)
	if err != nil {
		return nil, err
	}
	documents, err := s.store.ListKnowledge(tenantID)
	if err != nil {
		return nil, err
	}
	filtered := make([]KnowledgeDocument, 0, len(documents))
	for _, document := range documents {
		if normalizeDocumentSpaceID(document.SpaceID) == space.ID {
			filtered = append(filtered, document)
		}
	}
	return filtered, nil
}

func (s KnowledgeService) Disable(tenantID, documentID string) (KnowledgeDocument, error) {
	document, err := s.store.GetKnowledge(tenantID, documentID)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	document.Status = KnowledgeDisabled
	document.UpdatedAt = s.now()
	return s.store.SaveKnowledge(document)
}

func (s KnowledgeService) Enable(tenantID, documentID string) (KnowledgeDocument, error) {
	document, err := s.store.GetKnowledge(tenantID, documentID)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	document.Status = KnowledgeReady
	document.UpdatedAt = s.now()
	return s.store.SaveKnowledge(document)
}

func (s KnowledgeService) Reindex(tenantID, documentID, content string) (KnowledgeDocument, error) {
	document, err := s.store.GetKnowledge(tenantID, documentID)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	if _, err := s.requireWritableSpace(tenantID, document.SpaceID); err != nil {
		return KnowledgeDocument{}, err
	}
	chunks := chunkKnowledge(document.ID, content)
	if len(chunks) == 0 {
		return KnowledgeDocument{}, ErrKnowledgeUploadEmpty
	}
	document.Status = KnowledgeReady
	document.ContentHash = hashKnowledgeContent(content)
	document.ChunkCount = len(chunks)
	document.Chunks = chunks
	document.UpdatedAt = s.now()
	applyIndexMetadata(&document, document.UpdatedAt, KnowledgeVectorMissing, "")
	return s.store.SaveKnowledge(document)
}

func (s KnowledgeService) Delete(tenantID, documentID string) error {
	return s.store.DeleteKnowledge(tenantID, documentID)
}

func (s KnowledgeService) ListSpaces(tenantID string) ([]KnowledgeSpace, error) {
	return s.store.ListKnowledgeSpaces(tenantID)
}

func (s KnowledgeService) CreateSpace(tenantID string, input KnowledgeSpaceInput) (KnowledgeSpace, error) {
	if err := validateKnowledgeID(input.ID); err != nil {
		return KnowledgeSpace{}, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return KnowledgeSpace{}, fmt.Errorf("knowledge space name is required")
	}
	if _, err := s.store.GetKnowledgeSpace(tenantID, input.ID); err == nil {
		return KnowledgeSpace{}, fmt.Errorf("knowledge space already exists")
	} else if !errors.Is(err, ErrKnowledgeSpaceNotFound) {
		return KnowledgeSpace{}, err
	}
	now := s.now()
	return s.store.SaveKnowledgeSpace(KnowledgeSpace{
		ID:                   input.ID,
		TenantID:             tenantID,
		Name:                 strings.TrimSpace(input.Name),
		Description:          strings.TrimSpace(input.Description),
		Status:               KnowledgeSpaceActive,
		DefaultRetrievalMode: strings.TrimSpace(input.DefaultRetrievalMode),
		Tags:                 slices.Clone(input.Tags),
		CreatedAt:            now,
		UpdatedAt:            now,
	})
}

func (s KnowledgeService) GetSpace(tenantID, spaceID string) (KnowledgeSpace, error) {
	return s.store.GetKnowledgeSpace(tenantID, normalizeDocumentSpaceID(spaceID))
}

func (s KnowledgeService) UpdateSpace(tenantID string, input KnowledgeSpaceInput) (KnowledgeSpace, error) {
	space, err := s.GetSpace(tenantID, input.ID)
	if err != nil {
		return KnowledgeSpace{}, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return KnowledgeSpace{}, fmt.Errorf("knowledge space name is required")
	}
	space.Name = strings.TrimSpace(input.Name)
	space.Description = strings.TrimSpace(input.Description)
	space.DefaultRetrievalMode = strings.TrimSpace(input.DefaultRetrievalMode)
	space.Tags = slices.Clone(input.Tags)
	space.UpdatedAt = s.now()
	return s.store.SaveKnowledgeSpace(space)
}

func (s KnowledgeService) DisableSpace(tenantID, spaceID string) (KnowledgeSpace, error) {
	return s.updateSpaceStatus(tenantID, spaceID, KnowledgeSpaceDisabled)
}

func (s KnowledgeService) EnableSpace(tenantID, spaceID string) (KnowledgeSpace, error) {
	return s.updateSpaceStatus(tenantID, spaceID, KnowledgeSpaceActive)
}

func (s KnowledgeService) ArchiveSpace(tenantID, spaceID string) (KnowledgeSpace, error) {
	return s.updateSpaceStatus(tenantID, spaceID, KnowledgeSpaceArchived)
}

func (s KnowledgeService) MoveDocument(tenantID, documentID, spaceID string) (KnowledgeDocument, error) {
	space, err := s.requireWritableSpace(tenantID, spaceID)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	document, err := s.store.GetKnowledge(tenantID, documentID)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	document.SpaceID = space.ID
	document.UpdatedAt = s.now()
	return s.store.SaveKnowledge(document)
}

func (s KnowledgeService) CitationTest(tenantID, query string) (KnowledgeCitation, error) {
	documents, err := s.store.ListKnowledge(tenantID)
	if err != nil {
		return KnowledgeCitation{}, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	for _, document := range documents {
		if document.Status != KnowledgeReady {
			continue
		}
		for _, chunk := range document.Chunks {
			if strings.Contains(strings.ToLower(chunk.Text), needle) {
				return KnowledgeCitation{DocumentID: document.ID, ChunkID: chunk.ID, Text: chunk.Text}, nil
			}
		}
	}
	return KnowledgeCitation{}, ErrKnowledgeCitationMissing
}

func chunkKnowledge(documentID, content string) []KnowledgeChunk {
	parts := strings.Split(content, "\n\n")
	chunks := make([]KnowledgeChunk, 0, len(parts))
	for _, part := range parts {
		text := strings.TrimSpace(part)
		if text == "" {
			continue
		}
		ordinal := len(chunks) + 1
		chunks = append(chunks, KnowledgeChunk{
			ID:         fmt.Sprintf("%s:chunk-%04d", documentID, ordinal),
			DocumentID: documentID,
			Ordinal:    ordinal,
			Text:       text,
		})
	}
	return chunks
}

func applyIndexMetadata(document *KnowledgeDocument, indexedAt time.Time, vectorStatus, lastErrorCode string) {
	if document.Metadata == nil {
		document.Metadata = make(map[string]string)
	}
	document.Metadata[KnowledgeMetadataLexicalReady] = "true"
	document.Metadata[KnowledgeMetadataVectorStatus] = vectorStatus
	document.Metadata[KnowledgeMetadataIndexedAt] = indexedAt.Format(time.RFC3339)
	if lastErrorCode == "" {
		delete(document.Metadata, KnowledgeMetadataLastErrorCode)
	} else {
		document.Metadata[KnowledgeMetadataLastErrorCode] = lastErrorCode
	}

	for i := range document.Chunks {
		if document.Chunks[i].Metadata == nil {
			document.Chunks[i].Metadata = make(map[string]string)
		}
		document.Chunks[i].Metadata[KnowledgeMetadataLexicalReady] = "true"
		document.Chunks[i].Metadata[KnowledgeMetadataVectorStatus] = vectorStatus
		document.Chunks[i].Metadata[KnowledgeMetadataIndexedAt] = document.Metadata[KnowledgeMetadataIndexedAt]
		if lastErrorCode == "" {
			delete(document.Chunks[i].Metadata, KnowledgeMetadataLastErrorCode)
		} else {
			document.Chunks[i].Metadata[KnowledgeMetadataLastErrorCode] = lastErrorCode
		}
	}
}

func validateKnowledgeID(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\`) || !safeKnowledgeIDPattern.MatchString(value) {
		return fmt.Errorf("invalid knowledge id")
	}
	return nil
}

func hashKnowledgeContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func sourceTypeFromName(name string) KnowledgeSourceType {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), ".md") {
		return KnowledgeSourceMarkdown
	}
	return KnowledgeSourceText
}

func (s KnowledgeService) updateSpaceStatus(tenantID, spaceID string, status KnowledgeSpaceStatus) (KnowledgeSpace, error) {
	space, err := s.GetSpace(tenantID, spaceID)
	if err != nil {
		return KnowledgeSpace{}, err
	}
	space.Status = status
	space.UpdatedAt = s.now()
	return s.store.SaveKnowledgeSpace(space)
}

func (s KnowledgeService) requireWritableSpace(tenantID, spaceID string) (KnowledgeSpace, error) {
	space, err := s.GetSpace(tenantID, spaceID)
	if err != nil {
		return KnowledgeSpace{}, err
	}
	switch space.Status {
	case KnowledgeSpaceDisabled:
		return KnowledgeSpace{}, ErrKnowledgeSpaceDisabled
	case KnowledgeSpaceArchived:
		return KnowledgeSpace{}, ErrKnowledgeSpaceArchived
	default:
		return space, nil
	}
}

func normalizeDocumentSpaceID(spaceID string) string {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return DefaultKnowledgeSpaceID
	}
	return spaceID
}

func defaultKnowledgeSpace(tenantID string, now time.Time) KnowledgeSpace {
	return KnowledgeSpace{
		ID:                   DefaultKnowledgeSpaceID,
		TenantID:             tenantID,
		Name:                 "Default",
		Status:               KnowledgeSpaceActive,
		DefaultRetrievalMode: "auto",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

type InMemoryKnowledgeStore struct {
	mu        sync.Mutex
	documents map[string]map[string]KnowledgeDocument
	spaces    map[string]map[string]KnowledgeSpace
}

func NewInMemoryKnowledgeStore() *InMemoryKnowledgeStore {
	return &InMemoryKnowledgeStore{
		documents: make(map[string]map[string]KnowledgeDocument),
		spaces:    make(map[string]map[string]KnowledgeSpace),
	}
}

func (s *InMemoryKnowledgeStore) SaveKnowledge(document KnowledgeDocument) (KnowledgeDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	document.SpaceID = normalizeDocumentSpaceID(document.SpaceID)
	s.ensureDefaultSpaceLocked(document.TenantID)
	if _, ok := s.documents[document.TenantID]; !ok {
		s.documents[document.TenantID] = make(map[string]KnowledgeDocument)
	}
	s.documents[document.TenantID][document.ID] = document
	return document, nil
}

func (s *InMemoryKnowledgeStore) ListKnowledge(tenantID string) ([]KnowledgeDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSpaceLocked(tenantID)
	documents := s.documents[tenantID]
	out := make([]KnowledgeDocument, 0, len(documents))
	for _, document := range documents {
		out = append(out, document)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *InMemoryKnowledgeStore) GetKnowledge(tenantID, documentID string) (KnowledgeDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSpaceLocked(tenantID)
	document, ok := s.documents[tenantID][documentID]
	if !ok {
		return KnowledgeDocument{}, ErrKnowledgeDocumentNotFound
	}
	return document, nil
}

func (s *InMemoryKnowledgeStore) DeleteKnowledge(tenantID, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSpaceLocked(tenantID)
	if _, ok := s.documents[tenantID][documentID]; !ok {
		return ErrKnowledgeDocumentNotFound
	}
	delete(s.documents[tenantID], documentID)
	return nil
}

func (s *InMemoryKnowledgeStore) SaveKnowledgeSpace(space KnowledgeSpace) (KnowledgeSpace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.spaces[space.TenantID]; !ok {
		s.spaces[space.TenantID] = make(map[string]KnowledgeSpace)
	}
	s.ensureDefaultSpaceLocked(space.TenantID)
	s.spaces[space.TenantID][space.ID] = space
	return space, nil
}

func (s *InMemoryKnowledgeStore) ListKnowledgeSpaces(tenantID string) ([]KnowledgeSpace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSpaceLocked(tenantID)
	spaces := s.spaces[tenantID]
	out := make([]KnowledgeSpace, 0, len(spaces))
	for _, space := range spaces {
		out = append(out, space)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *InMemoryKnowledgeStore) GetKnowledgeSpace(tenantID, spaceID string) (KnowledgeSpace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultSpaceLocked(tenantID)
	space, ok := s.spaces[tenantID][normalizeDocumentSpaceID(spaceID)]
	if !ok {
		return KnowledgeSpace{}, ErrKnowledgeSpaceNotFound
	}
	return space, nil
}

func (s *InMemoryKnowledgeStore) ensureDefaultSpaceLocked(tenantID string) {
	if tenantID == "" {
		return
	}
	if _, ok := s.spaces[tenantID]; !ok {
		s.spaces[tenantID] = make(map[string]KnowledgeSpace)
	}
	if _, ok := s.spaces[tenantID][DefaultKnowledgeSpaceID]; !ok {
		now := time.Now().UTC()
		s.spaces[tenantID][DefaultKnowledgeSpaceID] = defaultKnowledgeSpace(tenantID, now)
	}
	if _, ok := s.documents[tenantID]; ok {
		for id, document := range s.documents[tenantID] {
			if document.SpaceID == "" {
				document.SpaceID = DefaultKnowledgeSpaceID
				s.documents[tenantID][id] = document
			}
		}
	}
}
