package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nobodycan/digital-twin/internal/config"
	"github.com/nobodycan/digital-twin/pkg/types"
)

const defaultHTTPTimeout = 30 * time.Second

// NewClientFromConfig constructs an LLM client from app configuration.
func NewClientFromConfig(cfg config.LLMConfig) (Client, error) {
	provider := strings.TrimSpace(cfg.Provider)
	if provider == "" || provider == "local" || provider == "mock" {
		return LocalClient{}, nil
	}
	if provider == "openai-compatible" {
		timeout := defaultHTTPTimeout
		if cfg.TimeoutMS > 0 {
			timeout = time.Duration(cfg.TimeoutMS) * time.Millisecond
		}
		httpClient := &http.Client{Timeout: timeout}
		return NewOpenAIClient(OpenAIConfig{
			BaseURL:    cfg.BaseURL,
			APIKey:     cfg.APIKey,
			Model:      cfg.Model,
			HTTPClient: httpClient,
		}), nil
	}
	return nil, fmt.Errorf("unsupported llm provider %q; supported providers: local, mock, openai-compatible", provider)
}

// LocalClient is a deterministic local-first client used when no provider is configured.
type LocalClient struct{}

func (LocalClient) Chat(_ context.Context, request ChatRequest) (ChatResponse, error) {
	content := "I think I'm running in local deterministic mode with no real model configured."
	if len(request.Messages) > 0 {
		last := request.Messages[len(request.Messages)-1]
		if strings.Contains(strings.ToLower(last.Content), "model") {
			content = "I think I'm running in local deterministic mode with no configured external model."
		}
	}
	return ChatResponse{
		Message: types.Message{Role: types.RoleAssistant, Content: content},
	}, nil
}

func (c LocalClient) Stream(ctx context.Context, request ChatRequest, onChunk func(ChatChunk) error) error {
	response, err := c.Chat(ctx, request)
	if err != nil {
		return err
	}
	if err := onChunk(ChatChunk{Content: response.Message.Content}); err != nil {
		return err
	}
	return onChunk(ChatChunk{Done: true})
}

func (LocalClient) Embed(context.Context, string) ([]float64, error) {
	return nil, nil
}

func (LocalClient) Summarize(context.Context, types.Conversation) (string, error) {
	return "", nil
}
