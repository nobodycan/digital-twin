package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/avatar"
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

func TestHandlerServesStaticAppAndAdminShells(t *testing.T) {
	handler := NewHandler(Config{
		Metrics:   observability.NewMemoryMetrics(),
		StaticDir: "../../web",
	})

	tests := map[string][]string{
		"/app":   {"Digital Human Console", "conversation-panel", "avatar-stage"},
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
	knowledgeService := admin.NewKnowledgeService(admin.NewInMemoryKnowledgeStore())
	handler := NewHandler(Config{
		Metrics:        observability.NewMemoryMetrics(),
		KnowledgeAdmin: &knowledgeService,
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
