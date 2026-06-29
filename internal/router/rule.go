package router

import (
	"context"
	"strings"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// RuleRouter classifies obvious requests with deterministic keyword rules.
type RuleRouter struct {
	rules []rule
}

type rule struct {
	name     string
	intent   types.IntentName
	keywords []string
}

// NewRuleRouter creates a rule router with stable priority order.
func NewRuleRouter() RuleRouter {
	return RuleRouter{
		rules: []rule{
			{name: "knowledge", intent: types.IntentKnowledgeQuery, keywords: []string{"knowledge base", "search", "cite", "source"}},
			{name: "memory", intent: types.IntentMemoryRecall, keywords: []string{"remember", "recall", "yesterday", "last time"}},
			{name: "task", intent: types.IntentTaskExecution, keywords: []string{"plan", "task", "decompose", "sprint"}},
			{name: "tool", intent: types.IntentToolCall, keywords: []string{"tool", "call", "calendar", "http"}},
			{name: "safety", intent: types.IntentSafetyCheck, keywords: []string{"private data", "pii", "prompt injection", "policy"}},
		},
	}
}

// Route returns the first matching rule or persona fallback for small talk.
func (r RuleRouter) Route(_ context.Context, conversation types.Conversation) (types.Intent, error) {
	query := lastUserText(conversation)
	normalized := strings.ToLower(query)
	for _, rule := range r.rules {
		for _, keyword := range rule.keywords {
			if strings.Contains(normalized, keyword) {
				return types.Intent{
					Name:       rule.intent,
					Query:      query,
					Confidence: types.Confidence(0.9),
					Metadata: types.Metadata{
						"source": "rule",
						"rule":   rule.name,
					},
				}, nil
			}
		}
	}

	return types.Intent{
		Name:       types.IntentPersonaChat,
		Query:      query,
		Confidence: types.Confidence(0.8),
		Metadata: types.Metadata{
			"source": "fallback",
			"rule":   "persona",
		},
	}, nil
}

func lastUserText(conversation types.Conversation) string {
	for i := len(conversation.Messages) - 1; i >= 0; i-- {
		if conversation.Messages[i].Role == types.RoleUser {
			return conversation.Messages[i].Content
		}
	}
	return ""
}
