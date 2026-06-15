package testutil

import (
	"context"
	"sync"

	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/memory"
	"github.com/nobodycan/digital-twin/internal/runtime"
	"github.com/nobodycan/digital-twin/internal/store"
	"github.com/nobodycan/digital-twin/pkg/types"
)

// FakeLLM is a deterministic llm.Client implementation for tests.
type FakeLLM struct {
	ChatResponse llm.ChatResponse
	ChatErr      error
	Summary      string
	SummaryErr   error
	Vector       []float64
	EmbedErr     error

	mu             sync.Mutex
	summarizeCalls []types.Conversation
	embedCalls     []string
}

// Chat returns the configured chat response or error.
func (f *FakeLLM) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return f.ChatResponse, f.ChatErr
}

// Stream emits the configured chat response as one chunk.
func (f *FakeLLM) Stream(_ context.Context, _ llm.ChatRequest, onChunk func(llm.ChatChunk) error) error {
	if f.ChatErr != nil {
		return f.ChatErr
	}
	if err := onChunk(llm.ChatChunk{Content: f.ChatResponse.Message.Content}); err != nil {
		return err
	}
	return onChunk(llm.ChatChunk{Done: true})
}

// Embed records text and returns the configured vector or error.
func (f *FakeLLM) Embed(_ context.Context, text string) ([]float64, error) {
	f.mu.Lock()
	f.embedCalls = append(f.embedCalls, text)
	f.mu.Unlock()
	return append([]float64(nil), f.Vector...), f.EmbedErr
}

// Summarize records the conversation and returns the configured summary or error.
func (f *FakeLLM) Summarize(_ context.Context, conversation types.Conversation) (string, error) {
	f.mu.Lock()
	f.summarizeCalls = append(f.summarizeCalls, conversation)
	f.mu.Unlock()
	return f.Summary, f.SummaryErr
}

// SummarizeCalls returns recorded summarize calls.
func (f *FakeLLM) SummarizeCalls() []types.Conversation {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := make([]types.Conversation, len(f.summarizeCalls))
	copy(calls, f.summarizeCalls)
	return calls
}

// EmbedCalls returns recorded embed calls.
func (f *FakeLLM) EmbedCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := make([]string, len(f.embedCalls))
	copy(calls, f.embedCalls)
	return calls
}

// FakeStore wraps the in-memory store for tests.
type FakeStore struct {
	*store.InMemoryStore
}

// NewFakeStore creates a fake Store.
func NewFakeStore() *FakeStore {
	return &FakeStore{InMemoryStore: store.NewInMemoryStore()}
}

// FakeVectorStore wraps the in-memory vector store for tests.
type FakeVectorStore struct {
	*store.InMemoryVectorStore
}

// NewFakeVectorStore creates a fake VectorStore.
func NewFakeVectorStore(dimension int) *FakeVectorStore {
	return &FakeVectorStore{InMemoryVectorStore: store.NewInMemoryVectorStore(dimension)}
}

// FakeMemory is a deterministic memory.Memory implementation.
type FakeMemory struct {
	Records []memory.Record
	Err     error
}

// Remember returns the configured error.
func (f *FakeMemory) Remember(context.Context, types.Conversation) error {
	return f.Err
}

// Recall returns the configured records or error.
func (f *FakeMemory) Recall(context.Context, string, string, string, int) ([]memory.Record, error) {
	return f.Records, f.Err
}

// FakeEventBus wraps the local runtime event bus for tests.
type FakeEventBus struct {
	*runtime.LocalEventBus
}

// NewFakeEventBus creates a fake EventBus.
func NewFakeEventBus() *FakeEventBus {
	return &FakeEventBus{LocalEventBus: runtime.NewEventBus()}
}

var _ llm.Client = (*FakeLLM)(nil)
var _ store.Store = (*FakeStore)(nil)
var _ store.VectorStore = (*FakeVectorStore)(nil)
var _ memory.Memory = (*FakeMemory)(nil)
var _ runtime.EventBus = (*FakeEventBus)(nil)
