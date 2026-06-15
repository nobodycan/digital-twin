package memory

import (
	"context"
	"strings"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// ShortTermMemory creates deterministic conversation windows.
type ShortTermMemory struct {
	maxBudget int
}

// NewShortTermMemory creates a short-term memory window with a simple word budget.
func NewShortTermMemory(maxBudget int) *ShortTermMemory {
	return &ShortTermMemory{maxBudget: maxBudget}
}

// Window returns a conversation with system messages and recent messages that fit.
func (m *ShortTermMemory) Window(ctx context.Context, conversation types.Conversation) (types.Conversation, error) {
	if err := ctx.Err(); err != nil {
		return types.Conversation{}, err
	}
	if len(conversation.Messages) == 0 {
		return conversation, nil
	}

	system := make([]types.Message, 0)
	remaining := m.maxBudget
	recentReversed := make([]types.Message, 0)
	for i := len(conversation.Messages) - 1; i >= 0; i-- {
		message := conversation.Messages[i]
		if message.Role == types.RoleSystem {
			continue
		}
		cost := estimate(message.Content)
		if cost <= remaining {
			recentReversed = append(recentReversed, message)
			remaining -= cost
		}
	}
	for _, message := range conversation.Messages {
		if message.Role == types.RoleSystem {
			system = append(system, message)
		}
	}
	window := conversation
	window.Messages = append([]types.Message{}, system...)
	for i := len(recentReversed) - 1; i >= 0; i-- {
		window.Messages = append(window.Messages, recentReversed[i])
	}
	return window, nil
}

func estimate(content string) int {
	fields := strings.Fields(content)
	if len(fields) == 0 && content != "" {
		return 1
	}
	return len(fields)
}
