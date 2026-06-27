package conversation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/store"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type Clock interface {
	Now() time.Time
}

type windowBuilder interface {
	Window(context.Context, types.Conversation) (types.Conversation, error)
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}

type CoordinatorConfig struct {
	Store  store.Store
	Memory windowBuilder
	Clock  Clock
	Gate   *ActiveConversationGate
}

type Coordinator struct {
	store  store.Store
	memory windowBuilder
	clock  Clock
	gate   *ActiveConversationGate
}

type TurnSession struct {
	Conversation types.Conversation
	Window       types.Conversation
	TurnID       string
	AttemptID    string
	RequestID    string
	Replayed     bool
	ReplayResult *types.AgentResult

	key         string
	release     func()
	releaseOnce sync.Once
}

func NewCoordinator(config CoordinatorConfig) *Coordinator {
	clock := config.Clock
	if clock == nil {
		clock = realClock{}
	}
	gate := config.Gate
	if gate == nil {
		gate = NewActiveConversationGate()
	}
	return &Coordinator{
		store:  config.Store,
		memory: config.Memory,
		clock:  clock,
		gate:   gate,
	}
}

func (c *Coordinator) Begin(ctx context.Context, req types.TurnRequest, requestID string) (*TurnSession, error) {
	if err := req.Validate(); err != nil {
		return nil, core.WrapError(core.ErrInvalidInput, err.Error())
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	key := gateKey(req.TenantID, req.UserID, req.ConversationID)
	release, ok := c.gate.TryAcquire(key)
	if !ok {
		return nil, core.WrapError(core.ErrConflict, "conversation busy")
	}

	session := &TurnSession{
		TurnID:    req.TurnID,
		AttemptID: req.AttemptID,
		RequestID: requestID,
		key:       key,
		release:   release,
	}

	conversation, err := c.loadConversation(ctx, req)
	if err != nil {
		session.Release()
		return nil, err
	}

	turnIndex := findTurnIndex(conversation.Turns, req.TurnID)
	switch {
	case turnIndex >= 0:
		if err := ensureTurnContentMatches(conversation, conversation.Turns[turnIndex], req.Message.Content); err != nil {
			session.Release()
			return nil, err
		}
		turn := &conversation.Turns[turnIndex]
		if hasAttempt(turn.Attempts, req.AttemptID) {
			session.Release()
			return nil, core.WrapError(core.ErrConflict, "attempt already exists")
		}
		if turn.Status == types.TurnCompleted && turn.Result != nil {
			window, err := c.window(ctx, conversation, req.TurnID)
			if err != nil {
				session.Release()
				return nil, err
			}
			session.Conversation = conversation
			session.Window = window
			session.Replayed = true
			result := *turn.Result
			session.ReplayResult = &result
			return session, nil
		}

		turn.Status = types.TurnOpen
		turn.Attempts = append(turn.Attempts, types.TurnAttempt{
			ID:        req.AttemptID,
			Status:    types.AttemptGenerating,
			RequestID: requestID,
			StartedAt: c.clock.Now(),
		})
	default:
		conversation.Messages = append(conversation.Messages, req.Message)
		conversation.Turns = append(conversation.Turns, types.TurnRecord{
			ID:            req.TurnID,
			UserMessageID: req.Message.ID,
			Status:        types.TurnOpen,
			Attempts: []types.TurnAttempt{{
				ID:        req.AttemptID,
				Status:    types.AttemptGenerating,
				RequestID: requestID,
				StartedAt: c.clock.Now(),
			}},
		})
	}

	conversation.UpdatedAt = c.clock.Now()
	if err := c.store.SaveConversation(ctx, conversation); err != nil {
		session.Release()
		return nil, err
	}

	window, err := c.window(ctx, conversation, req.TurnID)
	if err != nil {
		session.Release()
		return nil, err
	}
	session.Conversation = conversation
	session.Window = window
	return session, nil
}

func (c *Coordinator) Complete(ctx context.Context, session *TurnSession, result types.AgentResult) error {
	defer session.Release()

	conversation, err := c.currentConversation(ctx, session)
	if err != nil {
		return err
	}
	turn := findTurn(conversation.Turns, session.TurnID)
	if turn == nil {
		return core.WrapError(core.ErrConflict, "turn missing")
	}
	attempt := findAttempt(turn.Attempts, session.AttemptID)
	if attempt == nil {
		return core.WrapError(core.ErrConflict, "attempt missing")
	}

	attempt.Status = types.AttemptCompleted
	attempt.CompletedAt = c.clock.Now()
	turn.Status = types.TurnCompleted
	turn.AssistantMessageID = result.Message.ID
	turn.Result = cloneAgentResult(result)
	conversation.Messages = append(conversation.Messages, result.Message)
	conversation.UpdatedAt = c.clock.Now()
	return c.store.SaveConversation(ctx, conversation)
}

func (c *Coordinator) Fail(ctx context.Context, session *TurnSession, code string) error {
	return c.finishWithoutAssistant(ctx, session, types.TurnFailed, types.AttemptFailed, code)
}

func (c *Coordinator) Cancel(ctx context.Context, session *TurnSession) error {
	return c.finishWithoutAssistant(ctx, session, types.TurnCanceled, types.AttemptCanceled, "canceled")
}

func (s *TurnSession) Release() {
	if s == nil {
		return
	}
	s.releaseOnce.Do(func() {
		if s.release != nil {
			s.release()
		}
	})
}

func (c *Coordinator) finishWithoutAssistant(ctx context.Context, session *TurnSession, turnStatus types.TurnStatus, attemptStatus types.AttemptStatus, code string) error {
	defer session.Release()

	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()

	conversation, err := c.currentConversation(persistCtx, session)
	if err != nil {
		return err
	}
	turn := findTurn(conversation.Turns, session.TurnID)
	if turn == nil {
		return core.WrapError(core.ErrConflict, "turn missing")
	}
	attempt := findAttempt(turn.Attempts, session.AttemptID)
	if attempt == nil {
		return core.WrapError(core.ErrConflict, "attempt missing")
	}

	turn.Status = turnStatus
	attempt.Status = attemptStatus
	attempt.ErrorCode = code
	attempt.CompletedAt = c.clock.Now()
	conversation.UpdatedAt = c.clock.Now()
	return c.store.SaveConversation(persistCtx, conversation)
}

func (c *Coordinator) loadConversation(ctx context.Context, req types.TurnRequest) (types.Conversation, error) {
	conversation, err := c.store.GetConversation(ctx, req.TenantID, req.UserID, req.ConversationID)
	if err == nil {
		return conversation, nil
	}
	if !errors.Is(err, core.ErrConversationNotFound) {
		return types.Conversation{}, err
	}
	now := c.clock.Now()
	return types.Conversation{
		ID:        req.ConversationID,
		TenantID:  req.TenantID,
		UserID:    req.UserID,
		Messages:  nil,
		Turns:     nil,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (c *Coordinator) currentConversation(ctx context.Context, session *TurnSession) (types.Conversation, error) {
	return c.store.GetConversation(ctx, session.Conversation.TenantID, session.Conversation.UserID, session.Conversation.ID)
}

func (c *Coordinator) window(ctx context.Context, conversation types.Conversation, activeTurnID string) (types.Conversation, error) {
	projected := projectConversation(conversation, activeTurnID)
	if c.memory == nil {
		return projected, nil
	}
	return c.memory.Window(ctx, projected)
}

func projectConversation(conversation types.Conversation, activeTurnID string) types.Conversation {
	allowed := map[string]struct{}{}
	for _, turn := range conversation.Turns {
		switch turn.Status {
		case types.TurnCompleted:
			allowed[turn.UserMessageID] = struct{}{}
			if turn.AssistantMessageID != "" {
				allowed[turn.AssistantMessageID] = struct{}{}
			}
		case types.TurnOpen:
			if turn.ID == activeTurnID {
				allowed[turn.UserMessageID] = struct{}{}
			}
		}
	}

	projected := conversation
	projected.Messages = projected.Messages[:0]
	for _, message := range conversation.Messages {
		if message.Role == types.RoleSystem {
			projected.Messages = append(projected.Messages, message)
			continue
		}
		if _, ok := allowed[message.ID]; ok {
			projected.Messages = append(projected.Messages, message)
		}
	}
	return projected
}

func ensureTurnContentMatches(conversation types.Conversation, turn types.TurnRecord, content string) error {
	for _, message := range conversation.Messages {
		if message.ID == turn.UserMessageID {
			if message.Content != content {
				return core.WrapError(core.ErrConflict, "turn content mismatch")
			}
			return nil
		}
	}
	return core.WrapError(core.ErrConflict, "turn user message missing")
}

func hasAttempt(attempts []types.TurnAttempt, attemptID string) bool {
	return findAttempt(attempts, attemptID) != nil
}

func findTurnIndex(turns []types.TurnRecord, turnID string) int {
	for i := range turns {
		if turns[i].ID == turnID {
			return i
		}
	}
	return -1
}

func findTurn(turns []types.TurnRecord, turnID string) *types.TurnRecord {
	for i := range turns {
		if turns[i].ID == turnID {
			return &turns[i]
		}
	}
	return nil
}

func findAttempt(attempts []types.TurnAttempt, attemptID string) *types.TurnAttempt {
	for i := range attempts {
		if attempts[i].ID == attemptID {
			return &attempts[i]
		}
	}
	return nil
}

func cloneAgentResult(result types.AgentResult) *types.AgentResult {
	cloned := result
	if result.Metadata != nil {
		cloned.Metadata = make(types.Metadata, len(result.Metadata))
		for key, value := range result.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return &cloned
}

func gateKey(tenantID, userID, conversationID string) string {
	return fmt.Sprintf("%s/%s/%s", tenantID, userID, conversationID)
}
