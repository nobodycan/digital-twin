package memory

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

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

	system := collectSystemMessages(conversation.Messages)
	nonSystem := collectNonSystemMessages(conversation.Messages)
	selected := selectRecentTurns(nonSystem, m.maxBudget)
	window := conversation
	window.Messages = append([]types.Message{}, system...)
	window.Messages = append(window.Messages, selected...)
	return window, nil
}

func estimate(content string) int {
	fields := strings.Fields(content)
	if len(fields) > 0 {
		if len(fields) == 1 && !strings.ContainsAny(content, " \t\r\n") && containsNonASCII(content) {
			return utf8.RuneCountInString(content)
		}
		return len(fields)
	}
	if content == "" {
		return 0
	}
	return utf8.RuneCountInString(content)
}

func collectSystemMessages(messages []types.Message) []types.Message {
	system := make([]types.Message, 0)
	for _, message := range messages {
		if message.Role == types.RoleSystem {
			system = append(system, message)
		}
	}
	return system
}

func collectNonSystemMessages(messages []types.Message) []types.Message {
	nonSystem := make([]types.Message, 0, len(messages))
	for _, message := range messages {
		if message.Role != types.RoleSystem {
			nonSystem = append(nonSystem, message)
		}
	}
	return nonSystem
}

func selectRecentTurns(messages []types.Message, maxBudget int) []types.Message {
	if len(messages) == 0 {
		return nil
	}

	selected := make([]types.Message, 0, len(messages))
	remaining := maxBudget
	index := len(messages) - 1

	if messages[index].Role == types.RoleUser {
		selected = append([]types.Message{messages[index]}, selected...)
		remaining -= estimate(messages[index].Content)
		index--
	}

	for index >= 1 {
		assistant := messages[index]
		user := messages[index-1]
		if assistant.Role != types.RoleAssistant || user.Role != types.RoleUser {
			break
		}

		pairCost := estimate(user.Content) + estimate(assistant.Content)
		if pairCost > remaining {
			break
		}

		selected = append([]types.Message{user, assistant}, selected...)
		remaining -= pairCost
		index -= 2
	}

	return selected
}

func containsNonASCII(value string) bool {
	for _, r := range value {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}
