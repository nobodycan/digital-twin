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
	Tags    []string `json:"tags,omitempty"`
}

type KnowledgeSourceType string

const (
	KnowledgeSourceText     KnowledgeSourceType = "text"
	KnowledgeSourceMarkdown KnowledgeSourceType = "markdown"
)

type KnowledgeDocument struct {
	ID          string              `json:"id"`
	TenantID    string              `json:"tenant_id"`
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
	chunks := chunkKnowledge(upload.ID, upload.Content)
	if len(chunks) == 0 {
		return KnowledgeDocument{}, ErrKnowledgeUploadEmpty
	}
	now := s.now()
	document := KnowledgeDocument{
		ID:          upload.ID,
		TenantID:    tenantID,
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

type InMemoryKnowledgeStore struct {
	mu        sync.Mutex
	documents map[string]map[string]KnowledgeDocument
}

func NewInMemoryKnowledgeStore() *InMemoryKnowledgeStore {
	return &InMemoryKnowledgeStore{documents: make(map[string]map[string]KnowledgeDocument)}
}

func (s *InMemoryKnowledgeStore) SaveKnowledge(document KnowledgeDocument) (KnowledgeDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.documents[document.TenantID]; !ok {
		s.documents[document.TenantID] = make(map[string]KnowledgeDocument)
	}
	s.documents[document.TenantID][document.ID] = document
	return document, nil
}

func (s *InMemoryKnowledgeStore) ListKnowledge(tenantID string) ([]KnowledgeDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	document, ok := s.documents[tenantID][documentID]
	if !ok {
		return KnowledgeDocument{}, ErrKnowledgeDocumentNotFound
	}
	return document, nil
}

func (s *InMemoryKnowledgeStore) DeleteKnowledge(tenantID, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.documents[tenantID][documentID]; !ok {
		return ErrKnowledgeDocumentNotFound
	}
	delete(s.documents[tenantID], documentID)
	return nil
}
