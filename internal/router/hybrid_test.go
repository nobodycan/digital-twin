package router

import (
	"context"
	"errors"
	"testing"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestHybridRouterUsesRuleHitBeforeLLM(t *testing.T) {
	r := NewHybridRouter(stubRouter{intent: types.Intent{Name: types.IntentKnowledgeQuery, Query: "search", Confidence: 0.9}}, panicRouter{})

	intent, err := r.Route(context.Background(), conversationWithUserText("search docs"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentKnowledgeQuery {
		t.Fatalf("Route() intent = %q, want rule intent", intent.Name)
	}
	if intent.Metadata["source"] != "hybrid_rule" {
		t.Fatalf("Route() source = %v, want hybrid_rule", intent.Metadata["source"])
	}
}

func TestHybridRouterUsesLLMFallbackWhenRuleLowConfidence(t *testing.T) {
	r := NewHybridRouter(
		stubRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "unclear", Confidence: 0.2}},
		stubRouter{intent: types.Intent{Name: types.IntentTaskExecution, Query: "unclear", Confidence: 0.8}},
	)

	intent, err := r.Route(context.Background(), conversationWithUserText("unclear"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentTaskExecution {
		t.Fatalf("Route() intent = %q, want LLM intent", intent.Name)
	}
	if intent.Metadata["source"] != "hybrid_llm" {
		t.Fatalf("Route() source = %v, want hybrid_llm", intent.Metadata["source"])
	}
}

func TestHybridRouterPreservesPersonaChatWhenRuleIsConfidentAndLLMUnavailable(t *testing.T) {
	r := NewHybridRouter(
		stubRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.8}},
		nil,
	)

	intent, err := r.Route(context.Background(), conversationWithUserText("hello"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentPersonaChat {
		t.Fatalf("Route() intent = %q, want persona chat", intent.Name)
	}
	if intent.Confidence != types.Confidence(0.8) {
		t.Fatalf("Route() confidence = %v, want 0.8", intent.Confidence)
	}
	if intent.Metadata["source"] != "hybrid_rule" {
		t.Fatalf("Route() source = %v, want hybrid_rule", intent.Metadata["source"])
	}
}

func TestHybridRouterReturnsPersonaFallbackWhenBothLowConfidence(t *testing.T) {
	r := NewHybridRouter(
		stubRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hi", Confidence: 0.2}},
		stubRouter{intent: types.Intent{Name: types.IntentToolCall, Query: "hi", Confidence: 0.2}},
	)

	intent, err := r.Route(context.Background(), conversationWithUserText("hi"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentPersonaChat {
		t.Fatalf("Route() intent = %q, want persona fallback", intent.Name)
	}
	if intent.Metadata["source"] != "hybrid_fallback" {
		t.Fatalf("Route() source = %v, want hybrid_fallback", intent.Metadata["source"])
	}
}

func TestHybridRouterFallsBackWhenRoutersFail(t *testing.T) {
	r := NewHybridRouter(
		stubRouter{err: errors.New("rule failed")},
		stubRouter{err: errors.New("llm failed")},
	)

	intent, err := r.Route(context.Background(), conversationWithUserText("hi"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if intent.Name != types.IntentPersonaChat {
		t.Fatalf("Route() intent = %q, want persona fallback", intent.Name)
	}
	if intent.Metadata["rule_error"] == "" || intent.Metadata["llm_error"] == "" {
		t.Fatalf("Route() metadata = %#v, want both errors", intent.Metadata)
	}
}

type stubRouter struct {
	intent types.Intent
	err    error
}

func (s stubRouter) Route(context.Context, types.Conversation) (types.Intent, error) {
	return s.intent, s.err
}

type panicRouter struct{}

func (panicRouter) Route(context.Context, types.Conversation) (types.Intent, error) {
	panic("LLM router should not be called")
}
