package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/observability"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestHandlerServesHealth(t *testing.T) {
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics()})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %q, want status ok", response.Body.String())
	}
}

func TestHandlerServesMetrics(t *testing.T) {
	metrics := observability.NewMemoryMetrics()
	metrics.IncCounter("requests_total", map[string]string{"route": "/health"})
	handler := NewHandler(Config{Metrics: metrics})
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain; version=0.0.4" {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if !strings.Contains(response.Body.String(), `requests_total{route="/health"} 1`) {
		t.Fatalf("body = %q, want requests_total metric", response.Body.String())
	}
}

func TestHandlerServesChat(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: stubOrchestrator{result: types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "hello back"}, Confidence: 0.8}},
	})
	body := `{"id":"conv-1","tenant_id":"tenant-1","user_id":"user-1","messages":[{"id":"msg-1","role":"user","content":"hello","created_at":"2026-06-16T12:00:00Z"}],"created_at":"2026-06-16T12:00:00Z","updated_at":"2026-06-16T12:00:00Z"}`
	request := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var result types.AgentResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.AgentName != "persona-agent" || result.Message.Content != "hello back" {
		t.Fatalf("result = %#v", result)
	}
}

func TestHandlerRejectsInvalidChatJSON(t *testing.T) {
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), Orchestrator: stubOrchestrator{}})
	request := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader("{bad json"))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), "invalid_json") {
		t.Fatalf("body = %q, want invalid_json", response.Body.String())
	}
}

func TestHandlerRequiresAPIKeyWhenConfigured(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: stubOrchestrator{},
		APIKeys:      []string{"secret"},
	})
	request := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestHandlerAllowsValidAPIKey(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: stubOrchestrator{result: types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "ok"}}},
		APIKeys:      []string{"secret"},
	})
	request := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(validChatJSON()))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHandlerRateLimitsByAPIKey(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:           observability.NewMemoryMetrics(),
		Orchestrator:      stubOrchestrator{result: types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "ok"}}},
		APIKeys:           []string{"secret"},
		RateLimitRequests: 1,
	})
	first := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(validChatJSON()))
	first.Header.Set("Authorization", "Bearer secret")
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, first)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status = %d", firstResponse.Code)
	}

	second := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(validChatJSON()))
	second.Header.Set("Authorization", "Bearer secret")
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, second)

	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", secondResponse.Code)
	}
}

func TestHandlerServesChatStream(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: stubOrchestrator{result: types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "streamed ok"}}},
	})
	request := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(validChatJSON()))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", contentType)
	}
	body := response.Body.String()
	for _, want := range []string{"event: message_completed", "data: streamed ok", "event: done"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerEscapesMultilineChatStreamData(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: stubOrchestrator{result: types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "line one\nevent: injected\ndata: bad"}}},
	})
	request := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(validChatJSON()))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	body := response.Body.String()
	if strings.Contains(body, "\nevent: injected") {
		t.Fatalf("stream contains injectable event frame:\n%s", body)
	}
	for _, want := range []string{"data: line one", "data: event: injected", "data: data: bad"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func validChatJSON() string {
	return `{"id":"conv-1","tenant_id":"tenant-1","user_id":"user-1","messages":[{"id":"msg-1","role":"user","content":"hello","created_at":"2026-06-16T12:00:00Z"}],"created_at":"2026-06-16T12:00:00Z","updated_at":"2026-06-16T12:00:00Z"}`
}

type stubOrchestrator struct {
	result types.AgentResult
	err    error
}

func (s stubOrchestrator) Handle(_ context.Context, conversation types.Conversation) (types.AgentResult, error) {
	if conversation.CreatedAt.IsZero() {
		conversation.CreatedAt = time.Now()
	}
	return s.result, s.err
}
