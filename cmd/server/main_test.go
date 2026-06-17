package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func validServerChatJSON() string {
	return `{"id":"conv-e2e","tenant_id":"tenant-1","user_id":"user-1","messages":[{"id":"msg-1","role":"user","content":"hello","created_at":"2026-06-16T12:00:00Z"}],"created_at":"2026-06-16T12:00:00Z","updated_at":"2026-06-16T12:00:00Z"}`
}
