package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/store"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestLongTermMemoryStoresSummaryAndRecallsByVector(t *testing.T) {
	client := &memoryLLM{summary: "Ada likes Go", vector: []float64{1, 0}}
	vectorStore := store.NewInMemoryVectorStore(2)
	memory := NewLongTermMemory(client, vectorStore)
	conversation := types.Conversation{ID: "conv-1", TenantID: "tenant-1", UserID: "user-1"}

	if err := memory.Remember(t.Context(), conversation); err != nil {
		t.Fatalf("remember: %v", err)
	}

	records, err := memory.Recall(t.Context(), "tenant-1", "user-1", "go", 1)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(records) != 1 || records[0].Content != "Ada likes Go" {
		t.Fatalf("unexpected records %#v", records)
	}
}

func TestLongTermMemoryDoesNotWritePartialMemoryWhenSummarizeFails(t *testing.T) {
	client := &memoryLLM{err: errors.New("summarize failed")}
	vectorStore := store.NewInMemoryVectorStore(2)
	memory := NewLongTermMemory(client, vectorStore)

	err := memory.Remember(t.Context(), types.Conversation{ID: "conv-1", TenantID: "tenant-1", UserID: "user-1"})
	if err == nil {
		t.Fatalf("expected error")
	}

	results, searchErr := vectorStore.Search(t.Context(), []float64{1, 0}, 1)
	if searchErr != nil {
		t.Fatalf("search: %v", searchErr)
	}
	if len(results) != 0 {
		t.Fatalf("expected no partial writes, got %#v", results)
	}
}

type memoryLLM struct {
	summary string
	vector  []float64
	err     error
}

func (m *memoryLLM) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}

func (m *memoryLLM) Stream(context.Context, llm.ChatRequest, func(llm.ChatChunk) error) error {
	return nil
}

func (m *memoryLLM) Embed(context.Context, string) ([]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.vector, nil
}

func (m *memoryLLM) Summarize(context.Context, types.Conversation) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.summary, nil
}
