package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nobodycan/digital-twin/internal/app"
	"github.com/nobodycan/digital-twin/internal/evals"
	"github.com/nobodycan/digital-twin/internal/governance"
	"github.com/nobodycan/digital-twin/internal/llm"
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
	case "eval":
		return runEval(args[1:], stdout, stderr)
	case "decisions":
		return runDecisions(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func runDecisions(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("decisions", flag.ContinueOnError)
	flags.SetOutput(stderr)
	storeRoot := flags.String("store", "data", "local governance store root")
	tenantID := flags.String("tenant", "tenant-1", "tenant id to list")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	store := governance.NewFileDecisionStore(*storeRoot)
	records, err := store.ListDecisions(*tenantID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "list decisions: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(records); err != nil {
		_, _ = fmt.Fprintf(stderr, "encode decisions: %v\n", err)
		return 1
	}
	return 0
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
	local, err := app.NewLocalRuntime(app.LocalRuntimeConfig{
		PersonaLLM:         llm.LocalClient{},
		PersonaLLMProvider: "local",
	})
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

func runEval(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eval", flag.ContinueOnError)
	flags.SetOutput(stderr)
	casesDir := flags.String("cases", "evals/conversations", "directory containing eval case JSON files")
	reportsDir := flags.String("reports", "evals/reports", "directory for generated eval reports")
	runID := flags.String("run-id", "local-eval", "eval run id used for report file names")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	cases, err := evals.LoadCases(*casesDir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load eval cases: %v\n", err)
		return 1
	}
	outputs := make(map[string]evals.EvaluationOutput, len(cases))
	for _, evalCase := range cases {
		outputs[evalCase.ID] = evalCase.Output
	}
	runner := evals.Runner{Evaluators: []evals.Evaluator{
		evals.PersonaEvaluator{},
		evals.RAGEvaluator{},
		evals.ToolEvaluator{},
		evals.MemoryEvaluator{},
		evals.SafetyEvaluator{},
		evals.TenantIsolationEvaluator{},
		evals.CostEvaluator{MaxEstimatedTokens: 1000},
	}}
	result := runner.Run(cases, outputs)
	result.ID = *runID
	result.VersionMetadata = types.Metadata{
		"tenant_id":                "local",
		"persona_version_id":       "fixture",
		"tool_policy_version_id":   "fixture",
		"knowledge_version_id":     "fixture",
		"memory_policy_version_id": "fixture",
		"model_policy_version_id":  "fixture",
	}
	paths, err := evals.WriteReports(*reportsDir, result)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "write eval reports: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "eval run %s status=%s json=%s markdown=%s\n", result.ID, result.Status, paths.JSONPath, paths.MarkdownPath)
	if result.Status == evals.SuiteFailed {
		return 1
	}
	return 0
}
