package store

import (
	"context"
	"math"
	"sort"
	"sync"

	"github.com/nobodycan/digital-twin/internal/core"
)

// InMemoryVectorStore stores vectors in memory for deterministic local tests.
type InMemoryVectorStore struct {
	mu        sync.RWMutex
	dimension int
	docs      map[string]VectorDocument
}

// NewInMemoryVectorStore creates an empty vector store with a fixed dimension.
func NewInMemoryVectorStore(dimension int) *InMemoryVectorStore {
	return &InMemoryVectorStore{dimension: dimension, docs: make(map[string]VectorDocument)}
}

// Upsert inserts or replaces a vector document.
func (s *InMemoryVectorStore) Upsert(ctx context.Context, doc VectorDocument) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if doc.ID == "" || len(doc.Vector) != s.dimension {
		return core.NewNamedError(core.ErrInvalidInput, "vector", doc.ID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	copyDoc := doc
	copyDoc.Vector = append([]float64(nil), doc.Vector...)
	s.docs[doc.ID] = copyDoc
	return nil
}

// Search returns top-k vector documents by cosine similarity.
func (s *InMemoryVectorStore) Search(ctx context.Context, vector []float64, topK int) ([]VectorSearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if topK <= 0 || len(vector) != s.dimension {
		return nil, core.NewNamedError(core.ErrInvalidInput, "vector", "query")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]VectorSearchResult, 0, len(s.docs))
	for _, doc := range s.docs {
		results = append(results, VectorSearchResult{Document: doc, Score: cosine(vector, doc.Vector)})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Document.ID < results[j].Document.ID
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func cosine(a, b []float64) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

var _ VectorStore = (*InMemoryVectorStore)(nil)
