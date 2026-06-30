package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/nobodycan/digital-twin/internal/admin"
)

func TestPipelineLexicalModeKeepsPhase10Ranking(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})
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

	response := pipeline.Search(context.Background(), documents, SearchRequest{
		Query: "source grounding",
		Limit: 3,
		Mode:  RetrievalModeLexical,
	})
	if response.Mode != RetrievalModeLexical {
		t.Fatalf("Mode = %q, want lexical", response.Mode)
	}
	if len(response.Results) != 3 {
		t.Fatalf("result count = %d, want 3", len(response.Results))
	}
	if response.Results[0].ChunkID != "doc-a:chunk-0001" {
		t.Fatalf("top result = %#v, want doc-a chunk-0001", response.Results[0])
	}
	if response.Results[1].ChunkID != "doc-b:chunk-0001" {
		t.Fatalf("second result = %#v, want doc-b chunk-0001", response.Results[1])
	}
	if response.Results[2].ChunkID != "doc-a:chunk-0002" {
		t.Fatalf("third result = %#v, want doc-a chunk-0002", response.Results[2])
	}
	if len(response.Explanations) != 3 {
		t.Fatalf("explanations len = %d, want 3", len(response.Explanations))
	}
	if response.Explanations[0].LexicalScore <= 0 || response.Explanations[0].FinalScore <= 0 {
		t.Fatalf("top explanation = %#v, want positive scores", response.Explanations[0])
	}
	if len(response.StagesRun) != 1 || response.StagesRun[0] != "lexical" {
		t.Fatalf("StagesRun = %#v, want lexical only", response.StagesRun)
	}
}

func TestPipelineAutoModeFallsBackWhenVectorUnavailable(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-a",
			TenantID: "tenant-1",
			Name:     "alpha.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-a:chunk-0001", DocumentID: "doc-a", Ordinal: 1, Text: "grounded search result"},
			},
		},
	}

	response := pipeline.Search(context.Background(), documents, SearchRequest{
		Query: "grounded search",
		Limit: 1,
		Mode:  RetrievalModeAuto,
	})
	if response.Mode != RetrievalModeAuto {
		t.Fatalf("Mode = %q, want auto", response.Mode)
	}
	if len(response.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(response.Results))
	}
	if len(response.StagesSkipped) != 1 || response.StagesSkipped[0] != "vector_unavailable" {
		t.Fatalf("StagesSkipped = %#v, want vector_unavailable", response.StagesSkipped)
	}
	if response.Explanations[0].IndexStatus != "vector_missing" {
		t.Fatalf("IndexStatus = %q, want vector_missing", response.Explanations[0].IndexStatus)
	}
}

func TestPipelineHybridModeUsesVectorSignalWhenAvailable(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{
		Vector: stubVectorSearcher{
			results: []VectorResult{
				{ChunkID: "doc-b:chunk-0001", Score: 9},
				{ChunkID: "doc-a:chunk-0001", Score: 1},
			},
		},
	})
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-a",
			TenantID: "tenant-1",
			Name:     "alpha.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-a:chunk-0001", DocumentID: "doc-a", Ordinal: 1, Text: "knowledge evidence"},
			},
		},
		{
			ID:       "doc-b",
			TenantID: "tenant-1",
			Name:     "beta.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-b:chunk-0001", DocumentID: "doc-b", Ordinal: 1, Text: "knowledge evidence"},
			},
		},
	}

	response := pipeline.Search(context.Background(), documents, SearchRequest{
		Query: "knowledge evidence",
		Limit: 2,
		Mode:  RetrievalModeHybrid,
	})
	if response.Results[0].ChunkID != "doc-b:chunk-0001" {
		t.Fatalf("top result = %#v, want vector-agreed doc-b first", response.Results[0])
	}
	if response.Explanations[0].VectorScore <= response.Explanations[1].VectorScore {
		t.Fatalf("vector scores = %#v, want top result to have stronger vector score", response.Explanations)
	}
	if len(response.StagesRun) != 2 || response.StagesRun[0] != "lexical" || response.StagesRun[1] != "vector" {
		t.Fatalf("StagesRun = %#v, want lexical then vector", response.StagesRun)
	}
}

func TestPipelineReturnsNoSourceReasonWhenNothingMatches(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-a",
			TenantID: "tenant-1",
			Name:     "alpha.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-a:chunk-0001", DocumentID: "doc-a", Ordinal: 1, Text: "phase 11 retrieval"},
			},
		},
	}

	response := pipeline.Search(context.Background(), documents, SearchRequest{
		Query: "calendar sync",
		Limit: 3,
		Mode:  RetrievalModeLexical,
	})
	if len(response.Results) != 0 {
		t.Fatalf("result count = %d, want 0", len(response.Results))
	}
	if response.NoSourceReason != "no_matching_chunks" {
		t.Fatalf("NoSourceReason = %q, want no_matching_chunks", response.NoSourceReason)
	}
}

func TestPipelineTreatsWeakMatchesAsNoSourceWhenBelowThreshold(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-a",
			TenantID: "tenant-1",
			Name:     "alpha.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-a:chunk-0001", DocumentID: "doc-a", Ordinal: 1, Text: "retrieval only"},
			},
		},
	}

	response := pipeline.Search(context.Background(), documents, SearchRequest{
		Query:    "retrieval",
		Limit:    1,
		Mode:     RetrievalModeLexical,
		MinScore: 4,
	})
	if len(response.Results) != 0 {
		t.Fatalf("result count = %d, want 0 below threshold", len(response.Results))
	}
	if response.NoSourceReason != "below_threshold" {
		t.Fatalf("NoSourceReason = %q, want below_threshold", response.NoSourceReason)
	}
}

func TestPipelineMarksVectorFailureWithoutBreakingLexicalResults(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{
		Vector: stubVectorSearcher{err: errors.New("vector down")},
	})
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-a",
			TenantID: "tenant-1",
			Name:     "alpha.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-a:chunk-0001", DocumentID: "doc-a", Ordinal: 1, Text: "grounded search result"},
			},
		},
	}

	response := pipeline.Search(context.Background(), documents, SearchRequest{
		Query: "grounded search",
		Limit: 1,
		Mode:  RetrievalModeHybrid,
	})
	if len(response.Results) != 1 {
		t.Fatalf("result count = %d, want 1 lexical fallback result", len(response.Results))
	}
	if len(response.StagesSkipped) != 1 || response.StagesSkipped[0] != "vector_failed" {
		t.Fatalf("StagesSkipped = %#v, want vector_failed", response.StagesSkipped)
	}
	if response.Explanations[0].IndexStatus != "vector_failed" {
		t.Fatalf("IndexStatus = %q, want vector_failed", response.Explanations[0].IndexStatus)
	}
}

func TestPipelineVectorModeReturnsUnavailableReasonWhenVectorSearcherMissing(t *testing.T) {
	pipeline := NewPipeline(PipelineConfig{})
	documents := []admin.KnowledgeDocument{
		{
			ID:       "doc-a",
			TenantID: "tenant-1",
			Name:     "alpha.md",
			Status:   admin.KnowledgeReady,
			Chunks: []admin.KnowledgeChunk{
				{ID: "doc-a:chunk-0001", DocumentID: "doc-a", Ordinal: 1, Text: "grounded search result"},
			},
		},
	}

	response := pipeline.Search(context.Background(), documents, SearchRequest{
		Query: "grounded search",
		Limit: 1,
		Mode:  RetrievalModeVector,
	})
	if len(response.Results) != 0 {
		t.Fatalf("result count = %d, want 0", len(response.Results))
	}
	if response.NoSourceReason != "vector_unavailable" {
		t.Fatalf("NoSourceReason = %q, want vector_unavailable", response.NoSourceReason)
	}
	if len(response.StagesSkipped) != 1 || response.StagesSkipped[0] != "vector_unavailable" {
		t.Fatalf("StagesSkipped = %#v, want vector_unavailable", response.StagesSkipped)
	}
}

type stubVectorSearcher struct {
	results []VectorResult
	err     error
}

func (s stubVectorSearcher) Search(context.Context, []admin.KnowledgeDocument, string, int) ([]VectorResult, error) {
	return s.results, s.err
}
