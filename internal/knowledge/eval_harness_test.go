package knowledge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nobodycan/digital-twin/internal/admin"
)

type ragEvalFixture struct {
	ID                 string                    `json:"id"`
	Query              string                    `json:"query"`
	Mode               RetrievalMode             `json:"mode"`
	Limit              int                       `json:"limit"`
	MinScore           float64                   `json:"min_score,omitempty"`
	Documents          []admin.KnowledgeDocument `json:"documents"`
	WantTopChunkID     string                    `json:"want_top_chunk_id,omitempty"`
	WantNoSourceReason string                    `json:"want_no_source_reason,omitempty"`
}

func TestRAGEvalFixtures(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("ReadDir(testdata): %v", err)
	}

	pipeline := NewPipeline(PipelineConfig{})
	runs := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		runs++
		data, err := os.ReadFile(filepath.Join("testdata", entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", entry.Name(), err)
		}
		var fixture ragEvalFixture
		if err := json.Unmarshal(data, &fixture); err != nil {
			t.Fatalf("Unmarshal(%s): %v", entry.Name(), err)
		}

		t.Run(fixture.ID, func(t *testing.T) {
			response := pipeline.Search(context.Background(), fixture.Documents, SearchRequest{
				Query:    fixture.Query,
				Mode:     fixture.Mode,
				Limit:    fixture.Limit,
				MinScore: fixture.MinScore,
			})

			if fixture.WantNoSourceReason != "" {
				if response.NoSourceReason != fixture.WantNoSourceReason {
					t.Fatalf("NoSourceReason = %q, want %q", response.NoSourceReason, fixture.WantNoSourceReason)
				}
				if len(response.Results) != 0 {
					t.Fatalf("results len = %d, want 0", len(response.Results))
				}
				return
			}

			if len(response.Results) == 0 {
				t.Fatalf("expected retrieval result, got none; explanations=%#v", response.Explanations)
			}
			if response.Results[0].ChunkID != fixture.WantTopChunkID {
				t.Fatalf("top chunk = %q, want %q; explanations=%s", response.Results[0].ChunkID, fixture.WantTopChunkID, explanationSummary(response.Explanations))
			}
		})
	}

	if runs == 0 {
		t.Fatal("expected at least one RAG eval fixture")
	}
}

func explanationSummary(explanations []Explanation) string {
	lines := make([]string, 0, len(explanations))
	for _, explanation := range explanations {
		lines = append(lines, explanation.ChunkID+":"+explanation.RankReason)
	}
	return strings.Join(lines, ", ")
}
