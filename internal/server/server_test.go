package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/avatar"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/knowledge"
	"github.com/nobodycan/digital-twin/internal/observability"
	"github.com/nobodycan/digital-twin/internal/presentation"
	"github.com/nobodycan/digital-twin/internal/runtime"
	"github.com/nobodycan/digital-twin/internal/voice"
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

func TestHandlerServesReadinessWhenDependenciesAreReady(t *testing.T) {
	metrics := observability.NewMemoryMetrics()
	handler := NewHandler(Config{
		Metrics: metrics,
		Readiness: ReadinessConfig{
			DataDir:           t.TempDir(),
			ConfigSummary:     "environment=local tts.provider=mock",
			ReleaseGateStatus: "skipped",
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{`"status":"ok"`, `"data_dir":"ok"`, `"release_gate":"skipped"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	if metrics.Snapshot().Gauges[`readiness_status`] != 1 {
		t.Fatalf("readiness_status gauge = %v, want 1", metrics.Snapshot().Gauges)
	}
}

func TestHandlerReadinessFailsWithoutWritableLocalDataDir(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(filePath, []byte("file"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	handler := NewHandler(Config{
		Metrics: observability.NewMemoryMetrics(),
		Readiness: ReadinessConfig{
			DataDir:           filePath,
			ReleaseGateStatus: "skipped",
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"data_dir":"failed"`) {
		t.Fatalf("body = %s, want data_dir failed", response.Body.String())
	}
}

func TestHandlerReadinessRedactsConfigErrors(t *testing.T) {
	handler := NewHandler(Config{
		Metrics: observability.NewMemoryMetrics(),
		Readiness: ReadinessConfig{
			DataDir:           t.TempDir(),
			ConfigError:       errors.New("provider rejected key tts-secret"),
			ReleaseGateStatus: "skipped",
			Redact: func(text string) string {
				return strings.ReplaceAll(text, "tts-secret", "<redacted>")
			},
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Contains(body, "tts-secret") {
		t.Fatalf("readiness leaked secret: %s", body)
	}
	if !strings.Contains(body, `redacted`) || !strings.Contains(body, `"config":"failed"`) {
		t.Fatalf("body = %s, want redacted config failure", body)
	}
}

func TestHandlerReadinessFailsWhenReleaseGateFailed(t *testing.T) {
	handler := NewHandler(Config{
		Metrics: observability.NewMemoryMetrics(),
		Readiness: ReadinessConfig{
			DataDir:           t.TempDir(),
			ReleaseGateStatus: "failed",
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"release_gate":"failed"`) {
		t.Fatalf("body = %s, want release_gate failed", response.Body.String())
	}
}

func TestHandlerAddsRequestIDHeader(t *testing.T) {
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics()})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("X-Request-ID header is empty")
	}
}

func TestHandlerPreservesIncomingRequestID(t *testing.T) {
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics()})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("X-Request-ID", "req-client")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Header().Get("X-Request-ID") != "req-client" {
		t.Fatalf("X-Request-ID = %q, want req-client", response.Header().Get("X-Request-ID"))
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

func TestHandlerServesRuntimeStatusWithoutSecrets(t *testing.T) {
	handler := NewHandler(Config{
		Metrics: observability.NewMemoryMetrics(),
		RuntimeStatus: RuntimeStatus{
			Environment:        "local",
			Provider:           "deepseek",
			Model:              "deepseek-v4-pro",
			FallbackPolicy:     "fail_closed",
			GenerationModeHint: "llm",
			BaseURL:            "https://api.deepseek.com",
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/runtime/status", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{
		`"environment":"local"`,
		`"provider":"deepseek"`,
		`"model":"deepseek-v4-pro"`,
		`"fallback_policy":"fail_closed"`,
		`"generation_mode_hint":"llm"`,
		`"base_url":"https://api.deepseek.com"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	for _, forbidden := range []string{"api_key", "authorization", "sk-"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("runtime status leaked %q:\n%s", forbidden, body)
		}
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

	for _, path := range []string{"/chat", "/experience/mock-voice/stream"} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", path, response.Code)
		}
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

func TestHandlerPrefersStreamingOrchestratorForChatStream(t *testing.T) {
	streaming := &stubStreamingOrchestrator{
		result: types.AgentResult{
			AgentName: "persona-agent",
			Message:   types.Message{ID: "msg-assistant-1", Role: types.RoleAssistant, Content: "streamed from runtime"},
		},
		events: []types.StreamEvent{
			{
				Name:           types.StreamEventRequestStarted,
				RequestID:      "req-1",
				TenantID:       "tenant-1",
				UserID:         "user-1",
				ConversationID: "conv-1",
				TurnID:         "turn-1",
				AttemptID:      "attempt-1",
				Sequence:       1,
				Timestamp:      time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC),
			},
			{
				Name:           types.StreamEventAssistantDelta,
				RequestID:      "req-1",
				TenantID:       "tenant-1",
				UserID:         "user-1",
				ConversationID: "conv-1",
				TurnID:         "turn-1",
				AttemptID:      "attempt-1",
				Sequence:       2,
				Timestamp:      time.Date(2026, 6, 26, 10, 0, 1, 0, time.UTC),
				Payload:        types.Metadata{"content": "streamed from runtime"},
			},
			{
				Name:           types.StreamEventMessageCompleted,
				RequestID:      "req-1",
				TenantID:       "tenant-1",
				UserID:         "user-1",
				ConversationID: "conv-1",
				TurnID:         "turn-1",
				AttemptID:      "attempt-1",
				Sequence:       3,
				Timestamp:      time.Date(2026, 6, 26, 10, 0, 2, 0, time.UTC),
				Payload:        types.Metadata{"content": "streamed from runtime"},
			},
			{
				Name:           types.StreamEventDone,
				RequestID:      "req-1",
				TenantID:       "tenant-1",
				UserID:         "user-1",
				ConversationID: "conv-1",
				TurnID:         "turn-1",
				AttemptID:      "attempt-1",
				Sequence:       4,
				Timestamp:      time.Date(2026, 6, 26, 10, 0, 3, 0, time.UTC),
				Payload:        types.Metadata{"status": "completed"},
			},
		},
	}
	handler := NewHandler(Config{
		Metrics:         observability.NewMemoryMetrics(),
		Orchestrator:    streaming,
		DefaultTenantID: "tenant-default",
		DefaultUserID:   "user-default",
	})
	body := `{"conversation_id":"conv-1","tenant_id":"tenant-attacker","user_id":"user-attacker","turn_id":"turn-1","attempt_id":"attempt-1","message":{"id":"msg-1","role":"user","content":"hello","created_at":"2026-06-16T12:00:00Z"}}`
	request := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !streaming.streamCalled {
		t.Fatal("expected streaming orchestrator to be used")
	}
	if streaming.handleCalled {
		t.Fatal("expected legacy Handle() path to be skipped")
	}
	got := response.Body.String()
	for _, want := range []string{"event: assistant_text_delta", "data: {\"name\":\"assistant_text_delta\"", "event: done"} {
		if !strings.Contains(got, want) {
			t.Fatalf("body missing %q:\n%s", want, got)
		}
	}
	if streaming.request.TurnID != "turn-1" || streaming.request.AttemptID != "attempt-1" {
		t.Fatalf("request = %#v, want turn request", streaming.request)
	}
	if streaming.request.TenantID != "tenant-default" || streaming.request.UserID != "user-default" {
		t.Fatalf("request identity = %q/%q, want authoritative defaults", streaming.request.TenantID, streaming.request.UserID)
	}
}

func TestHandlerRejectsInvalidTurnRequestBeforeStreamingHeaders(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: &stubStreamingOrchestrator{},
	})
	request := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(`{"conversation_id":"conv-1"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
}

func TestHandlerServesStaticAppAndAdminShells(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:   observability.NewMemoryMetrics(),
		StaticDir: "../../web",
	})

	tests := map[string][]string{
		"/app":   {"Professional Session Workspace", "conversation-panel", "presence-panel"},
		"/admin": {"Operations Console", "persona-admin", "audit-admin"},
	}
	for path, wants := range tests {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
				t.Fatalf("Content-Type = %q, want text/html", contentType)
			}
			for _, want := range wants {
				if !strings.Contains(response.Body.String(), want) {
					t.Fatalf("%s missing %q:\n%s", path, want, response.Body.String())
				}
			}
		})
	}
}

func TestHandlerServesEmptyFaviconWithoutBrowserConsole404(t *testing.T) {
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics()})
	request := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
}

func TestHandlerServesStaticWebAssets(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:   observability.NewMemoryMetrics(),
		StaticDir: "../../web",
	})

	tests := map[string][]string{
		"/web/app.css":  {"text/css"},
		"/web/app.js":   {"application/javascript", "text/javascript"},
		"/web/admin.js": {"application/javascript", "text/javascript"},
	}
	for path, wantContentTypes := range tests {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			contentType := response.Header().Get("Content-Type")
			if !containsAny(contentType, wantContentTypes) {
				t.Fatalf("Content-Type = %q, want one of %#v", contentType, wantContentTypes)
			}
		})
	}
}

func TestHandlerRejectsUnlistedStaticWebAssets(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:   observability.NewMemoryMetrics(),
		StaticDir: "../../web",
	})
	request := httptest.NewRequest(http.MethodGet, "/web/app_static_test.go", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", response.Code, response.Body.String())
	}
}

func containsAny(value string, wants []string) bool {
	for _, want := range wants {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func TestHandlerServesExperienceStreamWithPresentationEvents(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: stubOrchestrator{result: types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "experience ok"}}},
		PresentationAdapter: presentation.Adapter{
			TTS: voice.MockTTSClient{},
			Avatar: mustAvatarStateMachine(t, avatar.Manifest{
				Supported:     []avatar.State{avatar.StateIdle, avatar.StateSpeaking},
				FallbackState: avatar.StateIdle,
			}),
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/experience/stream", strings.NewReader(validChatJSON()))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", contentType)
	}
	body := response.Body.String()
	for _, want := range []string{"event: conversation_started", "event: assistant_text_delta", "event: subtitle", "event: audio_chunk", "event: avatar_state", "event: done"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerPrefersStreamingOrchestratorForExperienceStream(t *testing.T) {
	streaming := &stubStreamingOrchestrator{
		result: types.AgentResult{
			AgentName: "persona-agent",
			Message:   types.Message{ID: "msg-assistant-1", Role: types.RoleAssistant, Content: "experience streamed"},
		},
		events: []types.StreamEvent{
			{
				Name:           types.StreamEventRequestStarted,
				RequestID:      "req-1",
				TenantID:       "tenant-1",
				UserID:         "user-1",
				ConversationID: "conv-1",
				TurnID:         "msg-1",
				AttemptID:      "msg-1-attempt-1",
				Sequence:       1,
				Timestamp:      time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
			},
			{
				Name:           types.StreamEventAssistantDelta,
				RequestID:      "req-1",
				TenantID:       "tenant-1",
				UserID:         "user-1",
				ConversationID: "conv-1",
				TurnID:         "msg-1",
				AttemptID:      "msg-1-attempt-1",
				Sequence:       2,
				Timestamp:      time.Date(2026, 6, 27, 10, 0, 1, 0, time.UTC),
				Payload:        types.Metadata{"content": "experience streamed"},
			},
			{
				Name:           types.StreamEventMessageCompleted,
				RequestID:      "req-1",
				TenantID:       "tenant-1",
				UserID:         "user-1",
				ConversationID: "conv-1",
				TurnID:         "msg-1",
				AttemptID:      "msg-1-attempt-1",
				Sequence:       3,
				Timestamp:      time.Date(2026, 6, 27, 10, 0, 2, 0, time.UTC),
				Payload:        types.Metadata{"content": "experience streamed"},
			},
			{
				Name:           types.StreamEventDone,
				RequestID:      "req-1",
				TenantID:       "tenant-1",
				UserID:         "user-1",
				ConversationID: "conv-1",
				TurnID:         "msg-1",
				AttemptID:      "msg-1-attempt-1",
				Sequence:       4,
				Timestamp:      time.Date(2026, 6, 27, 10, 0, 3, 0, time.UTC),
				Payload:        types.Metadata{"status": "completed"},
			},
		},
	}
	handler := NewHandler(Config{
		Metrics:         observability.NewMemoryMetrics(),
		Orchestrator:    streaming,
		DefaultTenantID: "tenant-default",
		DefaultUserID:   "user-default",
		PresentationAdapter: presentation.Adapter{
			TTS: voice.MockTTSClient{},
			Avatar: mustAvatarStateMachine(t, avatar.Manifest{
				Supported:     []avatar.State{avatar.StateIdle, avatar.StateThinking, avatar.StateSpeaking},
				FallbackState: avatar.StateIdle,
			}),
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/experience/stream", strings.NewReader(validChatJSON()))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !streaming.streamCalled {
		t.Fatal("expected streaming orchestrator to be used")
	}
	if streaming.request.TenantID != "tenant-default" || streaming.request.UserID != "user-default" {
		t.Fatalf("request identity = %q/%q, want authoritative defaults", streaming.request.TenantID, streaming.request.UserID)
	}
	body := response.Body.String()
	for _, want := range []string{"event: conversation_started", "event: assistant_text_delta", "event: subtitle", "event: done"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerServesMockVoiceExperienceStream(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: stubOrchestrator{result: types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "voice ok"}}},
		ASR:          voice.MockASRClient{},
		PresentationAdapter: presentation.Adapter{
			TTS: voice.MockTTSClient{},
			Avatar: mustAvatarStateMachine(t, avatar.Manifest{
				Supported:     []avatar.State{avatar.StateIdle, avatar.StateSpeaking},
				FallbackState: avatar.StateIdle,
			}),
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/experience/mock-voice/stream", strings.NewReader(`{"audio_text":"hello by voice"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{"event: asr_final", `"text":"hello by voice"`, "event: assistant_text_delta", "event: audio_chunk", "event: done"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerPersonaAdminDraftPublishAndActive(t *testing.T) {
	personaService := admin.NewPersonaService(admin.NewInMemoryPersonaStore())
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		PersonaAdmin: &personaService,
	})
	draftBody := `{"id":"advisor","identity":"Ava","role":"professional digital advisor","tone":["calm","precise"],"boundaries":["state uncertainty when confidence is low"],"allowed_claims":["can explain planning tradeoffs"],"locale":"en-US"}`
	draftResponse := httptest.NewRecorder()

	handler.ServeHTTP(draftResponse, httptest.NewRequest(http.MethodPost, "/admin/persona/drafts", strings.NewReader(draftBody)))

	if draftResponse.Code != http.StatusOK {
		t.Fatalf("draft status = %d, body = %s", draftResponse.Code, draftResponse.Body.String())
	}
	var draft admin.PersonaVersion
	if err := json.NewDecoder(draftResponse.Body).Decode(&draft); err != nil {
		t.Fatalf("decode draft: %v", err)
	}

	publishResponse := httptest.NewRecorder()
	handler.ServeHTTP(publishResponse, httptest.NewRequest(http.MethodPost, "/admin/persona/publish", strings.NewReader(`{"version_id":"`+draft.ID+`"}`)))

	if publishResponse.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishResponse.Code, publishResponse.Body.String())
	}

	activeResponse := httptest.NewRecorder()
	handler.ServeHTTP(activeResponse, httptest.NewRequest(http.MethodGet, "/admin/persona/active", nil))

	if activeResponse.Code != http.StatusOK {
		t.Fatalf("active status = %d, body = %s", activeResponse.Code, activeResponse.Body.String())
	}
	if !strings.Contains(activeResponse.Body.String(), `"identity":"Ava"`) {
		t.Fatalf("active body = %s", activeResponse.Body.String())
	}
}

func TestHandlerPersonaActiveReturnsEmptyStateWhenUnpublished(t *testing.T) {
	personaService := admin.NewPersonaService(admin.NewInMemoryPersonaStore())
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		PersonaAdmin: &personaService,
	})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/persona/active", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"none"`) {
		t.Fatalf("body = %s, want empty status", response.Body.String())
	}
}

func TestHandlerPersonaAdminUnavailableDoesNotPanic(t *testing.T) {
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics()})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/persona/publish", strings.NewReader(`{"version_id":"missing"}`)))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", response.Code, response.Body.String())
	}
}

func TestHandlerMemoryAdminListsAndDisablesMemory(t *testing.T) {
	memoryService := admin.NewMemoryService(admin.NewInMemoryMemoryStore())
	if _, err := memoryService.Save("tenant-1", admin.MemoryRecord{ID: "mem-1", UserID: "user-1", Summary: "prefers concise plans", Status: admin.MemoryActive}); err != nil {
		t.Fatalf("Save memory: %v", err)
	}
	if _, err := memoryService.Save("tenant-1", admin.MemoryRecord{ID: "mem-2", UserID: "user-1", Summary: "disabled memory", Status: admin.MemoryDisabled}); err != nil {
		t.Fatalf("Save disabled memory: %v", err)
	}
	handler := NewHandler(Config{
		Metrics:     observability.NewMemoryMetrics(),
		MemoryAdmin: &memoryService,
	})

	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/admin/memory", nil))

	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), `"summary":"prefers concise plans"`) {
		t.Fatalf("list body = %s", listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), `"summary":"disabled memory"`) {
		t.Fatalf("list should include disabled records, body = %s", listResponse.Body.String())
	}

	disableResponse := httptest.NewRecorder()
	handler.ServeHTTP(disableResponse, httptest.NewRequest(http.MethodPost, "/admin/memory/disable", strings.NewReader(`{"memory_id":"mem-1"}`)))

	if disableResponse.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", disableResponse.Code, disableResponse.Body.String())
	}
	if !strings.Contains(disableResponse.Body.String(), `"status":"disabled"`) {
		t.Fatalf("disable body = %s", disableResponse.Body.String())
	}
}

func TestHandlerKnowledgeAdminUploadAndCitation(t *testing.T) {
	store := admin.NewInMemoryKnowledgeStore()
	knowledgeService := admin.NewKnowledgeService(store)
	knowledgeRetriever := knowledge.NewService(store)
	handler := NewHandler(Config{
		Metrics:            observability.NewMemoryMetrics(),
		KnowledgeAdmin:     &knowledgeService,
		KnowledgeRetriever: &knowledgeRetriever,
	})

	uploadResponse := httptest.NewRecorder()
	handler.ServeHTTP(uploadResponse, httptest.NewRequest(http.MethodPost, "/admin/knowledge/upload", strings.NewReader(`{"id":"kb-1","name":"planning.md","content":"Phase 4 adds a digital human UI."}`)))

	if uploadResponse.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", uploadResponse.Code, uploadResponse.Body.String())
	}
	if !strings.Contains(uploadResponse.Body.String(), `"chunks"`) {
		t.Fatalf("upload body = %s", uploadResponse.Body.String())
	}

	citationResponse := httptest.NewRecorder()
	handler.ServeHTTP(citationResponse, httptest.NewRequest(http.MethodPost, "/admin/knowledge/citation-test", strings.NewReader(`{"query":"digital human UI"}`)))

	if citationResponse.Code != http.StatusOK {
		t.Fatalf("citation status = %d, body = %s", citationResponse.Code, citationResponse.Body.String())
	}
	if !strings.Contains(citationResponse.Body.String(), `"document_id":"kb-1"`) {
		t.Fatalf("citation body = %s", citationResponse.Body.String())
	}
}

func TestHandlerKnowledgeRetrievalDiagnostics(t *testing.T) {
	store := admin.NewInMemoryKnowledgeStore()
	knowledgeAdmin := admin.NewKnowledgeService(store)
	if _, err := knowledgeAdmin.Upload("tenant-1", admin.KnowledgeUpload{
		ID:      "kb-1",
		Name:    "planning.md",
		Content: "Phase 11 adds retrieval diagnostics.\n\nGrounded answers stay auditable.",
	}); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	knowledgeRetriever := knowledge.NewService(store)
	handler := NewHandler(Config{
		Metrics:            observability.NewMemoryMetrics(),
		KnowledgeAdmin:     &knowledgeAdmin,
		KnowledgeRetriever: &knowledgeRetriever,
	})

	diagnosticsResponse := httptest.NewRecorder()
	handler.ServeHTTP(diagnosticsResponse, httptest.NewRequest(http.MethodPost, "/admin/knowledge/retrieval-diagnostics", strings.NewReader(`{"query":"retrieval diagnostics","mode":"auto","limit":2}`)))

	if diagnosticsResponse.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, body = %s", diagnosticsResponse.Code, diagnosticsResponse.Body.String())
	}
	body := diagnosticsResponse.Body.String()
	for _, want := range []string{
		`"mode":"auto"`,
		`"document_id":"kb-1"`,
		`"index_status":"vector_missing"`,
		`"stages_skipped":["vector_unavailable"]`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("diagnostics body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "data\\admin") {
		t.Fatalf("diagnostics body leaked local path:\n%s", body)
	}
}

func TestHandlerKnowledgeRetrievalDiagnosticsReturnsNoSourceReason(t *testing.T) {
	store := admin.NewInMemoryKnowledgeStore()
	knowledgeAdmin := admin.NewKnowledgeService(store)
	if _, err := knowledgeAdmin.Upload("tenant-1", admin.KnowledgeUpload{
		ID:      "kb-1",
		Name:    "planning.md",
		Content: "Phase 11 adds retrieval diagnostics.",
	}); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	knowledgeRetriever := knowledge.NewService(store)
	handler := NewHandler(Config{
		Metrics:            observability.NewMemoryMetrics(),
		KnowledgeAdmin:     &knowledgeAdmin,
		KnowledgeRetriever: &knowledgeRetriever,
	})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/knowledge/retrieval-diagnostics", strings.NewReader(`{"query":"quartz falcon runway","mode":"lexical","limit":2}`)))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"no_source_reason":"no_matching_chunks"`) {
		t.Fatalf("body = %s, want no_source_reason", response.Body.String())
	}
}

func TestHandlerKnowledgeAdminServesLifecycleEndpoints(t *testing.T) {
	knowledgeService := admin.NewKnowledgeService(admin.NewInMemoryKnowledgeStore())
	if _, err := knowledgeService.Upload("tenant-1", admin.KnowledgeUpload{
		ID:      "kb-ops",
		Name:    "ops.md",
		Content: "source grounding starts here.\n\nsecond chunk.",
	}); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	handler := NewHandler(Config{
		Metrics:        observability.NewMemoryMetrics(),
		KnowledgeAdmin: &knowledgeService,
	})

	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/admin/knowledge", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), `"id":"kb-ops"`) {
		t.Fatalf("list body = %s", listResponse.Body.String())
	}

	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/admin/knowledge/kb-ops", nil))
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}
	if !strings.Contains(getResponse.Body.String(), `"chunk_count":2`) {
		t.Fatalf("get body = %s", getResponse.Body.String())
	}

	disableResponse := httptest.NewRecorder()
	handler.ServeHTTP(disableResponse, httptest.NewRequest(http.MethodPost, "/admin/knowledge/disable", strings.NewReader(`{"document_id":"kb-ops"}`)))
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", disableResponse.Code, disableResponse.Body.String())
	}
	if !strings.Contains(disableResponse.Body.String(), `"status":"disabled"`) {
		t.Fatalf("disable body = %s", disableResponse.Body.String())
	}

	enableResponse := httptest.NewRecorder()
	handler.ServeHTTP(enableResponse, httptest.NewRequest(http.MethodPost, "/admin/knowledge/enable", strings.NewReader(`{"document_id":"kb-ops"}`)))
	if enableResponse.Code != http.StatusOK {
		t.Fatalf("enable status = %d, body = %s", enableResponse.Code, enableResponse.Body.String())
	}

	reindexResponse := httptest.NewRecorder()
	handler.ServeHTTP(reindexResponse, httptest.NewRequest(http.MethodPost, "/admin/knowledge/reindex", strings.NewReader(`{"document_id":"kb-ops","content":"reindexed source material only"}`)))
	if reindexResponse.Code != http.StatusOK {
		t.Fatalf("reindex status = %d, body = %s", reindexResponse.Code, reindexResponse.Body.String())
	}
	if !strings.Contains(reindexResponse.Body.String(), `"chunk_count":1`) {
		t.Fatalf("reindex body = %s", reindexResponse.Body.String())
	}

	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodPost, "/admin/knowledge/delete", strings.NewReader(`{"document_id":"kb-ops"}`)))
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}

	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, httptest.NewRequest(http.MethodGet, "/admin/knowledge/kb-ops", nil))
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, want 404; body = %s", missingResponse.Code, missingResponse.Body.String())
	}
}

func TestHandlerToolPolicyAdminSavesAndAuthorizesTools(t *testing.T) {
	toolPolicyService := admin.NewToolPolicyService(admin.NewInMemoryToolPolicyStore())
	handler := NewHandler(Config{
		Metrics:         observability.NewMemoryMetrics(),
		ToolPolicyAdmin: &toolPolicyService,
	})
	saveResponse := httptest.NewRecorder()
	handler.ServeHTTP(saveResponse, httptest.NewRequest(http.MethodPost, "/admin/tools/policy", strings.NewReader(`{"persona_id":"advisor","allowed_tools":["knowledge.search"],"approval_mode":"manual"}`)))

	if saveResponse.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", saveResponse.Code, saveResponse.Body.String())
	}

	allowResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowResponse, httptest.NewRequest(http.MethodPost, "/admin/tools/authorize", strings.NewReader(`{"persona_id":"advisor","tool_name":"knowledge.search"}`)))
	if allowResponse.Code != http.StatusOK {
		t.Fatalf("allow status = %d, body = %s", allowResponse.Code, allowResponse.Body.String())
	}

	denyResponse := httptest.NewRecorder()
	handler.ServeHTTP(denyResponse, httptest.NewRequest(http.MethodPost, "/admin/tools/authorize", strings.NewReader(`{"persona_id":"advisor","tool_name":"http.call"}`)))
	if denyResponse.Code != http.StatusForbidden {
		t.Fatalf("deny status = %d, want 403; body = %s", denyResponse.Code, denyResponse.Body.String())
	}
}

func TestHandlerRecordsExperienceStreamAudit(t *testing.T) {
	auditService := admin.NewAuditService(admin.NewInMemoryAuditStore())
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: stubOrchestrator{result: types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "audit ok"}}},
		AuditAdmin:   &auditService,
		PresentationAdapter: presentation.Adapter{
			TTS: voice.MockTTSClient{},
			Avatar: mustAvatarStateMachine(t, avatar.Manifest{
				Supported:     []avatar.State{avatar.StateIdle, avatar.StateSpeaking},
				FallbackState: avatar.StateIdle,
			}),
		},
	})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/experience/stream", strings.NewReader(validChatJSON())))

	auditResponse := httptest.NewRecorder()
	handler.ServeHTTP(auditResponse, httptest.NewRequest(http.MethodGet, "/admin/audit", nil))

	if auditResponse.Code != http.StatusOK {
		t.Fatalf("audit status = %d, body = %s", auditResponse.Code, auditResponse.Body.String())
	}
	if !strings.Contains(auditResponse.Body.String(), `"conversation_id":"conv-1"`) || !strings.Contains(auditResponse.Body.String(), `"agent_name":"persona-agent"`) {
		t.Fatalf("audit body = %s", auditResponse.Body.String())
	}
}

func TestHandlerRecordsMockVoiceAudit(t *testing.T) {
	auditService := admin.NewAuditService(admin.NewInMemoryAuditStore())
	handler := NewHandler(Config{
		Metrics:      observability.NewMemoryMetrics(),
		Orchestrator: stubOrchestrator{result: types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "voice audit"}}},
		AuditAdmin:   &auditService,
		ASR:          voice.MockASRClient{},
		PresentationAdapter: presentation.Adapter{
			TTS: voice.MockTTSClient{},
			Avatar: mustAvatarStateMachine(t, avatar.Manifest{
				Supported:     []avatar.State{avatar.StateIdle, avatar.StateSpeaking},
				FallbackState: avatar.StateIdle,
			}),
		},
	})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/experience/mock-voice/stream", strings.NewReader(`{"audio_text":"hello by voice"}`)))

	auditResponse := httptest.NewRecorder()
	handler.ServeHTTP(auditResponse, httptest.NewRequest(http.MethodGet, "/admin/audit", nil))

	if auditResponse.Code != http.StatusOK {
		t.Fatalf("audit status = %d, body = %s", auditResponse.Code, auditResponse.Body.String())
	}
	if !strings.Contains(auditResponse.Body.String(), `"conversation_id":"mock-voice-session"`) {
		t.Fatalf("audit body = %s", auditResponse.Body.String())
	}
}

func mustAvatarStateMachine(t *testing.T, manifest avatar.Manifest) avatar.StateMachine {
	t.Helper()

	machine, err := avatar.NewStateMachine(manifest)
	if err != nil {
		t.Fatalf("NewStateMachine returned error: %v", err)
	}
	return machine
}

func TestHandlerStreamsRecordedRuntimeEvents(t *testing.T) {
	recorder := runtime.NewEventRecorder()
	handler := NewHandler(Config{
		Metrics:       observability.NewMemoryMetrics(),
		Orchestrator:  recordingOrchestrator{recorder: recorder},
		EventRecorder: recorder,
	})
	request := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(validChatJSON()))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	body := response.Body.String()
	for _, want := range []string{"event: request_started", "event: route_selected", `"intent":"persona.chat"`, "event: message_completed", "event: done"} {
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

type stubStreamingOrchestrator struct {
	result       types.AgentResult
	err          error
	events       []types.StreamEvent
	request      types.TurnRequest
	streamCalled bool
	handleCalled bool
}

func (s *stubStreamingOrchestrator) Handle(_ context.Context, _ types.Conversation) (types.AgentResult, error) {
	s.handleCalled = true
	return s.result, s.err
}

func (s *stubStreamingOrchestrator) Stream(ctx context.Context, request types.TurnRequest, sink core.StreamSink) (types.AgentResult, error) {
	s.streamCalled = true
	s.request = request
	for _, event := range s.events {
		if err := sink.Emit(ctx, event); err != nil {
			return types.AgentResult{}, err
		}
	}
	return s.result, s.err
}

type recordingOrchestrator struct {
	recorder *runtime.EventRecorder
}

func (r recordingOrchestrator) Handle(_ context.Context, conversation types.Conversation) (types.AgentResult, error) {
	r.recorder.Record(runtime.NewRuntimeEvent(runtime.EventRequestStarted, "req-1", conversation, nil))
	r.recorder.Record(runtime.NewRuntimeEvent(runtime.EventRouteSelected, "req-1", conversation, types.Metadata{"intent": "persona.chat"}))
	return types.AgentResult{
		AgentName:  "persona-agent",
		Message:    types.Message{Role: types.RoleAssistant, Content: "streamed ok"},
		Confidence: 0.8,
	}, nil
}
