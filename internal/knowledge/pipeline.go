package knowledge

import (
	"context"
	"slices"
	"sort"
	"strings"

	"github.com/nobodycan/digital-twin/internal/admin"
)

type RetrievalMode string

const (
	RetrievalModeLexical RetrievalMode = "lexical"
	RetrievalModeVector  RetrievalMode = "vector"
	RetrievalModeHybrid  RetrievalMode = "hybrid"
	RetrievalModeAuto    RetrievalMode = "auto"
)

type SearchRequest struct {
	Query    string
	Limit    int
	Mode     RetrievalMode
	SpaceID  string
	MinScore float64
}

type Explanation struct {
	DocumentID   string   `json:"document_id"`
	DocumentName string   `json:"document_name"`
	ChunkID      string   `json:"chunk_id"`
	Rank         int      `json:"rank"`
	LexicalScore float64  `json:"lexical_score"`
	VectorScore  float64  `json:"vector_score"`
	FinalScore   float64  `json:"final_score"`
	MatchedTerms []string `json:"matched_terms,omitempty"`
	RankReason   string   `json:"rank_reason,omitempty"`
	IndexStatus  string   `json:"index_status,omitempty"`
}

type SearchResponse struct {
	Mode           RetrievalMode `json:"mode"`
	Results        []Result      `json:"results"`
	Explanations   []Explanation `json:"explanations"`
	NoSourceReason string        `json:"no_source_reason,omitempty"`
	StagesRun      []string      `json:"stages_run,omitempty"`
	StagesSkipped  []string      `json:"stages_skipped,omitempty"`
}

type VectorResult struct {
	ChunkID string  `json:"chunk_id"`
	Score   float64 `json:"score"`
}

type VectorSearcher interface {
	Search(context.Context, []admin.KnowledgeDocument, string, int) ([]VectorResult, error)
}

type PipelineConfig struct {
	Lexical Retriever
	Vector  VectorSearcher
}

type Pipeline struct {
	lexical Retriever
	vector  VectorSearcher
}

func NewPipeline(config PipelineConfig) Pipeline {
	lexical := config.Lexical
	if lexical == (Retriever{}) {
		lexical = NewRetriever()
	}
	return Pipeline{
		lexical: lexical,
		vector:  config.Vector,
	}
}

func (p Pipeline) Search(ctx context.Context, documents []admin.KnowledgeDocument, request SearchRequest) SearchResponse {
	mode := request.Mode
	if mode == "" {
		mode = RetrievalModeAuto
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 3
	}
	query := strings.TrimSpace(request.Query)
	response := SearchResponse{Mode: mode}
	if query == "" {
		response.NoSourceReason = "query_empty"
		return response
	}

	readyDocuments := readyKnowledgeDocuments(documents)
	if request.SpaceID != "" {
		readyDocuments = filterKnowledgeDocumentsBySpace(readyDocuments, request.SpaceID)
	}
	if len(readyDocuments) == 0 {
		response.NoSourceReason = "no_ready_documents"
		return response
	}

	candidates := make(map[string]*Explanation)
	documentIndex := indexDocuments(readyDocuments)

	shouldRunLexical := mode == RetrievalModeLexical || mode == RetrievalModeHybrid || mode == RetrievalModeAuto
	if shouldRunLexical {
		response.StagesRun = append(response.StagesRun, "lexical")
		lexicalResults := p.lexical.Search(readyDocuments, query, limit)
		queryTokens := tokenize(strings.ToLower(query))
		for _, result := range lexicalResults {
			candidate := ensureCandidate(candidates, documentIndex, result.ChunkID)
			candidate.DocumentID = result.DocumentID
			candidate.DocumentName = result.DocumentName
			candidate.ChunkID = result.ChunkID
			candidate.LexicalScore = result.Score
			candidate.MatchedTerms = matchedTerms(result.Text, queryTokens)
		}
	}

	shouldRunVector := mode == RetrievalModeVector || mode == RetrievalModeHybrid || mode == RetrievalModeAuto
	if shouldRunVector {
		switch p.vector {
		case nil:
			response.StagesSkipped = append(response.StagesSkipped, "vector_unavailable")
			markIndexStatus(candidates, "vector_missing")
		default:
			vectorResults, err := p.vector.Search(ctx, readyDocuments, query, limit)
			if err != nil {
				response.StagesSkipped = append(response.StagesSkipped, "vector_failed")
				markIndexStatus(candidates, "vector_failed")
			} else {
				response.StagesRun = append(response.StagesRun, "vector")
				for _, result := range vectorResults {
					candidate := ensureCandidate(candidates, documentIndex, result.ChunkID)
					candidate.VectorScore = result.Score
					if candidate.IndexStatus == "" {
						candidate.IndexStatus = "vector_ready"
					}
				}
			}
		}
	}

	explanations := make([]Explanation, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.FinalScore = candidate.LexicalScore + candidate.VectorScore
		if candidate.IndexStatus == "" && shouldRunVector {
			candidate.IndexStatus = "vector_ready"
		}
		candidate.RankReason = rankReason(*candidate)
		if candidate.FinalScore <= 0 {
			continue
		}
		explanations = append(explanations, *candidate)
	}

	sortExplanations(explanations)
	if request.MinScore > 0 {
		filtered := explanations[:0]
		for _, explanation := range explanations {
			if explanation.FinalScore >= request.MinScore {
				filtered = append(filtered, explanation)
			}
		}
		explanations = filtered
		if len(explanations) == 0 {
			response.NoSourceReason = "below_threshold"
			return response
		}
	}
	if len(explanations) == 0 {
		if mode == RetrievalModeVector && len(response.StagesSkipped) > 0 && response.StagesSkipped[0] == "vector_unavailable" {
			response.NoSourceReason = "vector_unavailable"
			return response
		}
		if mode == RetrievalModeVector && len(response.StagesSkipped) > 0 && response.StagesSkipped[0] == "vector_failed" {
			response.NoSourceReason = "vector_failed"
			return response
		}
		response.NoSourceReason = "no_matching_chunks"
		return response
	}
	if len(explanations) > limit {
		explanations = explanations[:limit]
	}

	results := make([]Result, 0, len(explanations))
	for index := range explanations {
		explanations[index].Rank = index + 1
		results = append(results, Result{
			DocumentID:   explanations[index].DocumentID,
			DocumentName: explanations[index].DocumentName,
			ChunkID:      explanations[index].ChunkID,
			Rank:         explanations[index].Rank,
			Score:        explanations[index].FinalScore,
			Text:         chunkText(documentIndex[explanations[index].ChunkID]),
		})
	}
	response.Results = results
	response.Explanations = explanations
	return response
}

func readyKnowledgeDocuments(documents []admin.KnowledgeDocument) []admin.KnowledgeDocument {
	ready := make([]admin.KnowledgeDocument, 0, len(documents))
	for _, document := range documents {
		if document.Status == admin.KnowledgeReady {
			ready = append(ready, document)
		}
	}
	return ready
}

func filterKnowledgeDocumentsBySpace(documents []admin.KnowledgeDocument, spaceID string) []admin.KnowledgeDocument {
	filtered := make([]admin.KnowledgeDocument, 0, len(documents))
	for _, document := range documents {
		if document.SpaceID == spaceID {
			filtered = append(filtered, document)
		}
	}
	return filtered
}

func indexDocuments(documents []admin.KnowledgeDocument) map[string]admin.KnowledgeChunk {
	index := make(map[string]admin.KnowledgeChunk)
	for _, document := range documents {
		for _, chunk := range document.Chunks {
			chunkCopy := chunk
			if chunkCopy.Metadata == nil {
				chunkCopy.Metadata = map[string]string{}
			}
			chunkCopy.Metadata["document_id"] = document.ID
			chunkCopy.Metadata["document_name"] = document.Name
			index[chunk.ID] = chunkCopy
		}
	}
	return index
}

func ensureCandidate(candidates map[string]*Explanation, index map[string]admin.KnowledgeChunk, chunkID string) *Explanation {
	if candidate, ok := candidates[chunkID]; ok {
		return candidate
	}
	chunk := index[chunkID]
	candidate := &Explanation{
		DocumentID:   chunk.Metadata["document_id"],
		DocumentName: chunk.Metadata["document_name"],
		ChunkID:      chunkID,
		IndexStatus:  explanationIndexStatus(chunk.Metadata[admin.KnowledgeMetadataVectorStatus]),
	}
	candidates[chunkID] = candidate
	return candidate
}

func explanationIndexStatus(vectorStatus string) string {
	switch vectorStatus {
	case admin.KnowledgeVectorReady:
		return "vector_ready"
	case admin.KnowledgeVectorFailed:
		return "vector_failed"
	case admin.KnowledgeVectorMissing:
		return "vector_missing"
	default:
		return ""
	}
}

func markIndexStatus(candidates map[string]*Explanation, status string) {
	for _, candidate := range candidates {
		candidate.IndexStatus = status
	}
}

func matchedTerms(text string, queryTokens []string) []string {
	normalized := strings.ToLower(text)
	matches := make([]string, 0, len(queryTokens))
	for _, token := range queryTokens {
		if token == "" || !strings.Contains(normalized, token) {
			continue
		}
		if slices.Contains(matches, token) {
			continue
		}
		matches = append(matches, token)
	}
	return matches
}

func rankReason(explanation Explanation) string {
	switch {
	case explanation.LexicalScore > 0 && explanation.VectorScore > 0:
		return "lexical+vector"
	case explanation.VectorScore > 0:
		return "vector"
	default:
		return "lexical"
	}
}

func chunkText(chunk admin.KnowledgeChunk) string {
	return chunk.Text
}

func sortExplanations(explanations []Explanation) {
	sort.Slice(explanations, func(i, j int) bool {
		if explanations[i].FinalScore != explanations[j].FinalScore {
			return explanations[i].FinalScore > explanations[j].FinalScore
		}
		if explanations[i].DocumentID != explanations[j].DocumentID {
			return explanations[i].DocumentID < explanations[j].DocumentID
		}
		return chunkOrdinalFromID(explanations[i].ChunkID) < chunkOrdinalFromID(explanations[j].ChunkID)
	})
}
