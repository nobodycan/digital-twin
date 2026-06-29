package scripts_test

import (
	"os"
	"strings"
	"testing"
)

func TestStartDeepSeekScriptPrintsPhase9RuntimeHints(t *testing.T) {
	data, err := os.ReadFile("start-deepseek.ps1")
	if err != nil {
		t.Fatalf("read start-deepseek.ps1: %v", err)
	}
	script := string(data)

	for _, want := range []string{
		"ServerPid",
		"BrowserUrl",
		"SmokeCommand",
		"FallbackPolicy",
		"fail_closed",
		"ConversationUrl",
		"Start-Process",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("start-deepseek.ps1 missing %q", want)
		}
	}
}

func TestSmokeConversationScriptPrintsProviderDiagnostics(t *testing.T) {
	data, err := os.ReadFile("smoke-conversation.ps1")
	if err != nil {
		t.Fatalf("read smoke-conversation.ps1: %v", err)
	}
	script := string(data)

	for _, want := range []string{
		"/runtime/status",
		"Provider diagnostic",
		"generation_mode_hint",
		"fallback_policy",
		"sanitized",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("smoke-conversation.ps1 missing %q", want)
		}
	}
}
