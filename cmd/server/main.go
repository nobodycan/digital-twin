package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/app"
	"github.com/nobodycan/digital-twin/internal/avatar"
	"github.com/nobodycan/digital-twin/internal/config"
	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/observability"
	"github.com/nobodycan/digital-twin/internal/presentation"
	"github.com/nobodycan/digital-twin/internal/server"
	"github.com/nobodycan/digital-twin/internal/voice"
	"log/slog"
)

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to app configuration file")
	flag.Parse()

	ctx := context.Background()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := observability.NewJSONLogger(os.Stdout, parseLogLevel(cfg.Log.Level))
	logger.Info(ctx, "digital-twin server starting",
		slog.String("component", "server"),
		slog.Int("port", cfg.Server.Port),
		slog.String("log_level", cfg.Log.Level),
		slog.String("tenant_default_id", cfg.Tenant.DefaultID),
		slog.String("tts_provider", cfg.TTS.Provider),
		slog.String("asr_provider", cfg.ASR.Provider),
		slog.String("config_summary", startupSummary(cfg)),
	)
	handler, err := buildHandler(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build handler: %v\n", err)
		os.Exit(1)
	}
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	if err := http.ListenAndServe(addr, handler); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}

func startupSummary(cfg config.AppConfig) string {
	return cfg.SafeSummary()
}

func buildHandler(cfg config.AppConfig) (http.Handler, error) {
	personaLLM, err := llm.NewClientFromConfig(cfg.LLM)
	if err != nil {
		return nil, err
	}
	providerLabel := runtimeStatusProvider(cfg.LLM.Provider, cfg.LLM.BaseURL)
	local, err := app.NewLocalRuntime(app.LocalRuntimeConfig{
		PersonaLLM:               personaLLM,
		PersonaLLMProvider:       providerLabel,
		PersonaLLMModel:          cfg.LLM.Model,
		PersonaLLMFallbackPolicy: cfg.LLM.FallbackPolicy,
		DataDir:                  defaultRuntimeDataDir(),
	})
	if err != nil {
		return nil, err
	}
	apiKeys := []string(nil)
	if cfg.Server.APIKey != "" {
		apiKeys = []string{cfg.Server.APIKey}
	}
	metrics := observability.NewMemoryMetrics()
	avatarMachine, err := avatar.NewStateMachine(avatar.Manifest{
		Supported: []avatar.State{
			avatar.StateIdle,
			avatar.StateListening,
			avatar.StateThinking,
			avatar.StateSpeaking,
			avatar.StateError,
			avatar.StateInterrupted,
		},
		FallbackState: avatar.StateIdle,
	})
	if err != nil {
		return nil, err
	}
	ttsClient, err := voice.NewTTSClientWithMetrics(cfg.TTS, metrics)
	if err != nil {
		return nil, err
	}
	adminDataDir := defaultAdminDataDir()
	if err := os.MkdirAll(adminDataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create admin data dir: %w", err)
	}
	personaAdmin := admin.NewPersonaService(admin.NewFilePersonaStore(adminDataDir))
	memoryAdmin := admin.NewMemoryService(admin.NewFileMemoryStore(adminDataDir))
	knowledgeAdmin := admin.NewKnowledgeService(admin.NewFileKnowledgeStore(adminDataDir))
	toolPolicyAdmin := admin.NewToolPolicyService(admin.NewFileToolPolicyStore(adminDataDir))
	auditAdmin := admin.NewAuditService(admin.NewFileAuditStore(adminDataDir))
	return server.NewHandler(server.Config{
		Metrics:       metrics,
		Orchestrator:  local.Orchestrator,
		EventRecorder: local.Recorder,
		PresentationAdapter: presentation.Adapter{
			TTS:    ttsClient,
			Avatar: avatarMachine,
		},
		Readiness: server.ReadinessConfig{
			DataDir:           adminDataDir,
			ConfigSummary:     cfg.SafeSummary(),
			ReleaseGateStatus: "skipped",
			Redact:            cfg.RedactSecrets,
		},
		RuntimeStatus: server.RuntimeStatus{
			Environment:        cfg.Environment,
			Provider:           providerLabel,
			Model:              cfg.LLM.Model,
			FallbackPolicy:     cfg.LLM.FallbackPolicy,
			GenerationModeHint: runtimeStatusModeHint(cfg.LLM.Provider),
			BaseURL:            config.SafeURLSummary(cfg.LLM.BaseURL),
		},
		PersonaAdmin:      &personaAdmin,
		MemoryAdmin:       &memoryAdmin,
		KnowledgeAdmin:    &knowledgeAdmin,
		ToolPolicyAdmin:   &toolPolicyAdmin,
		AuditAdmin:        &auditAdmin,
		StaticDir:         defaultStaticDir(),
		APIKeys:           apiKeys,
		RateLimitRequests: cfg.Server.RateLimitRequests,
		DefaultTenantID:   cfg.Tenant.DefaultID,
		DefaultUserID:     cfg.Tenant.DefaultUserID,
	}), nil
}

func defaultConfigPath() string {
	if path := os.Getenv("DIGITAL_TWIN_CONFIG"); path != "" {
		return path
	}
	return "configs/app.yaml"
}

func defaultStaticDir() string {
	for _, path := range []string{"web", filepath.Join("..", "..", "web")} {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return "web"
}

func defaultAdminDataDir() string {
	if path := os.Getenv("DIGITAL_TWIN_ADMIN_DATA"); path != "" {
		return path
	}
	return filepath.Join("data", "admin")
}

func defaultRuntimeDataDir() string {
	if path := os.Getenv("DIGITAL_TWIN_RUNTIME_DATA"); path != "" {
		return path
	}
	return filepath.Join("data", "runtime")
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func runtimeStatusModeHint(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "local", "mock":
		return "local"
	default:
		return "llm"
	}
}

func runtimeStatusProvider(provider, rawBaseURL string) string {
	if hostProvider := providerNameFromBaseURL(rawBaseURL); hostProvider != "" {
		return hostProvider
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "local", "mock":
		return "local"
	default:
		return strings.TrimSpace(provider)
	}
}

func providerNameFromBaseURL(rawBaseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case strings.Contains(host, "deepseek"):
		return "deepseek"
	case strings.Contains(host, "openai"):
		return "openai"
	default:
		return ""
	}
}
