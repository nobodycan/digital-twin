package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
