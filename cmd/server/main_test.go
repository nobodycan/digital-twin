package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	_, err := buildHandler(config.AppConfig{TTS: config.ProviderConfig{Provider: "unsupported"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported tts provider") {
		t.Fatalf("buildHandler() error = %v, want unsupported tts provider", err)
	}
}

func TestBuildHandlerAppliesServerAuthConfig(t *testing.T) {
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

func validServerChatJSON() string {
	return `{"id":"conv-e2e","tenant_id":"tenant-1","user_id":"user-1","messages":[{"id":"msg-1","role":"user","content":"hello","created_at":"2026-06-16T12:00:00Z"}],"created_at":"2026-06-16T12:00:00Z","updated_at":"2026-06-16T12:00:00Z"}`
}
