package router

import (
	"context"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

// HybridRouter routes rule-first, then LLM, then persona fallback.
type HybridRouter struct {
	rule          core.Router
	llm           core.Router
	minConfidence types.Confidence
}

// NewHybridRouter creates a hybrid router from rule and LLM routers.
func NewHybridRouter(ruleRouter, llmRouter core.Router) HybridRouter {
	return HybridRouter{rule: ruleRouter, llm: llmRouter, minConfidence: types.Confidence(0.5)}
}

// Route classifies conversation while preserving safe fallback behavior.
func (r HybridRouter) Route(ctx context.Context, conversation types.Conversation) (types.Intent, error) {
	query := lastUserText(conversation)
	var ruleErr, llmErr error

	if r.rule != nil {
		intent, err := r.rule.Route(ctx, conversation)
		if err == nil && intent.Confidence >= r.minConfidence && intent.Name != types.IntentPersonaChat {
			return withSource(intent, "hybrid_rule"), nil
		}
		ruleErr = err
	}

	if r.llm != nil {
		intent, err := r.llm.Route(ctx, conversation)
		if err == nil && intent.Confidence >= r.minConfidence && intent.Name != types.IntentPersonaChat {
			return withSource(intent, "hybrid_llm"), nil
		}
		llmErr = err
	}

	intent := types.Intent{
		Name:       types.IntentPersonaChat,
		Query:      query,
		Confidence: types.Confidence(0.3),
		Metadata: types.Metadata{
			"source": "hybrid_fallback",
		},
	}
	if ruleErr != nil {
		intent.Metadata["rule_error"] = ruleErr.Error()
	}
	if llmErr != nil {
		intent.Metadata["llm_error"] = llmErr.Error()
	}
	return intent, nil
}

func withSource(intent types.Intent, source string) types.Intent {
	if intent.Metadata == nil {
		intent.Metadata = types.Metadata{}
	}
	intent.Metadata["source"] = source
	return intent
}
