package knowledge

import (
	"context"
	"strings"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type Service struct {
	Store    admin.KnowledgeStore
	Pipeline Pipeline
}

type Grounding struct {
	RetrievalMode  string
	Citations      []Result
	Explanations   []Explanation
	NoSourceReason string
	StagesRun      []string
	StagesSkipped  []string
}

func NewService(store admin.KnowledgeStore) Service {
	return Service{Store: store, Pipeline: NewPipeline(PipelineConfig{})}
}

func (s Service) Ground(ctx context.Context, conversation types.Conversation, query string, limit int) (Grounding, error) {
	if s.Store == nil {
		return Grounding{}, nil
	}
	tenantID := strings.TrimSpace(conversation.TenantID)
	if tenantID == "" {
		return Grounding{}, nil
	}
	documents, err := s.Store.ListKnowledge(tenantID)
	if err != nil {
		return Grounding{}, err
	}
	response := s.Pipeline.Search(ctx, documents, SearchRequest{
		Query: query,
		Limit: limit,
		Mode:  RetrievalModeLexical,
	})
	return Grounding{
		RetrievalMode:  string(response.Mode),
		Citations:      response.Results,
		Explanations:   response.Explanations,
		NoSourceReason: response.NoSourceReason,
		StagesRun:      response.StagesRun,
		StagesSkipped:  response.StagesSkipped,
	}, nil
}

func (s Service) Diagnostics(ctx context.Context, tenantID string, request SearchRequest) (SearchResponse, error) {
	if s.Store == nil {
		return SearchResponse{Mode: request.Mode, NoSourceReason: "knowledge_store_unavailable"}, nil
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return SearchResponse{Mode: request.Mode, NoSourceReason: "tenant_missing"}, nil
	}
	documents, err := s.Store.ListKnowledge(tenantID)
	if err != nil {
		return SearchResponse{}, err
	}
	return s.Pipeline.Search(ctx, documents, request), nil
}
