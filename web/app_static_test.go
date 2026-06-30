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
		`fetch("/runtime/status")`,
		`fetch("/experience/stream"`,
		"parseSSEFrames",
		"assistant_text_delta",
		"activeAssistantLine",
		"runtime-status",
		"fallback",
		"providerStatus",
		"setProviderStatus",
		"subtitle",
		"avatar_state",
		"audio_chunk",
		"error",
		"done",
		"conversationId",
		"knowledge_used",
		"knowledge_citations",
		"memory_used",
		"Knowledge grounded",
		"No source used",
		"Memory considered",
		"renderGroundingState",
		"renderCitationSummary",
		"clearTranscriptMeta",
		"const completedLine = activeAssistantLine",
		"finalizeAssistantLine(completedLine, metadata)",
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
		":root {",
		"provider-strip",
		"status-chip",
		"presence-panel",
		"transcript-badge",
		"transcript-meta",
		"transcript-citation",
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
		`id="runtime-status"`,
		`id="provider-strip"`,
		`id="session-provider"`,
		`id="session-model"`,
		`id="status-chip"`,
		`id="presence-panel"`,
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
		`"/admin/knowledge"`,
		`"/admin/knowledge/"`,
		`"/admin/knowledge/upload"`,
		`"/admin/knowledge/disable"`,
		`"/admin/knowledge/enable"`,
		`"/admin/knowledge/delete"`,
		`"/admin/knowledge/reindex"`,
		`"/admin/knowledge/citation-test"`,
		`"/admin/knowledge/retrieval-diagnostics"`,
		`"/admin/tools/policy"`,
		`"/admin/tools/authorize"`,
		`"/admin/audit"`,
		"loadKnowledge",
		"knowledge-table-body",
		"knowledge-detail",
		"knowledge-query-mode",
		"renderKnowledgeRow",
		"renderMemoryRow",
		"chunk_count",
		"renderKnowledgeDiagnostics",
		"no_source_reason",
		"index_status",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("admin.js missing %q", want)
		}
	}
}

func TestAdminShellIncludesKnowledgeLifecycleControls(t *testing.T) {
	html, err := os.ReadFile("admin.html")
	if err != nil {
		t.Fatalf("read admin.html: %v", err)
	}
	source := string(html)
	for _, want := range []string{
		`id="knowledge-upload"`,
		`id="knowledge-upload-mock"`,
		`id="knowledge-query"`,
		`id="knowledge-query-mode"`,
		`id="knowledge-query-run"`,
		`id="knowledge-table-body"`,
		`id="knowledge-detail"`,
		`id="knowledge-status"`,
		"Chunk preview",
		"Run diagnostics",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("admin.html missing %q", want)
		}
	}
}
