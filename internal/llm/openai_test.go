package llm

import (
	"context"
	"errors"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestOpenAIClientSendsChatRequestAndParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header %q", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["model"] != "test-model" {
			t.Fatalf("expected model test-model, got %#v", body["model"])
		}
		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Fatalf("messages = %#v, want one message", body["messages"])
		}
		message, ok := messages[0].(map[string]any)
		if !ok || message["role"] != "user" || message["content"] != "hi" {
			t.Fatalf("message = %#v, want user hi", messages[0])
		}
		if stream, exists := body["stream"]; exists && stream != false {
			t.Fatalf("stream = %#v, want false or omitted", stream)
		}

		writeResponse(t, w, "{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"hello\"}}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1,\"total_tokens\":3}}")
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, APIKey: "test-key", Model: "test-model", HTTPClient: server.Client()})

	response, err := client.Chat(t.Context(), ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if response.Message.Role != types.RoleAssistant || response.Message.Content != "hello" {
		t.Fatalf("unexpected message %#v", response.Message)
	}
	if response.Usage.TotalTokens != 3 {
		t.Fatalf("expected total tokens 3, got %d", response.Usage.TotalTokens)
	}
}

func TestOpenAIClientStreamsChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeResponseLine(t, w, "data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}")
		writeResponseLine(t, w, "")
		writeResponseLine(t, w, "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}")
		writeResponseLine(t, w, "")
		writeResponseLine(t, w, `data: [DONE]`)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, APIKey: "test-key", Model: "test-model", HTTPClient: server.Client()})

	var got string
	if err := client.Stream(t.Context(), ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}}, func(chunk ChatChunk) error {
		got += chunk.Content
		return nil
	}); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected streamed content %q, got %q", "hello", got)
	}
}

func TestOpenAIClientStreamEmitsDoneOnExplicitDoneEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeResponseLine(t, w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}")
		writeResponseLine(t, w, "")
		writeResponseLine(t, w, "data: [DONE]")
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, HTTPClient: server.Client()})

	var chunks []ChatChunk
	if err := client.Stream(t.Context(), ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}}, func(chunk ChatChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunks len = %d, want 2", len(chunks))
	}
	if chunks[0].Content != "hi" || chunks[0].Done {
		t.Fatalf("first chunk = %#v, want content only", chunks[0])
	}
	if !chunks[1].Done || chunks[1].Content != "" {
		t.Fatalf("last chunk = %#v, want done sentinel", chunks[1])
	}
}

func TestOpenAIClientStreamEmitsDoneOnEOFWithoutDoneEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeResponseLine(t, w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}")
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, HTTPClient: server.Client()})

	var chunks []ChatChunk
	if err := client.Stream(t.Context(), ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}}, func(chunk ChatChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunks len = %d, want 2", len(chunks))
	}
	if !chunks[1].Done {
		t.Fatalf("final chunk = %#v, want done sentinel", chunks[1])
	}
}

func TestOpenAIClientStreamReturnsProviderFailureOnMalformedChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeResponseLine(t, w, "data: {bad json")
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, HTTPClient: server.Client()})

	err := client.Stream(t.Context(), ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}}, func(ChatChunk) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected malformed stream error")
	}
	if !core.IsProviderFailure(err) {
		t.Fatalf("expected provider failure, got %v", err)
	}
}

func TestOpenAIClientStreamPropagatesCallbackError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeResponseLine(t, w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}")
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	want := errors.New("sink failed")

	err := client.Stream(t.Context(), ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}}, func(ChatChunk) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected callback error %v, got %v", want, err)
	}
}

func TestOpenAIClientStreamSupportsLargeChunks(t *testing.T) {
	large := strings.Repeat("x", 128*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeResponseLine(t, w, fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%q}}]}", large))
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, HTTPClient: server.Client()})

	var got string
	if err := client.Stream(t.Context(), ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}}, func(chunk ChatChunk) error {
		got += chunk.Content
		return nil
	}); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got != large {
		t.Fatalf("streamed content len = %d, want %d", len(got), len(large))
	}
}

func TestOpenAIClientStatusErrorRedactsSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "provider said sk-live-super-secret exploded", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, HTTPClient: server.Client()})

	err := client.Stream(t.Context(), ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}}, func(ChatChunk) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected status error")
	}
	if !core.IsProviderFailure(err) {
		t.Fatalf("expected provider failure, got %v", err)
	}
	if strings.Contains(err.Error(), "sk-live-super-secret") {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func TestOpenAIClientStreamRespectsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not implement http.Flusher")
		}
		writeResponseLine(t, w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}")
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	ctx, cancel := context.WithCancel(t.Context())

	var seen int
	err := client.Stream(ctx, ChatRequest{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}}, func(chunk ChatChunk) error {
		seen++
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		t.Fatalf("expected cancellation-ish error, got %v", err)
	}
	if seen == 0 {
		t.Fatal("expected at least one chunk before cancellation")
	}
}

func writeResponse(t *testing.T, w http.ResponseWriter, value string) {
	t.Helper()

	if _, err := fmt.Fprint(w, value); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func writeResponseLine(t *testing.T, w http.ResponseWriter, value string) {
	t.Helper()

	if _, err := fmt.Fprintln(w, value); err != nil {
		t.Fatalf("write response line: %v", err)
	}
}
