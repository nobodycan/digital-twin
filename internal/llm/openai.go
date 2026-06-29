package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

const maxStreamEventBytes = 1024 * 1024

const (
	ProviderStatusCategory          = "provider_status"
	ProviderNetworkCategory         = "provider_network"
	ProviderStreamDecodeCategory    = "provider_stream_decode"
	ProviderStreamTruncatedCategory = "provider_stream_truncated"
	ProviderEmptyResponseCategory   = "provider_empty_response"
)

var secretPattern = regexp.MustCompile(`sk-[A-Za-z0-9_-]+`)

type ProviderFailureError struct {
	Category string
	Cause    string
	Err      error
}

func (e *ProviderFailureError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == "" {
		return e.Category
	}
	return fmt.Sprintf("%s: %s", e.Category, e.Cause)
}

func (e *ProviderFailureError) Unwrap() []error {
	if e == nil || e.Err == nil {
		return []error{core.ErrProviderFailure}
	}
	return []error{core.ErrProviderFailure, e.Err}
}

func ProviderFailureCategory(err error) string {
	var providerErr *ProviderFailureError
	if errors.As(err, &providerErr) {
		return providerErr.Category
	}
	return ""
}

// OpenAIConfig configures an OpenAI-compatible HTTP client.
type OpenAIConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// OpenAIClient calls an OpenAI-compatible chat completions endpoint.
type OpenAIClient struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAIClient creates an OpenAI-compatible client.
func NewOpenAIClient(config OpenAIConfig) *OpenAIClient {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OpenAIClient{
		baseURL: strings.TrimRight(config.BaseURL, "/"),
		apiKey:  config.APIKey,
		model:   config.Model,
		client:  httpClient,
	}
}

// Chat sends a non-streaming chat completion request.
func (c *OpenAIClient) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	request.Stream = false
	var response openAIChatResponse
	if err := c.doJSON(ctx, request, &response); err != nil {
		return ChatResponse{}, err
	}
	if len(response.Choices) == 0 {
		return ChatResponse{}, core.WrapError(core.ErrProviderFailure, "openai response has no choices")
	}
	message := response.Choices[0].Message
	return ChatResponse{
		Message: types.Message{Role: types.Role(message.Role), Content: message.Content},
		Usage:   response.Usage,
	}, nil
}

// Stream sends a streaming chat completion request and calls onChunk for each content delta.
func (c *OpenAIClient) Stream(ctx context.Context, request ChatRequest, onChunk func(ChatChunk) error) error {
	request.Stream = true
	body, err := c.requestBody(request)
	if err != nil {
		return err
	}
	httpRequest, err := c.newRequest(ctx, body)
	if err != nil {
		return err
	}
	response, err := c.client.Do(httpRequest)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return providerFailure(ProviderNetworkCategory, "openai stream", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return statusError(response)
	}

	var sawContent bool
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxStreamEventBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			if !sawContent {
				return providerFailureMessage(ProviderEmptyResponseCategory, "provider returned no usable content")
			}
			return onChunk(ChatChunk{Done: true})
		}
		var chunk openAIStreamResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return providerFailure(ProviderStreamDecodeCategory, "decode openai stream", err)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		content := chunk.Choices[0].Delta.Content
		if strings.TrimSpace(content) != "" {
			sawContent = true
		}
		if err := onChunk(ChatChunk{Content: content}); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return providerFailure(ProviderNetworkCategory, "read openai stream", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return providerFailureMessage(ProviderStreamTruncatedCategory, "provider stream ended before completion marker")
}

// Embed is intentionally not implemented for OpenAIClient in Phase 1.
func (c *OpenAIClient) Embed(context.Context, string) ([]float64, error) {
	return nil, core.WrapError(core.ErrProviderFailure, "openai embed not implemented")
}

// Summarize is intentionally not implemented for OpenAIClient in Phase 1.
func (c *OpenAIClient) Summarize(context.Context, types.Conversation) (string, error) {
	return "", core.WrapError(core.ErrProviderFailure, "openai summarize not implemented")
}

func (c *OpenAIClient) doJSON(ctx context.Context, request ChatRequest, out any) error {
	body, err := c.requestBody(request)
	if err != nil {
		return err
	}
	httpRequest, err := c.newRequest(ctx, body)
	if err != nil {
		return err
	}
	response, err := c.client.Do(httpRequest)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return providerFailure(ProviderNetworkCategory, "openai chat", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return statusError(response)
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return core.WrapError(err, "decode openai chat")
	}
	return nil
}

func (c *OpenAIClient) requestBody(request ChatRequest) ([]byte, error) {
	if request.Model == "" {
		request.Model = c.model
	}
	body := openAIChatRequest{
		Model:       request.Model,
		Temperature: request.Temperature,
		Stream:      request.Stream,
		Messages:    make([]openAIMessage, 0, len(request.Messages)),
	}
	for _, message := range request.Messages {
		body.Messages = append(body.Messages, openAIMessage{Role: string(message.Role), Content: message.Content})
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, core.WrapError(err, "encode openai chat")
	}
	return data, nil
}

func (c *OpenAIClient) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, core.WrapError(err, "create openai request")
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return request, nil
}

func statusError(response *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	return providerFailureMessage(ProviderStatusCategory, fmt.Sprintf("openai status %d: %s", response.StatusCode, redactSecrets(strings.TrimSpace(string(data)))))
}

func providerFailure(category string, message string, err error) error {
	if err == nil {
		return nil
	}
	return &ProviderFailureError{
		Category: category,
		Cause:    redactSecrets(fmt.Sprintf("%s: %v", message, err)),
		Err:      err,
	}
}

func providerFailureMessage(category string, cause string) error {
	return &ProviderFailureError{
		Category: category,
		Cause:    redactSecrets(cause),
	}
}

func redactSecrets(value string) string {
	if value == "" {
		return ""
	}
	return secretPattern.ReplaceAllString(value, "[REDACTED]")
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

type openAIStreamResponse struct {
	Choices []struct {
		Delta openAIMessage `json:"delta"`
	} `json:"choices"`
}
