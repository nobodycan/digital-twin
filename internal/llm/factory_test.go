package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/config"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestNewClientFromConfigDefaultsToLocalClient(t *testing.T) {
	client, err := NewClientFromConfig(config.LLMConfig{})
	if err != nil {
		t.Fatalf("NewClientFromConfig() error = %v", err)
	}

	response, err := client.Chat(t.Context(), ChatRequest{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if response.Message.Role != types.RoleAssistant {
		t.Fatalf("Chat() role = %q, want assistant", response.Message.Role)
	}
	if response.Message.Content == "" {
		t.Fatalf("Chat() content is empty")
	}
	if response.Message.Content != "I think I'm running in local deterministic mode with no real model configured." {
		t.Fatalf("Chat() content = %q, want deterministic local response", response.Message.Content)
	}
}

func TestNewClientFromConfigSupportsMockProvider(t *testing.T) {
	client, err := NewClientFromConfig(config.LLMConfig{Provider: "mock"})
	if err != nil {
		t.Fatalf("NewClientFromConfig() error = %v", err)
	}

	response, err := client.Chat(t.Context(), ChatRequest{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if response.Message.Content != "I think I'm running in local deterministic mode with no real model configured." {
		t.Fatalf("Chat() content = %q, want deterministic mock response", response.Message.Content)
	}
}

func TestNewClientFromConfigCreatesOpenAICompatibleClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, `{"choices":[{"message":{"role":"assistant","content":"configured"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	client, err := NewClientFromConfig(config.LLMConfig{
		Provider: "openai-compatible",
		BaseURL:  server.URL,
		Model:    "factory-model",
		APIKey:   "factory-key",
	})
	if err != nil {
		t.Fatalf("NewClientFromConfig() error = %v", err)
	}

	response, err := client.Chat(t.Context(), ChatRequest{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if response.Message.Content != "configured" {
		t.Fatalf("Chat() content = %q, want configured", response.Message.Content)
	}
}

func TestNewClientFromConfigUsesDefaultHTTPTimeout(t *testing.T) {
	client, err := NewClientFromConfig(config.LLMConfig{
		Provider: "openai-compatible",
		BaseURL:  "https://llm.example.test/v1",
		Model:    "factory-model",
		APIKey:   "factory-key",
	})
	if err != nil {
		t.Fatalf("NewClientFromConfig() error = %v", err)
	}

	openAIClient, ok := client.(*OpenAIClient)
	if !ok {
		t.Fatalf("client type = %T, want *OpenAIClient", client)
	}
	if openAIClient.client.Timeout != 30*time.Second {
		t.Fatalf("HTTP timeout = %s, want 30s", openAIClient.client.Timeout)
	}
}

func TestNewClientFromConfigRejectsUnsupportedProvider(t *testing.T) {
	_, err := NewClientFromConfig(config.LLMConfig{Provider: "claude-direct"})
	if err == nil || !strings.Contains(err.Error(), "local, mock, openai-compatible") {
		t.Fatalf("NewClientFromConfig() error = %v, want supported provider list", err)
	}
}

func TestNewClientFromConfigDoesNotCallNetworkDuringConstruction(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := NewClientFromConfig(config.LLMConfig{
		Provider: "openai-compatible",
		BaseURL:  server.URL,
		Model:    "factory-model",
		APIKey:   "factory-key",
	})
	if err != nil {
		t.Fatalf("NewClientFromConfig() error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("construction made %d network calls, want 0", calls)
	}
}

func TestNewClientFromConfigAppliesHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClientFromConfig(config.LLMConfig{
		Provider:  "openai-compatible",
		BaseURL:   server.URL,
		Model:     "factory-model",
		APIKey:    "factory-key",
		TimeoutMS: 10,
	})
	if err != nil {
		t.Fatalf("NewClientFromConfig() error = %v", err)
	}

	_, err = client.Chat(t.Context(), ChatRequest{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hello"}},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Chat() error = %v, want deadline exceeded", err)
	}
}
