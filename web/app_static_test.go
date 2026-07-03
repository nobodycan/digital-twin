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
		"presenceSummary",
		"presenceSignalStrip",
		"derivePresenceSummary",
		"presenceSummaryCopy",
		"setPresenceSummary",
		"renderPresenceSignals",
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
		"knowledge_answer_state",
		"knowledge_space_id",
		"knowledge_space_name",
		"knowledge_citations",
		"memory_used",
		"Knowledge grounded",
		"Partially supported",
		"No supporting source",
		"Provider fallback",
		"Guardrail fallback",
		"Local mode",
		"No source used",
		"Memory considered",
		"Ready for the next turn",
		"Thinking through the request",
		"Responding now",
		"Used a local fallback reply",
		"Provider response could not be trusted",
		"Request was interrupted",
		"renderGroundingState",
		"renderAnswerState",
		"answerStateCopy",
		"renderCitationSummary",
		"knowledgeSpaceSelect",
		"selectedKnowledgeSpaceId",
		"clearTranscriptMeta",
		`payload.state !== "idle"`,
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
		"presence-header",
		"presence-summary",
		"presence-summary-text",
		"presence-signal-strip",
		"presence-visual-slot",
		"aspect-ratio",
		"-webkit-line-clamp: 3",
		"max-height",
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
		`id="presence-summary"`,
		`id="presence-summary-text"`,
		`id="presence-signal-strip"`,
		`id="presence-live-state"`,
		`id="presence-grounding-signal"`,
		`id="presence-memory-signal"`,
		`id="presence-fallback-signal"`,
		`id="knowledge-space-select"`,
		`id="knowledge-space-status"`,
		`id="session-provider"`,
		`id="session-model"`,
		`id="status-chip"`,
		`id="presence-panel"`,
		`id="stop-button"`,
		"Latest takeaway",
		"Live state",
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
		`"/admin/knowledge/spaces"`,
		`"/admin/knowledge/spaces/create"`,
		`"/admin/persona/drafts"`,
		`"/admin/persona/publish"`,
		`"/admin/persona/rollback"`,
		`"/admin/persona/active"`,
		`"/admin/memory"`,
		`"/admin/memory/disable"`,
		`"/admin/knowledge"`,
		`"/admin/knowledge/health"`,
		`"/admin/knowledge/gaps"`,
		`"/admin/knowledge/gaps/update"`,
		`"/admin/knowledge/"`,
		`"/admin/knowledge/upload"`,
		`"/admin/knowledge/disable"`,
		`"/admin/knowledge/enable"`,
		`"/admin/knowledge/delete"`,
		`"/admin/knowledge/reindex"`,
		`"/admin/knowledge/citation-test"`,
		`"/admin/knowledge/retrieval-diagnostics"`,
		"selectedKnowledgeSpaceId",
		"loadKnowledgeSpaces",
		"knowledge-space-select",
		"knowledge-space-create",
		"space_id",
		`"/admin/tools/policy"`,
		`"/admin/tools/authorize"`,
		`"/admin/audit"`,
		"loadKnowledge",
		"knowledge-table-body",
		"knowledge-detail",
		"knowledge-health-summary",
		"knowledge-health-status",
		"knowledge-attention-reasons",
		"knowledge-gap-queue",
		"knowledge-debug-results",
		"knowledge-health-metrics",
		"knowledge-detail-flags",
		"knowledge-debug-empty",
		"knowledge-debug-row",
		"knowledge-gap-actions",
		"knowledge-query-mode",
		"renderKnowledgeRow",
		"renderMemoryRow",
		"loadKnowledgeHealth",
		"renderKnowledgeHealth",
		"renderKnowledgeDetail",
		"renderKnowledgeMetric",
		"renderKnowledgeFlag",
		"loadKnowledgeGaps",
		"renderKnowledgeGapRow",
		"renderKnowledgeDebugResults",
		"renderKnowledgeDebugRow",
		"clearElement",
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
		"Local-first controls for Phase 14",
		`id="knowledge-space-select"`,
		`id="knowledge-space-create"`,
		`id="knowledge-space-create-button"`,
		`id="knowledge-upload"`,
		`id="knowledge-upload-mock"`,
		`id="knowledge-query"`,
		`id="knowledge-query-mode"`,
		`id="knowledge-query-run"`,
		`id="knowledge-health-summary"`,
		`id="knowledge-health-status"`,
		`id="knowledge-health-metrics"`,
		`id="knowledge-attention-reasons"`,
		`id="knowledge-table-body"`,
		`id="knowledge-detail"`,
		`id="knowledge-detail-flags"`,
		`id="knowledge-debug-results"`,
		`id="knowledge-gap-queue"`,
		`id="knowledge-status"`,
		"Chunk preview",
		"Health summary",
		"Retrieval debug",
		"Knowledge gaps",
		"Run diagnostics",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("admin.html missing %q", want)
		}
	}
}
