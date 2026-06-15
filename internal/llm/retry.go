package llm

import (
	"context"
	"errors"
	"time"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// RetryConfig configures retry behavior for retryable LLM failures.
type RetryConfig struct {
	MaxAttempts int
	Backoff     time.Duration
}

// RetryClient decorates a Client with retry behavior.
type RetryClient struct {
	next   Client
	config RetryConfig
}

// NewRetryClient wraps client with retry behavior.
func NewRetryClient(client Client, config RetryConfig) *RetryClient {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 1
	}
	return &RetryClient{next: client, config: config}
}

// Chat retries retryable chat failures until success, cancellation, or max attempts.
func (c *RetryClient) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	var last error
	for attempt := 0; attempt < c.config.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return ChatResponse{}, err
		}
		response, err := c.next.Chat(ctx, request)
		if err == nil {
			return response, nil
		}
		last = err
		if !errors.Is(err, ErrRetryable) {
			return ChatResponse{}, err
		}
		if attempt == c.config.MaxAttempts-1 {
			break
		}
		if err := sleepContext(ctx, c.config.Backoff); err != nil {
			return ChatResponse{}, err
		}
	}
	return ChatResponse{}, last
}

// Stream delegates to the wrapped client without retrying partial streams.
func (c *RetryClient) Stream(ctx context.Context, request ChatRequest, onChunk func(ChatChunk) error) error {
	return c.next.Stream(ctx, request, onChunk)
}

// Embed delegates embedding to the wrapped client.
func (c *RetryClient) Embed(ctx context.Context, text string) ([]float64, error) {
	return c.next.Embed(ctx, text)
}

// Summarize delegates summarization to the wrapped client.
func (c *RetryClient) Summarize(ctx context.Context, conversation types.Conversation) (string, error) {
	return c.next.Summarize(ctx, conversation)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
