package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nobodycan/digital-twin/internal/app"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: digital-twin ask <prompt>")
		return 2
	}
	switch args[0] {
	case "ask":
		return runAsk(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func runAsk(args []string, stdout, stderr io.Writer) int {
	jsonOutput := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		filtered = append(filtered, arg)
	}
	prompt := strings.TrimSpace(strings.Join(filtered, " "))
	if prompt == "" {
		_, _ = fmt.Fprintln(stderr, "prompt is required")
		return 2
	}
	local, err := app.NewLocalRuntime(app.LocalRuntimeConfig{})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bootstrap runtime: %v\n", err)
		return 1
	}
	now := time.Now().UTC()
	result, err := local.Orchestrator.Handle(context.Background(), types.Conversation{
		ID:        "cli-conversation",
		TenantID:  "local",
		UserID:    "cli",
		CreatedAt: now,
		UpdatedAt: now,
		Messages: []types.Message{{
			ID:        "cli-message",
			Role:      types.RoleUser,
			Content:   prompt,
			CreatedAt: now,
		}},
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "ask: %v\n", err)
		return 1
	}
	if jsonOutput {
		_ = json.NewEncoder(stdout).Encode(result)
		return 0
	}
	_, _ = fmt.Fprintln(stdout, result.Message.Content)
	return 0
}
