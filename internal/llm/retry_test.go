package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestRetryClientRetriesTransientChatFailure(t *testing.T) {
	attempts := 0
	client := &stubClient{
		chat: func(context.Context, ChatRequest) (ChatResponse, error) {
			attempts++
			if attempts == 1 {
				return ChatResponse{}, ErrRetryable
			}
			return ChatResponse{Message: types.Message{Role: types.RoleAssistant, Content: "ok"}}, nil
		},
	}

	retry := NewRetryClient(client, RetryConfig{MaxAttempts: 2, Backoff: time.Nanosecond})
	response, err := retry.Chat(context.Background(), ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if response.Message.Content != "ok" {
		t.Fatalf("expected ok response, got %#v", response.Message)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryClientStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	retry := NewRetryClient(&stubClient{
		chat: func(context.Context, ChatRequest) (ChatResponse, error) {
			return ChatResponse{}, ErrRetryable
		},
	}, RetryConfig{MaxAttempts: 3, Backoff: time.Hour})

	_, err := retry.Chat(ctx, ChatRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestRetryClientDoesNotRetryPermanentFailure(t *testing.T) {
	attempts := 0
	retry := NewRetryClient(&stubClient{
		chat: func(context.Context, ChatRequest) (ChatResponse, error) {
			attempts++
			return ChatResponse{}, errors.New("permanent")
		},
	}, RetryConfig{MaxAttempts: 3, Backoff: time.Nanosecond})

	_, err := retry.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected one attempt, got %d", attempts)
	}
}

type stubClient struct {
	chat func(context.Context, ChatRequest) (ChatResponse, error)
}

func (s *stubClient) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	return s.chat(ctx, request)
}

func (s *stubClient) Stream(context.Context, ChatRequest, func(ChatChunk) error) error {
	return nil
}

func (s *stubClient) Embed(context.Context, string) ([]float64, error) {
	return nil, nil
}

func (s *stubClient) Summarize(context.Context, types.Conversation) (string, error) {
	return "", nil
}
