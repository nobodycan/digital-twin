package store

import (
	"context"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// Store persists conversations and messages.
type Store interface {
	SaveConversation(context.Context, types.Conversation) error
	GetConversation(context.Context, string, string, string) (types.Conversation, error)
	AppendMessage(context.Context, string, string, string, types.Message) error
	ListMessages(context.Context, string, string, string) ([]types.Message, error)
}

// VectorStore persists and searches vector documents.
type VectorStore interface {
	Upsert(context.Context, VectorDocument) error
	Search(context.Context, []float64, int) ([]VectorSearchResult, error)
}

// VectorDocument stores a vector and associated metadata.
type VectorDocument struct {
	ID       string         `json:"id"`
	Vector   []float64      `json:"vector"`
	Content  string         `json:"content,omitempty"`
	Metadata types.Metadata `json:"metadata,omitempty"`
}

// VectorSearchResult contains a matched document and score.
type VectorSearchResult struct {
	Document VectorDocument `json:"document"`
	Score    float64        `json:"score"`
}
