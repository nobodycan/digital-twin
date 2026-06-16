package main

import (
	"bytes"
	"strings"
	"testing"
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
