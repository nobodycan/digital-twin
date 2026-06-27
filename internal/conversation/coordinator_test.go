package conversation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/memory"
	"github.com/nobodycan/digital-twin/internal/store"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestCoordinatorBeginCreatesConversationAndGeneratingAttempt(t *testing.T) {
	c := newTestCoordinator(store.NewInMemoryStore())

	session, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-1", "hello"), "req-1")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer session.Release()

	if len(session.Conversation.Messages) != 1 || session.Conversation.Messages[0].Content != "hello" {
		t.Fatalf("conversation messages = %#v", session.Conversation.Messages)
	}
	if len(session.Conversation.Turns) != 1 {
		t.Fatalf("turn count = %d, want 1", len(session.Conversation.Turns))
	}
	turn := session.Conversation.Turns[0]
	if turn.Status != types.TurnOpen || len(turn.Attempts) != 1 || turn.Attempts[0].Status != types.AttemptGenerating {
		t.Fatalf("turn = %#v", turn)
	}
	if len(session.Window.Messages) != 1 || session.Window.Messages[0].ID != "msg-turn-1" {
		t.Fatalf("window messages = %#v", session.Window.Messages)
	}
}

func TestCoordinatorCompletePersistsAssistantAndReleasesLock(t *testing.T) {
	s := store.NewInMemoryStore()
	c := newTestCoordinator(s)
	session, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-1", "hello"), "req-1")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	result := types.AgentResult{
		AgentName: "persona-agent",
		Message: types.Message{
			ID:        "assistant-1",
			Role:      types.RoleAssistant,
			Content:   "Hi there.",
			CreatedAt: time.Date(2026, 6, 26, 10, 0, 1, 0, time.UTC),
		},
		Confidence: 0.9,
	}
	if err := c.Complete(t.Context(), session, result); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	updated, err := s.GetConversation(t.Context(), "tenant-1", "user-1", "conv-1")
	if err != nil {
		t.Fatalf("GetConversation() error = %v", err)
	}
	if len(updated.Messages) != 2 || updated.Messages[1].ID != "assistant-1" {
		t.Fatalf("messages = %#v", updated.Messages)
	}
	if updated.Turns[0].Status != types.TurnCompleted || updated.Turns[0].AssistantMessageID != "assistant-1" || updated.Turns[0].Result == nil {
		t.Fatalf("turn = %#v", updated.Turns[0])
	}

	nextSession, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-2", "attempt-1", "next"), "req-2")
	if err != nil {
		t.Fatalf("Begin() after complete error = %v", err)
	}
	nextSession.Release()
}

func TestCoordinatorCancelPersistsAfterRequestContextCancellation(t *testing.T) {
	s := store.NewInMemoryStore()
	c := newTestCoordinator(s)
	session, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-1", "hello"), "req-1")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := c.Cancel(ctx, session); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	updated, err := s.GetConversation(t.Context(), "tenant-1", "user-1", "conv-1")
	if err != nil {
		t.Fatalf("GetConversation() error = %v", err)
	}
	if updated.Turns[0].Status != types.TurnCanceled || updated.Turns[0].Attempts[0].Status != types.AttemptCanceled {
		t.Fatalf("turn = %#v", updated.Turns[0])
	}
	if len(updated.Messages) != 1 {
		t.Fatalf("messages = %#v", updated.Messages)
	}
}

func TestCoordinatorRetryReusesUserMessageAfterFailure(t *testing.T) {
	s := store.NewInMemoryStore()
	c := newTestCoordinator(s)
	session, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-1", "hello"), "req-1")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := c.Fail(t.Context(), session, "provider_failed"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}

	retry, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-2", "hello"), "req-2")
	if err != nil {
		t.Fatalf("retry Begin() error = %v", err)
	}
	defer retry.Release()

	if len(retry.Conversation.Messages) != 1 {
		t.Fatalf("messages = %#v, want one user message", retry.Conversation.Messages)
	}
	if len(retry.Conversation.Turns[0].Attempts) != 2 {
		t.Fatalf("attempts = %#v", retry.Conversation.Turns[0].Attempts)
	}
	if retry.Conversation.Turns[0].Status != types.TurnOpen || retry.Conversation.Turns[0].Attempts[1].Status != types.AttemptGenerating {
		t.Fatalf("turn = %#v", retry.Conversation.Turns[0])
	}
}

func TestCoordinatorReplaysCompletedTurn(t *testing.T) {
	c := newTestCoordinator(store.NewInMemoryStore())
	session, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-1", "hello"), "req-1")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	result := types.AgentResult{
		AgentName: "persona-agent",
		Message: types.Message{
			ID:        "assistant-1",
			Role:      types.RoleAssistant,
			Content:   "Hello again.",
			CreatedAt: time.Date(2026, 6, 26, 10, 0, 1, 0, time.UTC),
		},
		Confidence: 0.9,
	}
	if err := c.Complete(t.Context(), session, result); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	replay, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-2", "hello"), "req-2")
	if err != nil {
		t.Fatalf("replay Begin() error = %v", err)
	}
	defer replay.Release()

	if !replay.Replayed || replay.ReplayResult == nil {
		t.Fatalf("replay session = %#v", replay)
	}
	if replay.ReplayResult.Message.Content != "Hello again." {
		t.Fatalf("replay result = %#v", replay.ReplayResult)
	}
}

func TestCoordinatorRejectsChangedContentForExistingTurn(t *testing.T) {
	s := store.NewInMemoryStore()
	c := newTestCoordinator(s)
	session, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-1", "hello"), "req-1")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := c.Fail(t.Context(), session, "provider_failed"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}

	_, err = c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-2", "different"), "req-2")
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("Begin() error = %v, want ErrConflict", err)
	}
}

func TestCoordinatorRejectsBusyConversationButAllowsDifferentConversations(t *testing.T) {
	c := newTestCoordinator(store.NewInMemoryStore())
	first, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-1", "hello"), "req-1")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer first.Release()

	_, err = c.Begin(t.Context(), testTurnRequest("conv-1", "turn-2", "attempt-1", "blocked"), "req-2")
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("same conversation error = %v, want ErrConflict", err)
	}

	other, err := c.Begin(t.Context(), testTurnRequest("conv-2", "turn-1", "attempt-1", "parallel"), "req-3")
	if err != nil {
		t.Fatalf("different conversation Begin() error = %v", err)
	}
	other.Release()
}

func TestCoordinatorExcludesFailedTurnsFromLaterModelWindow(t *testing.T) {
	s := store.NewInMemoryStore()
	c := newTestCoordinator(s)

	first, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-1", "attempt-1", "failed user"), "req-1")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := c.Fail(t.Context(), first, "provider_failed"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}

	second, err := c.Begin(t.Context(), testTurnRequest("conv-1", "turn-2", "attempt-1", "fresh user"), "req-2")
	if err != nil {
		t.Fatalf("second Begin() error = %v", err)
	}
	defer second.Release()

	got := []string{}
	for _, message := range second.Window.Messages {
		got = append(got, message.Content)
	}
	if len(got) != 1 || got[0] != "fresh user" {
		t.Fatalf("window contents = %#v", got)
	}
}

func newTestCoordinator(s store.Store) *Coordinator {
	return NewCoordinator(CoordinatorConfig{
		Store:  s,
		Memory: memory.NewShortTermMemory(32),
		Clock: fixedClock{
			now: time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC),
		},
	})
}

func testTurnRequest(conversationID, turnID, attemptID, content string) types.TurnRequest {
	return types.TurnRequest{
		ConversationID: conversationID,
		TenantID:       "tenant-1",
		UserID:         "user-1",
		TurnID:         turnID,
		AttemptID:      attemptID,
		Message: types.Message{
			ID:        "msg-" + turnID,
			Role:      types.RoleUser,
			Content:   content,
			CreatedAt: time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC),
		},
	}
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}
