package presentation

import (
	"context"
	"sync"
)

type InterruptionController struct {
	clock  Clock
	mu     sync.Mutex
	active map[string]activeRequest
}

type activeRequest struct {
	cancel  context.CancelFunc
	context EventContext
}

func NewInterruptionController(clock Clock) *InterruptionController {
	if clock == nil {
		clock = systemClock{}
	}
	return &InterruptionController{
		clock:  clock,
		active: make(map[string]activeRequest),
	}
}

func (c *InterruptionController) Begin(parent context.Context, eventContext EventContext) context.Context {
	ctx, cancel := context.WithCancel(parent)
	c.mu.Lock()
	c.active[eventContext.ConversationID] = activeRequest{
		cancel:  cancel,
		context: eventContext,
	}
	c.mu.Unlock()
	return ctx
}

func (c *InterruptionController) Interrupt(conversationID, reason string) (Event, bool) {
	c.mu.Lock()
	active, ok := c.active[conversationID]
	if ok {
		delete(c.active, conversationID)
	}
	c.mu.Unlock()
	if !ok {
		return Event{}, false
	}

	active.cancel()
	eventContext := active.context
	eventContext.Sequence = 1
	eventContext.OccurredAt = c.clock.Now().UTC()
	return NewEvent(EventInterrupted, eventContext, map[string]any{
		"reason": reason,
	}, nil), true
}
