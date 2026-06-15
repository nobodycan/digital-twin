package store

import (
	"errors"
	"reflect"
	"testing"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestInMemoryVectorStoreSearchesTopKByCosineSimilarity(t *testing.T) {
	store := NewInMemoryVectorStore(2)
	docs := []VectorDocument{
		{ID: "b", Vector: []float64{1, 0}, Metadata: types.Metadata{"name": "b"}},
		{ID: "a", Vector: []float64{1, 0}, Metadata: types.Metadata{"name": "a"}},
		{ID: "c", Vector: []float64{0, 1}, Metadata: types.Metadata{"name": "c"}},
	}
	for _, doc := range docs {
		if err := store.Upsert(t.Context(), doc); err != nil {
			t.Fatalf("upsert %s: %v", doc.ID, err)
		}
	}

	results, err := store.Search(t.Context(), []float64{1, 0}, 2)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	gotIDs := []string{results[0].Document.ID, results[1].Document.ID}
	if !reflect.DeepEqual(gotIDs, []string{"a", "b"}) {
		t.Fatalf("expected deterministic IDs [a b], got %#v", gotIDs)
	}
	if results[0].Score != 1 {
		t.Fatalf("expected score 1, got %v", results[0].Score)
	}
}

func TestInMemoryVectorStoreRejectsDimensionMismatch(t *testing.T) {
	store := NewInMemoryVectorStore(2)
	if err := store.Upsert(t.Context(), VectorDocument{ID: "bad", Vector: []float64{1}}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if _, err := store.Search(t.Context(), []float64{1}, 1); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestInMemoryVectorStoreHandlesEmptyResultsAndInvalidTopK(t *testing.T) {
	store := NewInMemoryVectorStore(2)
	results, err := store.Search(t.Context(), []float64{1, 0}, 3)
	if err != nil {
		t.Fatalf("search empty: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %#v", results)
	}

	if _, err := store.Search(t.Context(), []float64{1, 0}, 0); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("expected invalid top-k, got %v", err)
	}
}
