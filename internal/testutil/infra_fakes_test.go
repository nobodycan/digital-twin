package testutil

import (
	"testing"

	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/memory"
	"github.com/nobodycan/digital-twin/internal/runtime"
	"github.com/nobodycan/digital-twin/internal/store"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestInfrastructureFakesImplementInterfaces(t *testing.T) {
	var _ llm.Client = &FakeLLM{}
	var _ store.Store = &FakeStore{}
	var _ store.VectorStore = &FakeVectorStore{}
	var _ memory.Memory = &FakeMemory{}
	var _ runtime.EventBus = &FakeEventBus{}
}

func TestFakeLLMRecordsPrompts(t *testing.T) {
	fake := &FakeLLM{Summary: "summary", Vector: []float64{1, 0}}

	if _, err := fake.Summarize(t.Context(), types.Conversation{TenantID: "tenant", UserID: "user", ID: "conv"}); err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if _, err := fake.Embed(t.Context(), "query"); err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(fake.SummarizeCalls()) != 1 || len(fake.EmbedCalls()) != 1 {
		t.Fatalf("expected calls to be recorded")
	}
}
