package web_test

import (
	"os"
	"strings"
	"testing"
)

func TestAppScriptPostsToExperienceStreamAndRendersPresentationEvents(t *testing.T) {
	data, err := os.ReadFile("app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	script := string(data)

	for _, want := range []string{
		`fetch("/experience/stream"`,
		"parseSSEFrames",
		"assistant_text_delta",
		"activeAssistantLine",
		"subtitle",
		"avatar_state",
		"audio_chunk",
		"error",
		"done",
		"conversationId",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}

func TestAppScriptSupportsAbortableStreamingAndStopState(t *testing.T) {
	data, err := os.ReadFile("app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	script := string(data)

	for _, want := range []string{
		"AbortController",
		"stop-button",
		"activeRequestController",
		`error.name === "AbortError"`,
		"not saved",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}

func TestAppScriptPostsMockVoiceToVoiceStream(t *testing.T) {
	data, err := os.ReadFile("app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	script := string(data)

	for _, want := range []string{
		`fetch("/experience/mock-voice/stream"`,
		"audio_text",
		"asr_final",
		"Mock voice",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}

func TestAppStylesDefineVisibleAvatarStates(t *testing.T) {
	data, err := os.ReadFile("app.css")
	if err != nil {
		t.Fatalf("read app.css: %v", err)
	}
	styles := string(data)

	for _, want := range []string{
		`[data-state="listening"]`,
		`[data-state="thinking"]`,
		`[data-state="speaking"]`,
		`[data-state="error"]`,
		`[data-state="interrupted"]`,
		"transcript-line-assistant",
		"transcript-line-pending",
		"transcript-line-status",
	} {
		if !strings.Contains(styles, want) {
			t.Fatalf("app.css missing %q", want)
		}
	}
}

func TestAppShellIncludesStopButton(t *testing.T) {
	html, err := os.ReadFile("app.html")
	if err != nil {
		t.Fatalf("read app.html: %v", err)
	}
	source := string(html)
	for _, want := range []string{
		`id="stop-button"`,
		">Stop<",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("app.html missing %q", want)
		}
	}
}

func TestAdminShellLoadsPersonaAdminScript(t *testing.T) {
	html, err := os.ReadFile("admin.html")
	if err != nil {
		t.Fatalf("read admin.html: %v", err)
	}
	if !strings.Contains(string(html), `/web/admin.js`) {
		t.Fatalf("admin.html should load /web/admin.js")
	}

	script, err := os.ReadFile("admin.js")
	if err != nil {
		t.Fatalf("read admin.js: %v", err)
	}
	source := string(script)
	for _, want := range []string{
		`"/admin/persona/drafts"`,
		`"/admin/persona/publish"`,
		`"/admin/persona/rollback"`,
		`"/admin/persona/active"`,
		`"/admin/memory"`,
		`"/admin/memory/disable"`,
		`"/admin/knowledge/upload"`,
		`"/admin/knowledge/citation-test"`,
		`"/admin/tools/policy"`,
		`"/admin/tools/authorize"`,
		`"/admin/audit"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("admin.js missing %q", want)
		}
	}
}
