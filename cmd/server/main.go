package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/nobodycan/digital-twin/internal/app"
	"github.com/nobodycan/digital-twin/internal/config"
	"github.com/nobodycan/digital-twin/internal/observability"
	"github.com/nobodycan/digital-twin/internal/server"
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

func buildHandler(cfg config.AppConfig) (http.Handler, error) {
	local, err := app.NewLocalRuntime(app.LocalRuntimeConfig{})
	if err != nil {
		return nil, err
	}
	apiKeys := []string(nil)
	if cfg.Server.APIKey != "" {
		apiKeys = []string{cfg.Server.APIKey}
	}
	return server.NewHandler(server.Config{
		Metrics:           observability.NewMemoryMetrics(),
		Orchestrator:      local.Orchestrator,
		EventRecorder:     local.Recorder,
		APIKeys:           apiKeys,
		RateLimitRequests: cfg.Server.RateLimitRequests,
	}), nil
}

func defaultConfigPath() string {
	if path := os.Getenv("DIGITAL_TWIN_CONFIG"); path != "" {
		return path
	}
	return "configs/app.yaml"
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
