package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"strings"
	"testing"

	"github.com/nobodycan/digital-twin/internal/config"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestDefaultConfigPathUsesEnvironment(t *testing.T) {
	t.Setenv("DIGITAL_TWIN_CONFIG", "custom.yaml")

	if got := defaultConfigPath(); got != "custom.yaml" {
		t.Fatalf("defaultConfigPath() = %q, want custom.yaml", got)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"unknown": slog.LevelInfo,
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := parseLogLevel(input); got != want {
				t.Fatalf("parseLogLevel(%q) = %v, want %v", input, got, want)
			}
		})
	}
}

func TestStartupSummaryDoesNotLeakSecrets(t *testing.T) {
	cfg := config.AppConfig{
		Environment: "production",
		Server:      config.ServerConfig{Port: 8080, APIKey: "server-secret"},
		Log:         config.LogConfig{Level: "info"},
		LLM:         config.LLMConfig{APIKey: "llm-secret"},
		TTS:         config.ProviderConfig{Provider: "http", BaseURL: "https://tts.example.test", APIKey: "tts-secret"},
		ASR:         config.ProviderConfig{Provider: "local", APIKey: "asr-secret"},
		Tenant:      config.TenantConfig{DefaultID: "tenant-a"},
	}

	summary := startupSummary(cfg)

	for _, secret := range []string{"server-secret", "llm-secret", "tts-secret", "asr-secret"} {
		if strings.Contains(summary, secret) {
			t.Fatalf("startupSummary() leaked %q in %q", secret, summary)
		}
	}
	if !strings.Contains(summary, "environment=production") || !strings.Contains(summary, "tts.api_key=<redacted>") {
		t.Fatalf("startupSummary() = %q, want safe config summary", summary)
	}
}

func TestBuildHandlerServesHealth(t *testing.T) {
	isolateRuntimeData(t)
	handler, err := buildHandler(config.AppConfig{})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
}

func TestBuildHandlerRejectsInvalidAdminDataDir(t *testing.T) {
	isolateRuntimeData(t)
	filePath := filepath.Join(t.TempDir(), "admin-data-file")
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write admin data file: %v", err)
	}
	t.Setenv("DIGITAL_TWIN_ADMIN_DATA", filePath)
	_, err := buildHandler(config.AppConfig{})
	if err == nil || !strings.Contains(err.Error(), "create admin data dir") {
		t.Fatalf("buildHandler() error = %v, want create admin data dir", err)
	}
}

func TestBuildHandlerCreatesMissingAdminDataDirForReadiness(t *testing.T) {
	isolateRuntimeData(t)
	dataDir := filepath.Join(t.TempDir(), "admin-data")
	t.Setenv("DIGITAL_TWIN_ADMIN_DATA", dataDir)
	handler, err := buildHandler(config.AppConfig{})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}
	info, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf("stat admin data dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("admin data path is not a directory")
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}

func TestBuildHandlerRejectsUnsupportedTTSProvider(t *testing.T) {
	isolateRuntimeData(t)
	_, err := buildHandler(config.AppConfig{TTS: config.ProviderConfig{Provider: "unsupported"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported tts provider") {
		t.Fatalf("buildHandler() error = %v, want unsupported tts provider", err)
	}
}

func TestBuildHandlerAppliesServerAuthConfig(t *testing.T) {
	isolateRuntimeData(t)
	handler, err := buildHandler(config.AppConfig{Server: config.ServerConfig{APIKey: "secret"}})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{}`))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestBuildHandlerServesLocalChatEndToEnd(t *testing.T) {
	isolateRuntimeData(t)
	handler, err := buildHandler(config.AppConfig{})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(validServerChatJSON()))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var result types.AgentResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.AgentName != "persona-agent" || result.Message.Role != types.RoleAssistant {
		t.Fatalf("result = %#v, want local persona response", result)
	}
}

func TestBuildHandlerServesConfiguredLLMChatEndToEnd(t *testing.T) {
	isolateRuntimeData(t)
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "I think this response came from the configured LLM.",
				},
			}},
			"usage": map[string]any{
				"prompt_tokens":     3,
				"completion_tokens": 4,
				"total_tokens":      7,
			},
		})
	}))
	defer llmServer.Close()

	handler, err := buildHandler(config.AppConfig{
		LLM: config.LLMConfig{
			Provider:       "openai-compatible",
			BaseURL:        llmServer.URL,
			Model:          "gpt-server",
			FallbackPolicy: "fallback_to_local",
			APIKey:         "llm-secret",
		},
	})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(validServerChatJSON()))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var result types.AgentResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Message.Content != "I think this response came from the configured LLM." {
		t.Fatalf("result message = %q, want configured llm response", result.Message.Content)
	}
	if result.Metadata["llm_model"] != "gpt-server" {
		t.Fatalf("llm_model = %v, want gpt-server", result.Metadata["llm_model"])
	}
}

func TestBuildHandlerServesExperienceStreamWithDefaultPresentationAdapter(t *testing.T) {
	isolateRuntimeData(t)
	handler, err := buildHandler(config.AppConfig{})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/experience/stream", strings.NewReader(validServerChatJSON()))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{"event: assistant_text_delta", "event: audio_chunk", `"state":"speaking"`, "event: done"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestBuildHandlerExposesHTTPProviderMetrics(t *testing.T) {
	isolateRuntimeData(t)
	ttsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"provider": "http",
			"chunks": []map[string]any{{
				"index":     0,
				"mime_type": "audio/mpeg",
				"data":      []byte("audio"),
			}},
		})
	}))
	defer ttsServer.Close()
	t.Setenv("DIGITAL_TWIN_ADMIN_DATA", t.TempDir())
	handler, err := buildHandler(config.AppConfig{TTS: config.ProviderConfig{Provider: "http", BaseURL: ttsServer.URL, APIKey: "tts-secret"}})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}

	experience := httptest.NewRecorder()
	handler.ServeHTTP(experience, httptest.NewRequest(http.MethodPost, "/experience/stream", strings.NewReader(validServerChatJSON())))
	if experience.Code != http.StatusOK {
		t.Fatalf("experience status = %d, body = %s", experience.Code, experience.Body.String())
	}

	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := metrics.Body.String()
	if !strings.Contains(body, `voice_provider_latency_ms`) {
		t.Fatalf("metrics body missing provider latency:\n%s", body)
	}
	if strings.Contains(body, "tts-secret") {
		t.Fatalf("metrics leaked TTS secret:\n%s", body)
	}
}

func TestBuildHandlerServesStaticAppShell(t *testing.T) {
	isolateRuntimeData(t)
	handler, err := buildHandler(config.AppConfig{})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Digital Human Console") {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestBuildHandlerServesPersonaAdminDraft(t *testing.T) {
	t.Setenv("DIGITAL_TWIN_ADMIN_DATA", t.TempDir())
	isolateRuntimeData(t)
	handler, err := buildHandler(config.AppConfig{})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}
	response := httptest.NewRecorder()
	body := `{"id":"advisor","identity":"Ava","role":"professional digital advisor","tone":["calm","precise"]}`
	request := httptest.NewRequest(http.MethodPost, "/admin/persona/drafts", strings.NewReader(body))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"draft"`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestBuildHandlerPersistsTenTurnStreamingConversation(t *testing.T) {
	isolateRuntimeData(t)
	runtimeDataDir := os.Getenv("DIGITAL_TWIN_RUNTIME_DATA")
	var mu sync.Mutex
	requests := make([]openAIChatRequest, 0, 10)
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var request openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode LLM request: %v", err)
		}
		mu.Lock()
		requests = append(requests, request)
		index := len(requests)
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"reply ` + fmt.Sprintf(`%d`, index) + `."}}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer llmServer.Close()

	handler, err := buildHandler(config.AppConfig{
		LLM: config.LLMConfig{
			Provider:       "openai-compatible",
			BaseURL:        llmServer.URL,
			Model:          "gpt-server",
			FallbackPolicy: "fallback_to_local",
			APIKey:         "llm-secret",
		},
	})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}

	for i := 1; i <= 10; i++ {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(validTurnRequestJSON("conv-e2e", fmt.Sprintf("turn-%d", i), "attempt-1", fmt.Sprintf("msg-%d", i), fmt.Sprintf("question %d", i))))
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("turn %d status = %d, body = %s", i, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "event: message_completed") {
			t.Fatalf("turn %d body missing completion:\n%s", i, response.Body.String())
		}
	}

	mu.Lock()
	last := requests[len(requests)-1]
	mu.Unlock()
	if len(last.Messages) < 5 {
		t.Fatalf("last LLM request messages = %d, want history included", len(last.Messages))
	}

	data, err := os.ReadFile(filepath.Join(runtimeDataDir, "tenants", "tenant-1", "users", "user-1", "conversations", "conv-e2e.json"))
	if err != nil {
		t.Fatalf("ReadFile conversation: %v", err)
	}
	var conversation types.Conversation
	if err := json.Unmarshal(data, &conversation); err != nil {
		t.Fatalf("Unmarshal conversation: %v", err)
	}
	if len(conversation.Turns) != 10 {
		t.Fatalf("turns len = %d, want 10", len(conversation.Turns))
	}
	if len(conversation.Messages) != 20 {
		t.Fatalf("messages len = %d, want 20", len(conversation.Messages))
	}
}

func TestBuildHandlerRestartContinuesConversationHistory(t *testing.T) {
	isolateRuntimeData(t)
	runtimeDataDir := os.Getenv("DIGITAL_TWIN_RUNTIME_DATA")
	var mu sync.Mutex
	requests := make([]openAIChatRequest, 0, 2)
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode LLM request: %v", err)
		}
		mu.Lock()
		requests = append(requests, request)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"ok."}}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer llmServer.Close()

	cfg := config.AppConfig{
		LLM: config.LLMConfig{
			Provider:       "openai-compatible",
			BaseURL:        llmServer.URL,
			Model:          "gpt-server",
			FallbackPolicy: "fallback_to_local",
			APIKey:         "llm-secret",
		},
	}
	first, err := buildHandler(cfg)
	if err != nil {
		t.Fatalf("first buildHandler() error = %v", err)
	}
	firstResponse := httptest.NewRecorder()
	first.ServeHTTP(firstResponse, httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(validTurnRequestJSON("conv-restart", "turn-1", "attempt-1", "msg-1", "hello one"))))
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", firstResponse.Code, firstResponse.Body.String())
	}

	second, err := buildHandler(cfg)
	if err != nil {
		t.Fatalf("second buildHandler() error = %v", err)
	}
	secondResponse := httptest.NewRecorder()
	second.ServeHTTP(secondResponse, httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(validTurnRequestJSON("conv-restart", "turn-2", "attempt-1", "msg-2", "hello two"))))
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", secondResponse.Code, secondResponse.Body.String())
	}

	mu.Lock()
	last := requests[len(requests)-1]
	mu.Unlock()
	if len(last.Messages) < 3 {
		t.Fatalf("restart request messages = %d, want prior turn context", len(last.Messages))
	}

	data, err := os.ReadFile(filepath.Join(runtimeDataDir, "tenants", "tenant-1", "users", "user-1", "conversations", "conv-restart.json"))
	if err != nil {
		t.Fatalf("ReadFile conversation: %v", err)
	}
	var conversation types.Conversation
	if err := json.Unmarshal(data, &conversation); err != nil {
		t.Fatalf("Unmarshal conversation: %v", err)
	}
	if len(conversation.Turns) != 2 {
		t.Fatalf("turns len = %d, want 2", len(conversation.Turns))
	}
}

func TestBuildHandlerCancellationDoesNotCommitAssistantMessage(t *testing.T) {
	isolateRuntimeData(t)
	runtimeDataDir := os.Getenv("DIGITAL_TWIN_RUNTIME_DATA")
	firstDelta := make(chan struct{}, 1)
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"partial"}}]}` + "\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		firstDelta <- struct{}{}
		<-r.Context().Done()
	}))
	defer llmServer.Close()

	handler, err := buildHandler(config.AppConfig{
		LLM: config.LLMConfig{
			Provider:       "openai-compatible",
			BaseURL:        llmServer.URL,
			Model:          "gpt-server",
			FallbackPolicy: "fallback_to_local",
			APIKey:         "llm-secret",
		},
	})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(validTurnRequestJSON("conv-cancel", "turn-1", "attempt-1", "msg-1", "cancel me"))).WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()
	<-firstDelta
	cancel()
	<-done

	data, err := os.ReadFile(filepath.Join(runtimeDataDir, "tenants", "tenant-1", "users", "user-1", "conversations", "conv-cancel.json"))
	if err != nil {
		t.Fatalf("ReadFile conversation: %v", err)
	}
	var conversation types.Conversation
	if err := json.Unmarshal(data, &conversation); err != nil {
		t.Fatalf("Unmarshal conversation: %v", err)
	}
	if len(conversation.Messages) != 1 {
		t.Fatalf("messages len = %d, want only user message", len(conversation.Messages))
	}
	if conversation.Turns[0].Status != types.TurnCanceled {
		t.Fatalf("turn status = %q, want canceled", conversation.Turns[0].Status)
	}
	if conversation.Turns[0].AssistantMessageID != "" {
		t.Fatalf("assistant message id = %q, want empty", conversation.Turns[0].AssistantMessageID)
	}
}

func TestBuildHandlerRetryCompletesOneUserOneAssistantPair(t *testing.T) {
	isolateRuntimeData(t)
	runtimeDataDir := os.Getenv("DIGITAL_TWIN_RUNTIME_DATA")
	var (
		mu      sync.Mutex
		callNum int
	)
	firstDelta := make(chan struct{}, 1)
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callNum++
		current := callNum
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		if current == 1 {
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"partial"}}]}` + "\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			firstDelta <- struct{}{}
			<-r.Context().Done()
			return
		}
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"recovered answer."}}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer llmServer.Close()

	handler, err := buildHandler(config.AppConfig{
		LLM: config.LLMConfig{
			Provider:       "openai-compatible",
			BaseURL:        llmServer.URL,
			Model:          "gpt-server",
			FallbackPolicy: "fallback_to_local",
			APIKey:         "llm-secret",
		},
	})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	firstRequest := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(validTurnRequestJSON("conv-retry", "turn-1", "attempt-1", "msg-1", "retry me"))).WithContext(ctx)
	firstResponse := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(firstResponse, firstRequest)
		close(done)
	}()
	<-firstDelta
	cancel()
	<-done

	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(validTurnRequestJSON("conv-retry", "turn-1", "attempt-2", "msg-1", "retry me"))))
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("retry status = %d, body = %s", secondResponse.Code, secondResponse.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(runtimeDataDir, "tenants", "tenant-1", "users", "user-1", "conversations", "conv-retry.json"))
	if err != nil {
		t.Fatalf("ReadFile conversation: %v", err)
	}
	var conversation types.Conversation
	if err := json.Unmarshal(data, &conversation); err != nil {
		t.Fatalf("Unmarshal conversation: %v", err)
	}
	if len(conversation.Messages) != 2 {
		t.Fatalf("messages len = %d, want one user and one assistant", len(conversation.Messages))
	}
	if len(conversation.Turns) != 1 {
		t.Fatalf("turns len = %d, want 1", len(conversation.Turns))
	}
	if len(conversation.Turns[0].Attempts) != 2 {
		t.Fatalf("attempts len = %d, want 2", len(conversation.Turns[0].Attempts))
	}
	if conversation.Turns[0].Status != types.TurnCompleted {
		t.Fatalf("turn status = %q, want completed", conversation.Turns[0].Status)
	}
}

func TestBuildHandlerUsesAuthoritativeConversationIdentity(t *testing.T) {
	isolateRuntimeData(t)
	runtimeDataDir := os.Getenv("DIGITAL_TWIN_RUNTIME_DATA")
	handler, err := buildHandler(config.AppConfig{
		Tenant: config.TenantConfig{
			DefaultID:     "tenant-default",
			DefaultUserID: "user-default",
		},
	})
	if err != nil {
		t.Fatalf("buildHandler() error = %v", err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(`{"conversation_id":"conv-auth","tenant_id":"tenant-attacker","user_id":"user-attacker","turn_id":"turn-1","attempt_id":"attempt-1","message":{"id":"msg-1","role":"user","content":"hello","created_at":"2026-06-16T12:00:00Z"}}`))
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	defaultPath := filepath.Join(runtimeDataDir, "tenants", "tenant-default", "users", "user-default", "conversations", "conv-auth.json")
	if _, err := os.Stat(defaultPath); err != nil {
		t.Fatalf("stat default conversation path: %v", err)
	}
	attackerPath := filepath.Join(runtimeDataDir, "tenants", "tenant-attacker", "users", "user-attacker", "conversations", "conv-auth.json")
	if _, err := os.Stat(attackerPath); !os.IsNotExist(err) {
		t.Fatalf("attacker conversation path should not exist, stat err = %v", err)
	}
}

func validServerChatJSON() string {
	return `{"id":"conv-e2e","tenant_id":"tenant-1","user_id":"user-1","messages":[{"id":"msg-1","role":"user","content":"hello","created_at":"2026-06-16T12:00:00Z"}],"created_at":"2026-06-16T12:00:00Z","updated_at":"2026-06-16T12:00:00Z"}`
}

func isolateRuntimeData(t *testing.T) {
	t.Helper()
	t.Setenv("DIGITAL_TWIN_RUNTIME_DATA", t.TempDir())
}

func validTurnRequestJSON(conversationID, turnID, attemptID, messageID, content string) string {
	return fmt.Sprintf(`{"conversation_id":"%s","tenant_id":"tenant-1","user_id":"user-1","turn_id":"%s","attempt_id":"%s","message":{"id":"%s","role":"user","content":"%s","created_at":"2026-06-16T12:00:00Z"}}`,
		conversationID, turnID, attemptID, messageID, content)
}

type openAIChatRequest struct {
	Model       string `json:"model"`
	Messages    []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Temperature float64 `json:"temperature,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
}
