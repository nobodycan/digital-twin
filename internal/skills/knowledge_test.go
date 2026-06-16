package skills

import (
	"context"
	"errors"
	"testing"

	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/store"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestKnowledgeSkillsRunWithValidParams(t *testing.T) {
	client := &fakeKnowledgeLLM{embedding: []float64{1, 0}}
	vector := &fakeVectorStore{results: []store.VectorSearchResult{{Document: store.VectorDocument{ID: "doc-1", Content: "answer"}, Score: 0.9}}}

	embedResult, err := NewEmbedSkill(client).Run(context.Background(), map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("embed Run() error = %v", err)
	}
	if embedResult.SkillName != "embed" {
		t.Fatalf("embed skill = %q", embedResult.SkillName)
	}

	searchResult, err := NewVectorSearchSkill(vector).Run(context.Background(), map[string]any{"vector": []any{1.0, 0.0}, "limit": 1})
	if err != nil {
		t.Fatalf("vector_search Run() error = %v", err)
	}
	if searchResult.SkillName != "vector_search" {
		t.Fatalf("vector_search skill = %q", searchResult.SkillName)
	}

	citeResult, err := NewCiteSkill().Run(context.Background(), map[string]any{"source_id": "doc-1", "content": "answer"})
	if err != nil {
		t.Fatalf("cite Run() error = %v", err)
	}
	if citeResult.Output != "[doc-1] answer" {
		t.Fatalf("cite output = %v, want citation", citeResult.Output)
	}
}

func TestKnowledgeSkillsRejectInvalidParams(t *testing.T) {
	_, err := NewEmbedSkill(&fakeKnowledgeLLM{}).Run(context.Background(), map[string]any{"text": 1})
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("embed error = %v, want ErrInvalidParams", err)
	}

	_, err = NewVectorSearchSkill(&fakeVectorStore{}).Run(context.Background(), map[string]any{"vector": []any{"bad"}})
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("vector_search error = %v, want ErrInvalidParams", err)
	}
}

func TestKnowledgeSkillsReturnDependencyErrors(t *testing.T) {
	client := &fakeKnowledgeLLM{err: errors.New("embed down")}
	_, err := NewEmbedSkill(client).Run(context.Background(), map[string]any{"text": "hello"})
	if !errors.Is(err, client.err) {
		t.Fatalf("embed error = %v, want dependency error", err)
	}
}

type fakeKnowledgeLLM struct {
	embedding []float64
	err       error
}

func (f *fakeKnowledgeLLM) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}

func (f *fakeKnowledgeLLM) Stream(context.Context, llm.ChatRequest, func(llm.ChatChunk) error) error {
	return nil
}

func (f *fakeKnowledgeLLM) Embed(context.Context, string) ([]float64, error) {
	return f.embedding, f.err
}

func (f *fakeKnowledgeLLM) Summarize(context.Context, types.Conversation) (string, error) {
	return "", nil
}

type fakeVectorStore struct {
	results []store.VectorSearchResult
	err     error
}

func (f *fakeVectorStore) Upsert(context.Context, store.VectorDocument) error {
	return nil
}

func (f *fakeVectorStore) Search(context.Context, []float64, int) ([]store.VectorSearchResult, error) {
	return f.results, f.err
}
