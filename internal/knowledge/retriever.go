package knowledge

import (
	"sort"
	"strings"
	"unicode"

	"github.com/nobodycan/digital-twin/internal/admin"
)

type Result struct {
	SpaceID      string  `json:"space_id,omitempty"`
	DocumentID   string  `json:"document_id"`
	DocumentName string  `json:"document_name"`
	SourceLabel  string  `json:"source_label,omitempty"`
	SourceType   string  `json:"source_type,omitempty"`
	ReviewStatus string  `json:"review_status,omitempty"`
	ChunkID      string  `json:"chunk_id"`
	ChunkOrdinal int     `json:"chunk_ordinal,omitempty"`
	Rank         int     `json:"rank"`
	Score        float64 `json:"score"`
	Text         string  `json:"text"`
	Snippet      string  `json:"snippet,omitempty"`
	MatchReason  string  `json:"match_reason,omitempty"`
}

type Retriever struct{}

func NewRetriever() Retriever {
	return Retriever{}
}

func (Retriever) Search(documents []admin.KnowledgeDocument, query string, limit int) []Result {
	if limit <= 0 {
		return nil
	}
	normalizedQuery := strings.TrimSpace(strings.ToLower(query))
	if normalizedQuery == "" {
		return nil
	}
	queryTokens := tokenize(normalizedQuery)
	candidates := make([]Result, 0)
	for _, document := range documents {
		if document.Status != admin.KnowledgeReady {
			continue
		}
		for _, chunk := range document.Chunks {
			score := scoreChunk(chunk.Text, normalizedQuery, queryTokens)
			if score <= 0 {
				continue
			}
			candidates = append(candidates, Result{
				SpaceID:      document.SpaceID,
				DocumentID:   document.ID,
				DocumentName: document.Name,
				SourceLabel:  strings.TrimSpace(document.Metadata["source_label"]),
				SourceType:   strings.TrimSpace(document.Metadata["source_type"]),
				ReviewStatus: string(document.ReviewStatus),
				ChunkID:      chunk.ID,
				ChunkOrdinal: chunk.Ordinal,
				Score:        score,
				Text:         chunk.Text,
				Snippet:      snippetForChunk(chunk.Text, normalizedQuery, 160),
				MatchReason:  "lexical",
			})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		if candidates[i].DocumentID != candidates[j].DocumentID {
			return candidates[i].DocumentID < candidates[j].DocumentID
		}
		return chunkOrdinalFromID(candidates[i].ChunkID) < chunkOrdinalFromID(candidates[j].ChunkID)
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	for i := range candidates {
		candidates[i].Rank = i + 1
	}
	return candidates
}

func scoreChunk(text, normalizedQuery string, queryTokens []string) float64 {
	normalizedText := strings.ToLower(strings.TrimSpace(text))
	if normalizedText == "" {
		return 0
	}
	score := 0.0
	for _, token := range queryTokens {
		if token != "" && strings.Contains(normalizedText, token) {
			score += 1
		}
	}
	if containsCJK(normalizedQuery) && strings.Contains(normalizedText, normalizedQuery) {
		score += 2
	}
	if strings.Contains(normalizedText, normalizedQuery) {
		score += 2
	}
	return score
}

func tokenize(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
}

func containsCJK(text string) bool {
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func chunkOrdinalFromID(chunkID string) int {
	lastDash := strings.LastIndexByte(chunkID, '-')
	if lastDash == -1 || lastDash == len(chunkID)-1 {
		return 0
	}
	value := 0
	for _, r := range chunkID[lastDash+1:] {
		if r < '0' || r > '9' {
			return 0
		}
		value = value*10 + int(r-'0')
	}
	return value
}

func snippetForChunk(text, query string, maxLen int) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if normalized == "" {
		return ""
	}
	if maxLen <= 0 || len(normalized) <= maxLen {
		return normalized
	}
	matchIdx := strings.Index(strings.ToLower(normalized), strings.ToLower(strings.TrimSpace(query)))
	if matchIdx < 0 {
		matchIdx = 0
	}
	start := matchIdx - maxLen/4
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(normalized) {
		end = len(normalized)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}
	snippet := normalized[start:end]
	if start > 0 {
		snippet = "..." + strings.TrimSpace(snippet)
	}
	if end < len(normalized) {
		snippet = strings.TrimSpace(snippet) + "..."
	}
	return snippet
}
