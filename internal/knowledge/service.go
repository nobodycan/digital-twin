package knowledge

import (
	"context"
	"strings"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type Service struct {
	Store     admin.KnowledgeStore
	Retriever Retriever
}

type Grounding struct {
	RetrievalMode string
	Citations     []Result
}

func NewService(store admin.KnowledgeStore) Service {
	return Service{Store: store, Retriever: NewRetriever()}
}

func (s Service) Ground(_ context.Context, conversation types.Conversation, query string, limit int) (Grounding, error) {
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
	results := s.Retriever.Search(documents, query, limit)
	if len(results) == 0 {
		return Grounding{}, nil
	}
	return Grounding{
		RetrievalMode: "lexical",
		Citations:     results,
	}, nil
}
