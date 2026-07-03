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
		"ServerListenerPid",
		"BrowserUrl",
		"SmokeCommand",
		"FallbackPolicy",
		"fail_closed",
		"ConversationUrl",
		"Start-Process",
		"Wait-ServerReady",
		"-FilePath \"go\"",
		"http://127.0.0.1:$Port/health",
		"http://127.0.0.1:$Port/app",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("start-deepseek.ps1 missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"DIGITAL_TWIN_LLM_API_KEY='$ApiKey'",
		"-FilePath \"powershell\"",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("start-deepseek.ps1 unexpectedly contains %q", forbidden)
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
		"curl.exe",
		"--data-binary",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("smoke-conversation.ps1 missing %q", want)
		}
	}
}

func TestStopServerScriptSupportsListenerPidCleanup(t *testing.T) {
	data, err := os.ReadFile("stop-server.ps1")
	if err != nil {
		t.Fatalf("read stop-server.ps1: %v", err)
	}
	script := string(data)

	for _, want := range []string{
		"ServerListenerPid",
		"Stop-Process",
		"Get-NetTCPConnection",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("stop-server.ps1 missing %q", want)
		}
	}
}
