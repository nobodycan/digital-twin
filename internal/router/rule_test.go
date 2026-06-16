package router

import (
	"context"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestRuleRouterRoutesCommonIntents(t *testing.T) {
	r := NewRuleRouter()
	tests := []struct {
		name string
		text string
		want types.IntentName
	}{
		{name: "knowledge", text: "search the knowledge base for onboarding", want: types.IntentKnowledgeQuery},
		{name: "memory", text: "do you remember what I said yesterday?", want: types.IntentMemoryRecall},
		{name: "task", text: "please plan the next sprint tasks", want: types.IntentTaskExecution},
		{name: "tool", text: "call the calendar tool for tomorrow", want: types.IntentToolCall},
		{name: "safety", text: "check whether this contains private data", want: types.IntentSafetyCheck},
		{name: "persona", text: "hello, how are you?", want: types.IntentPersonaChat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent, err := r.Route(context.Background(), conversationWithUserText(tt.text))
			if err != nil {
				t.Fatalf("Route() error = %v", err)
			}
			if intent.Name != tt.want {
				t.Fatalf("Route() intent = %q, want %q", intent.Name, tt.want)
			}
			if intent.Query != tt.text {
				t.Fatalf("Route() query = %q, want %q", intent.Query, tt.text)
			}
			if intent.Metadata["source"] != "rule" && intent.Metadata["source"] != "fallback" {
				t.Fatalf("Route() source = %v, want rule or fallback", intent.Metadata["source"])
			}
		})
	}
}

func TestRuleRouterUsesDeterministicPriority(t *testing.T) {
	r := NewRuleRouter()

	intent, err := r.Route(context.Background(), conversationWithUserText("remember to search the knowledge base"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if intent.Name != types.IntentKnowledgeQuery {
		t.Fatalf("Route() intent = %q, want knowledge priority", intent.Name)
	}
	if intent.Metadata["rule"] != "knowledge" {
		t.Fatalf("Route() rule = %v, want knowledge", intent.Metadata["rule"])
	}
}

func conversationWithUserText(text string) types.Conversation {
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	return types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
		Messages: []types.Message{
			{ID: "msg-1", Role: types.RoleUser, Content: text, CreatedAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}
