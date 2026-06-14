package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nobodycan/digital-twin/internal/config"
	"github.com/nobodycan/digital-twin/internal/observability"
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
