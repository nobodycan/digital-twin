package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestRunAskPrintsAssistantResponse(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"ask", "hello"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "professional persona") {
		t.Fatalf("stdout = %q, want assistant response", stdout.String())
	}
}

func TestRunAskRequiresPrompt(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"ask"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("run() code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "prompt is required") {
		t.Fatalf("stderr = %q, want prompt is required", stderr.String())
	}
}

func TestRunAskPrintsJSONWhenRequested(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"ask", "--json", "hello"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() code = %d, stderr = %s", code, stderr.String())
	}
	var result types.AgentResult
	if err := json.NewDecoder(&stdout).Decode(&result); err != nil {
		t.Fatalf("decode stdout JSON: %v\nstdout=%s", err, stdout.String())
	}
	if result.AgentName == "" || result.Message.Role != types.RoleAssistant {
		t.Fatalf("result = %#v, want assistant agent result", result)
	}
}
