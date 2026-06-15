package memory

import (
	"context"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/store"
	"github.com/nobodycan/digital-twin/pkg/types"
)

// LongTermMemory stores summaries in a vector store and recalls them semantically.
type LongTermMemory struct {
	client  llm.Client
	vectors store.VectorStore
}

// NewLongTermMemory creates a long-term memory implementation.
func NewLongTermMemory(client llm.Client, vectors store.VectorStore) *LongTermMemory {
	return &LongTermMemory{client: client, vectors: vectors}
}

// Remember summarizes, embeds, and stores a conversation memory.
func (m *LongTermMemory) Remember(ctx context.Context, conversation types.Conversation) error {
	summary, err := m.client.Summarize(ctx, conversation)
	if err != nil {
		return core.WrapError(err, "summarize memory")
	}
	vector, err := m.client.Embed(ctx, summary)
	if err != nil {
		return core.WrapError(err, "embed memory")
	}
	return m.vectors.Upsert(ctx, store.VectorDocument{
		ID:      conversation.TenantID + "/" + conversation.UserID + "/" + conversation.ID,
		Vector:  vector,
		Content: summary,
		Metadata: types.Metadata{
			"tenant_id":       conversation.TenantID,
			"user_id":         conversation.UserID,
			"conversation_id": conversation.ID,
		},
	})
}

// Recall returns long-term memory records for a tenant and user.
func (m *LongTermMemory) Recall(ctx context.Context, tenantID, userID, query string, topK int) ([]Record, error) {
	vector, err := m.client.Embed(ctx, query)
	if err != nil {
		return nil, core.WrapError(err, "embed memory query")
	}
	results, err := m.vectors.Search(ctx, vector, topK)
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(results))
	for _, result := range results {
		if result.Document.Metadata["tenant_id"] != tenantID || result.Document.Metadata["user_id"] != userID {
			continue
		}
		records = append(records, Record{
			ID:             result.Document.ID,
			TenantID:       tenantID,
			UserID:         userID,
			ConversationID: stringValue(result.Document.Metadata["conversation_id"]),
			Content:        result.Document.Content,
			Score:          result.Score,
			Metadata:       result.Document.Metadata,
		})
	}
	return records, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
