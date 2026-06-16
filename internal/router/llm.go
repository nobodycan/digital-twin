package router

import (
	"context"
	"encoding/json"

	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/pkg/types"
)

// LLMRouter classifies requests through an LLM client and strict JSON output.
type LLMRouter struct {
	client        llm.Client
	minConfidence types.Confidence
}

// NewLLMRouter creates an LLM-backed router.
func NewLLMRouter(client llm.Client) LLMRouter {
	return LLMRouter{client: client, minConfidence: types.Confidence(0.5)}
}

// Route classifies the last user message or returns a persona fallback.
func (r LLMRouter) Route(ctx context.Context, conversation types.Conversation) (types.Intent, error) {
	query := lastUserText(conversation)
	if r.client == nil {
		return llmFallback(query, "missing_client"), nil
	}

	response, err := r.client.Chat(ctx, llm.ChatRequest{
		Messages: []types.Message{
			{Role: types.RoleSystem, Content: "Classify the user request as strict JSON with intent, confidence, and optional entities."},
			{Role: types.RoleUser, Content: query},
		},
		Temperature: 0,
	})
	if err != nil {
		intent := llmFallback(query, "provider_error")
		intent.Metadata["error"] = err.Error()
		return intent, nil
	}

	var parsed classifierResponse
	if err := json.Unmarshal([]byte(response.Message.Content), &parsed); err != nil {
		return llmFallback(query, "invalid_json"), nil
	}

	confidence := types.Confidence(parsed.Confidence)
	if !confidence.Valid() || confidence < r.minConfidence {
		intent := llmFallback(query, "low_confidence")
		intent.Confidence = confidence
		return intent, nil
	}

	intentName := types.IntentName(parsed.Intent)
	if !knownIntent(intentName) {
		return llmFallback(query, "unknown_intent"), nil
	}

	return types.Intent{
		Name:       intentName,
		Query:      query,
		Confidence: confidence,
		Entities:   parsed.Entities,
		Metadata: types.Metadata{
			"source": "llm",
		},
	}, nil
}

func knownIntent(intent types.IntentName) bool {
	switch intent {
	case types.IntentKnowledgeQuery,
		types.IntentMemoryRecall,
		types.IntentTaskExecution,
		types.IntentToolCall,
		types.IntentPersonaChat,
		types.IntentSafetyCheck:
		return true
	default:
		return false
	}
}

type classifierResponse struct {
	Intent     string         `json:"intent"`
	Confidence float64        `json:"confidence"`
	Entities   types.Metadata `json:"entities,omitempty"`
}

func llmFallback(query, reason string) types.Intent {
	return types.Intent{
		Name:       types.IntentPersonaChat,
		Query:      query,
		Confidence: types.Confidence(0.3),
		Metadata: types.Metadata{
			"source": "llm_fallback",
			"reason": reason,
		},
	}
}
