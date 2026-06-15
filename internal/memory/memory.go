package memory

import (
	"context"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// Memory provides short or long-term memory operations.
type Memory interface {
	Remember(context.Context, types.Conversation) error
	Recall(context.Context, string, string, string, int) ([]Record, error)
}

// Record is a recalled long-term memory item.
type Record struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenant_id"`
	UserID         string         `json:"user_id"`
	ConversationID string         `json:"conversation_id"`
	Content        string         `json:"content"`
	Score          float64        `json:"score"`
	Metadata       types.Metadata `json:"metadata,omitempty"`
}
