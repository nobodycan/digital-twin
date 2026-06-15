package llm

import (
	"context"
	"errors"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// ErrRetryable marks a transient provider error that can be retried.
var ErrRetryable = errors.New("retryable llm error")

// Client describes provider-neutral language model operations.
type Client interface {
	Chat(context.Context, ChatRequest) (ChatResponse, error)
	Stream(context.Context, ChatRequest, func(ChatChunk) error) error
	Embed(context.Context, string) ([]float64, error)
	Summarize(context.Context, types.Conversation) (string, error)
}

// ChatRequest contains messages and model parameters for a chat request.
type ChatRequest struct {
	Model       string          `json:"model,omitempty"`
	Messages    []types.Message `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Metadata    types.Metadata  `json:"metadata,omitempty"`
}

// Usage reports token counts returned by a provider.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse contains a model response message and usage.
type ChatResponse struct {
	Message types.Message `json:"message"`
	Usage   Usage         `json:"usage"`
}

// ChatChunk is a streamed chat delta.
type ChatChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done,omitempty"`
}
