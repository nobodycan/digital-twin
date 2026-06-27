package web_test

import (
	"os"
	"strings"
	"testing"
)

// Regression: ISSUE-QA-001 - transcript could miss a later assistant reply
// Found by /qa on 2026-06-27
// Report: .gstack/qa-reports/qa-report-localhost-2026-06-27.md
func TestAppScriptKeepsAssistantTranscriptWhenSubtitleArrivesWithoutDelta(t *testing.T) {
	data, err := os.ReadFile("app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	script := string(data)

	for _, want := range []string{
		"latestAssistantText",
		`const subtitleText = renderSubtitle(payload.subtitles || []);`,
		"if (subtitleText && !activeAssistantLine)",
		"const finalAssistantText = latestAssistantText || subtitleLine.textContent.trim();",
		"appendLine(\"assistant\", finalAssistantText)",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}
