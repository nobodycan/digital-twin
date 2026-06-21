package admin

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrKnowledgeUploadEmpty = errors.New("knowledge upload is empty")
var ErrKnowledgeCitationMissing = errors.New("knowledge citation missing")

type KnowledgeStatus string

const (
	KnowledgeReady  KnowledgeStatus = "ready"
	KnowledgeFailed KnowledgeStatus = "failed"
)

type KnowledgeUpload struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type KnowledgeDocument struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenant_id"`
	Name      string            `json:"name"`
	Status    KnowledgeStatus   `json:"status"`
	Chunks    []KnowledgeChunk  `json:"chunks"`
	CreatedAt time.Time         `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type KnowledgeChunk struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type KnowledgeCitation struct {
	DocumentID string `json:"document_id"`
	ChunkID    string `json:"chunk_id"`
	Text       string `json:"text"`
}

type KnowledgeStore interface {
	SaveKnowledge(KnowledgeDocument) (KnowledgeDocument, error)
	ListKnowledge(tenantID string) ([]KnowledgeDocument, error)
}

type KnowledgeService struct {
	store KnowledgeStore
	now   func() time.Time
}

func NewKnowledgeService(store KnowledgeStore) KnowledgeService {
	return KnowledgeService{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s KnowledgeService) Upload(tenantID string, upload KnowledgeUpload) (KnowledgeDocument, error) {
	chunks := chunkKnowledge(upload.Content)
	if len(chunks) == 0 {
		return KnowledgeDocument{}, ErrKnowledgeUploadEmpty
	}
	document := KnowledgeDocument{
		ID:        upload.ID,
		TenantID:  tenantID,
		Name:      upload.Name,
		Status:    KnowledgeReady,
		Chunks:    chunks,
		CreatedAt: s.now(),
	}
	return s.store.SaveKnowledge(document)
}

func (s KnowledgeService) CitationTest(tenantID, query string) (KnowledgeCitation, error) {
	documents, err := s.store.ListKnowledge(tenantID)
	if err != nil {
		return KnowledgeCitation{}, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	for _, document := range documents {
		for _, chunk := range document.Chunks {
			if strings.Contains(strings.ToLower(chunk.Text), needle) {
				return KnowledgeCitation{DocumentID: document.ID, ChunkID: chunk.ID, Text: chunk.Text}, nil
			}
		}
	}
	return KnowledgeCitation{}, ErrKnowledgeCitationMissing
}

func chunkKnowledge(content string) []KnowledgeChunk {
	parts := strings.Split(content, "\n\n")
	chunks := make([]KnowledgeChunk, 0, len(parts))
	for _, part := range parts {
		text := strings.TrimSpace(part)
		if text == "" {
			continue
		}
		chunks = append(chunks, KnowledgeChunk{
			ID:   "chunk-" + string(rune('1'+len(chunks))),
			Text: text,
		})
	}
	return chunks
}

type InMemoryKnowledgeStore struct {
	mu        sync.Mutex
	documents map[string][]KnowledgeDocument
}

func NewInMemoryKnowledgeStore() *InMemoryKnowledgeStore {
	return &InMemoryKnowledgeStore{documents: make(map[string][]KnowledgeDocument)}
}

func (s *InMemoryKnowledgeStore) SaveKnowledge(document KnowledgeDocument) (KnowledgeDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents[document.TenantID] = append(s.documents[document.TenantID], document)
	return document, nil
}

func (s *InMemoryKnowledgeStore) ListKnowledge(tenantID string) ([]KnowledgeDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	documents := s.documents[tenantID]
	out := make([]KnowledgeDocument, len(documents))
	copy(out, documents)
	return out, nil
}
