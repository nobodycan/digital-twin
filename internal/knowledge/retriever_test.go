package knowledge

import (
	"testing"

	"github.com/nobodycan/digital-twin/internal/admin"
)

func TestRetrieverRanksMatchesDeterministically(t *testing.T) {
	retriever := NewRetriever()
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-b",
			TenantID: "tenant-1",
			Name:     "beta.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-b:chunk-0001", DocumentID: "doc-b", Ordinal: 1, Text: "phase 10 retrieval keeps source grounding visible"},
			},
		},
		{
			ID:       "doc-a",
			TenantID: "tenant-1",
			Name:     "alpha.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-a:chunk-0001", DocumentID: "doc-a", Ordinal: 1, Text: "phase 10 retrieval source source grounding"},
				{ID: "doc-a:chunk-0002", DocumentID: "doc-a", Ordinal: 2, Text: "grounding details only"},
			},
		},
	}

	results := retriever.Search(documents, "source grounding", 3)
	if len(results) != 3 {
		t.Fatalf("result count = %d, want 3", len(results))
	}
	if results[0].DocumentID != "doc-a" || results[0].ChunkID != "doc-a:chunk-0001" {
		t.Fatalf("top result = %#v, want doc-a chunk-0001", results[0])
	}
	if results[1].DocumentID != "doc-b" || results[1].ChunkID != "doc-b:chunk-0001" {
		t.Fatalf("second result = %#v, want doc-b chunk-0001", results[1])
	}
	if results[2].ChunkID != "doc-a:chunk-0002" {
		t.Fatalf("third result = %#v, want doc-a chunk-0002", results[2])
	}
}

func TestRetrieverIgnoresDisabledDocuments(t *testing.T) {
	retriever := NewRetriever()
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-disabled",
			TenantID: "tenant-1",
			Name:     "disabled.md",
			Status:   admin.KnowledgeDisabled,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-disabled:chunk-0001", DocumentID: "doc-disabled", Ordinal: 1, Text: "source grounding appears here"},
			},
		},
	}

	results := retriever.Search(documents, "source grounding", 3)
	if len(results) != 0 {
		t.Fatalf("disabled documents should not retrieve, got %#v", results)
	}
}

func TestRetrieverReturnsNoResultsForNoMatches(t *testing.T) {
	retriever := NewRetriever()
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-a",
			TenantID: "tenant-1",
			Name:     "alpha.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-a:chunk-0001", DocumentID: "doc-a", Ordinal: 1, Text: "phase 10 retrieval"},
			},
		},
	}

	results := retriever.Search(documents, "calendar sync", 3)
	if len(results) != 0 {
		t.Fatalf("result count = %d, want 0", len(results))
	}
}

func TestRetrieverSupportsCJKSubstringFallback(t *testing.T) {
	retriever := NewRetriever()
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-zh",
			TenantID: "tenant-1",
			Name:     "policy.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-zh:chunk-0001", DocumentID: "doc-zh", Ordinal: 1, Text: "知识库管理需要清晰引用来源"},
			},
		},
	}

	results := retriever.Search(documents, "引用来源", 1)
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].ChunkID != "doc-zh:chunk-0001" {
		t.Fatalf("top result = %#v, want doc-zh chunk-0001", results[0])
	}
}

func TestRetrieverBreaksScoreTiesByDocumentThenChunkOrdinal(t *testing.T) {
	retriever := NewRetriever()
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-b",
			TenantID: "tenant-1",
			Name:     "beta.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-b:chunk-0002", DocumentID: "doc-b", Ordinal: 2, Text: "citation evidence"},
				{ID: "doc-b:chunk-0001", DocumentID: "doc-b", Ordinal: 1, Text: "citation evidence"},
			},
		},
		{
			ID:       "doc-a",
			TenantID: "tenant-1",
			Name:     "alpha.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-a:chunk-0001", DocumentID: "doc-a", Ordinal: 1, Text: "citation evidence"},
			},
		},
	}

	results := retriever.Search(documents, "citation evidence", 3)
	if len(results) != 3 {
		t.Fatalf("result count = %d, want 3", len(results))
	}
	if results[0].DocumentID != "doc-a" {
		t.Fatalf("first tie-break result = %#v, want doc-a first", results[0])
	}
	if results[1].ChunkID != "doc-b:chunk-0001" {
		t.Fatalf("second tie-break result = %#v, want lower ordinal first", results[1])
	}
	if results[2].ChunkID != "doc-b:chunk-0002" {
		t.Fatalf("third tie-break result = %#v, want higher ordinal last", results[2])
	}
}
