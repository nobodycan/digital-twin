package router

import (
	"context"
	"errors"
	"testing"

	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestLLMRouterParsesStrictJSONIntent(t *testing.T) {
	client := &fakeLLMClient{
		response: llm.ChatResponse{
			Message: types.Message{Role: types.RoleAssistant, Content: `{"intent":"task.execute","confidence":0.82,"entities":{"kind":"plan"}}`},
		},
	}
	r := NewLLMRouter(client)

	intent, err := r.Route(context.Background(), conversationWithUserText("make a plan"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentTaskExecution {
		t.Fatalf("Route() intent = %q, want task", intent.Name)
	}
	if intent.Confidence != types.Confidence(0.82) {
		t.Fatalf("Route() confidence = %v, want 0.82", intent.Confidence)
	}
	if intent.Entities["kind"] != "plan" {
		t.Fatalf("Route() entities = %#v, want kind", intent.Entities)
	}
	if intent.Metadata["source"] != "llm" {
		t.Fatalf("Route() source = %v, want llm", intent.Metadata["source"])
	}
}

func TestLLMRouterFallsBackOnInvalidJSON(t *testing.T) {
	r := NewLLMRouter(&fakeLLMClient{
		response: llm.ChatResponse{Message: types.Message{Role: types.RoleAssistant, Content: `not-json`}},
	})

	intent, err := r.Route(context.Background(), conversationWithUserText("classify this"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentPersonaChat {
		t.Fatalf("Route() intent = %q, want persona fallback", intent.Name)
	}
	if intent.Metadata["source"] != "llm_fallback" {
		t.Fatalf("Route() source = %v, want llm_fallback", intent.Metadata["source"])
	}
}

func TestLLMRouterFallsBackOnLowConfidence(t *testing.T) {
	r := NewLLMRouter(&fakeLLMClient{
		response: llm.ChatResponse{Message: types.Message{Role: types.RoleAssistant, Content: `{"intent":"tool.call","confidence":0.2}`}},
	})

	intent, err := r.Route(context.Background(), conversationWithUserText("maybe do something"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentPersonaChat {
		t.Fatalf("Route() intent = %q, want persona fallback", intent.Name)
	}
}

func TestLLMRouterFallsBackOnUnknownIntent(t *testing.T) {
	r := NewLLMRouter(&fakeLLMClient{
		response: llm.ChatResponse{Message: types.Message{Role: types.RoleAssistant, Content: `{"intent":"admin.delete","confidence":0.9}`}},
	})

	intent, err := r.Route(context.Background(), conversationWithUserText("do something unsafe"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentPersonaChat {
		t.Fatalf("Route() intent = %q, want persona fallback", intent.Name)
	}
	if intent.Metadata["reason"] != "unknown_intent" {
		t.Fatalf("Route() reason = %v, want unknown_intent", intent.Metadata["reason"])
	}
}

func TestLLMRouterFallsBackOnProviderError(t *testing.T) {
	r := NewLLMRouter(&fakeLLMClient{err: errors.New("provider down")})

	intent, err := r.Route(context.Background(), conversationWithUserText("classify this"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentPersonaChat {
		t.Fatalf("Route() intent = %q, want persona fallback", intent.Name)
	}
	if intent.Metadata["error"] == "" {
		t.Fatalf("Route() metadata = %#v, want error", intent.Metadata)
	}
}

type fakeLLMClient struct {
	response llm.ChatResponse
	err      error
}

func (f *fakeLLMClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return f.response, f.err
}

func (f *fakeLLMClient) Stream(context.Context, llm.ChatRequest, func(llm.ChatChunk) error) error {
	return nil
}

func (f *fakeLLMClient) Embed(context.Context, string) ([]float64, error) {
	return nil, nil
}

func (f *fakeLLMClient) Summarize(context.Context, types.Conversation) (string, error) {
	return "", nil
}
