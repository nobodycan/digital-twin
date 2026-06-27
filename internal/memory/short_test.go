package memory

import (
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestShortTermMemoryKeepsSystemMessagesAndCompleteTurnSuffix(t *testing.T) {
	memory := NewShortTermMemory(5)
	conversation := types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
		Messages: []types.Message{
			msg("system", types.RoleSystem, "always preserve"),
			msg("u1", types.RoleUser, "alpha beta"),
			msg("a1", types.RoleAssistant, "gamma delta"),
			msg("u2", types.RoleUser, "recent ask"),
			msg("a2", types.RoleAssistant, "recent reply"),
		},
	}

	window, err := memory.Window(t.Context(), conversation)
	if err != nil {
		t.Fatalf("Window() error = %v", err)
	}

	got := messageIDs(window.Messages)
	want := []string{"system", "u2", "a2"}
	assertIDs(t, got, want)
}

func TestShortTermMemoryDoesNotSkipAnOversizedMiddlePairToIncludeOlderTurns(t *testing.T) {
	memory := NewShortTermMemory(6)
	conversation := types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
		Messages: []types.Message{
			msg("system", types.RoleSystem, "always preserve"),
			msg("u1", types.RoleUser, "older small"),
			msg("a1", types.RoleAssistant, "older reply"),
			msg("u2", types.RoleUser, "this pair is too large"),
			msg("a2", types.RoleAssistant, "and blocks older inclusion"),
			msg("u3", types.RoleUser, "latest ask"),
			msg("a3", types.RoleAssistant, "latest reply"),
		},
	}

	window, err := memory.Window(t.Context(), conversation)
	if err != nil {
		t.Fatalf("Window() error = %v", err)
	}

	got := messageIDs(window.Messages)
	want := []string{"system", "u3", "a3"}
	assertIDs(t, got, want)
}

func TestShortTermMemoryAlwaysKeepsLatestUserMessageEvenWhenOversized(t *testing.T) {
	memory := NewShortTermMemory(3)
	conversation := types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
		Messages: []types.Message{
			msg("system", types.RoleSystem, "always preserve"),
			msg("u1", types.RoleUser, "older ask"),
			msg("a1", types.RoleAssistant, "older reply"),
			msg("u2", types.RoleUser, "one two three four five"),
		},
	}

	window, err := memory.Window(t.Context(), conversation)
	if err != nil {
		t.Fatalf("Window() error = %v", err)
	}

	got := messageIDs(window.Messages)
	want := []string{"system", "u2"}
	assertIDs(t, got, want)
}

func TestShortTermMemoryEstimatesCJKByRunesInsteadOfOneToken(t *testing.T) {
	memory := NewShortTermMemory(4)
	conversation := types.Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
		Messages: []types.Message{
			msg("system", types.RoleSystem, "always preserve"),
			msg("u1", types.RoleUser, "你好世界"),
			msg("a1", types.RoleAssistant, "收到"),
			msg("u2", types.RoleUser, "next"),
			msg("a2", types.RoleAssistant, "reply"),
		},
	}

	window, err := memory.Window(t.Context(), conversation)
	if err != nil {
		t.Fatalf("Window() error = %v", err)
	}

	got := messageIDs(window.Messages)
	want := []string{"system", "u2", "a2"}
	assertIDs(t, got, want)
}

func TestShortTermMemoryHandlesEmptyConversation(t *testing.T) {
	memory := NewShortTermMemory(1)
	window, err := memory.Window(t.Context(), types.Conversation{ID: "conv-1"})
	if err != nil {
		t.Fatalf("Window() error = %v", err)
	}
	if len(window.Messages) != 0 {
		t.Fatalf("expected no messages, got %#v", window.Messages)
	}
}

func msg(id string, role types.Role, content string) types.Message {
	return types.Message{ID: id, Role: role, Content: content, CreatedAt: time.Now().UTC()}
}

func messageIDs(messages []types.Message) []string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	return ids
}

func assertIDs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("message count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("message[%d] = %q, want %q (all %v)", i, got[i], want[i], got)
		}
	}
}
