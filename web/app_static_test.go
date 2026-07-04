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
		"#knowledge-gap-queue",
		"#knowledge-edit-form[hidden]",
		".knowledge-gap-summary",
		".knowledge-gap-row",
		"grid-template-columns: 1fr",
		"min-width: 0",
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
		`"/admin/knowledge/notes/create"`,
		`"/admin/knowledge/"`,
		`"/admin/knowledge/upload"`,
		`"/admin/knowledge/import"`,
		`"/admin/knowledge/imports"`,
		`"/admin/knowledge/disable"`,
		`"/admin/knowledge/enable"`,
		`"/admin/knowledge/delete"`,
		`"/admin/knowledge/update"`,
		`"/admin/knowledge/reindex"`,
		`"/admin/knowledge/citation-test"`,
		`"/admin/knowledge/retrieval-diagnostics"`,
		"selectedKnowledgeSpaceId",
		"loadKnowledgeSpaces",
		"knowledge-space-select",
		"knowledge-space-create",
		"space_id",
		"knowledge-import-source-type",
		"knowledge-import-source-label",
		"knowledge-import-url",
		"knowledge-import-content",
		"knowledge-import-run",
		"knowledge-import-jobs",
		"knowledge-import-panel",
		"knowledge-import-job-row",
		"knowledge-import-job-summary",
		"knowledge-import-job-actions",
		"loadKnowledgeImportJobs",
		"renderKnowledgeImportJob",
		"importKnowledge",
		"readKnowledgeImportSources",
		"inspectKnowledgeDocument",
		`"/admin/tools/policy"`,
		`"/admin/tools/authorize"`,
		`"/admin/audit"`,
		"loadKnowledge",
		"knowledge-table-body",
		"knowledge-detail",
		"knowledge-detail-title",
		"knowledge-detail-meta",
		"knowledge-detail-relations",
		"knowledge-filter-query",
		"knowledge-filter-status",
		"knowledge-filter-source-type",
		"knowledge-filter-gap-linked",
		"knowledge-filter-apply",
		"knowledge-filter-reset",
		"knowledge-edit-name",
		"knowledge-edit-content",
		"knowledge-edit-source-label",
		"knowledge-edit-save",
		"knowledge-edit-cancel",
		"knowledge-edit-toggle",
		"knowledge-edit-form",
		"knowledge-health-summary",
		"knowledge-health-status",
		"knowledge-attention-reasons",
		"knowledge-gap-queue",
		"knowledge-note-title",
		"knowledge-note-body",
		"knowledge-note-create",
		"knowledge-note-gap-context",
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
		"renderKnowledgeRelations",
		"renderKnowledgeSourceMeta",
		"renderKnowledgeMetric",
		"renderKnowledgeFlag",
		"loadKnowledgeGaps",
		"renderKnowledgeGapRow",
		"selectedKnowledgeDocumentId",
		"selectedKnowledgeDocument",
		"knowledgeEditDraft",
		"startKnowledgeEdit",
		"cancelKnowledgeEdit",
		"saveKnowledgeEdit",
		`removeAttribute("hidden")`,
		`setAttribute("hidden", "hidden")`,
		"applyKnowledgeFilters",
		"resetKnowledgeFilters",
		"createKnowledgeNoteFromGap",
		"runKnowledgeGapDiagnostics",
		"resolveKnowledgeGap",
		"knowledgeGapInvestigate",
		"renderKnowledgeDebugResults",
		"renderKnowledgeDebugRow",
		"clearElement",
		"chunk_count",
		"renderKnowledgeDiagnostics",
		"no_source_reason",
		"index_status",
		"source_gap_id",
		"source_label",
		"source_warning",
		"duplicate_content",
		"unsupported_extension",
		"resolved_gaps",
		"relations",
		"resolution_note",
		"investigating",
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
		"Local-first controls for Phase 18",
		`id="knowledge-space-select"`,
		`id="knowledge-space-create"`,
		`id="knowledge-space-create-button"`,
		`id="knowledge-import-panel"`,
		`id="knowledge-import-source-type"`,
		`id="knowledge-import-source-label"`,
		`id="knowledge-import-url"`,
		`id="knowledge-import-content"`,
		`id="knowledge-import-run"`,
		`id="knowledge-import-jobs"`,
		`value="local_text_file"`,
		`value="url_text_snapshot"`,
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
		`id="knowledge-filter-query"`,
		`id="knowledge-filter-status"`,
		`id="knowledge-filter-source-type"`,
		`id="knowledge-filter-gap-linked"`,
		`id="knowledge-filter-apply"`,
		`id="knowledge-filter-reset"`,
		`id="knowledge-detail-title"`,
		`id="knowledge-detail-meta"`,
		`id="knowledge-detail-relations"`,
		`id="knowledge-edit-toggle"`,
		`id="knowledge-edit-name"`,
		`id="knowledge-edit-content"`,
		`id="knowledge-edit-source-label"`,
		`id="knowledge-edit-save"`,
		`id="knowledge-edit-cancel"`,
		`id="knowledge-edit-form" class="knowledge-edit-form" hidden`,
		`id="knowledge-detail-flags"`,
		`id="knowledge-debug-results"`,
		`id="knowledge-gap-queue"`,
		`id="knowledge-note-title"`,
		`id="knowledge-note-body"`,
		`id="knowledge-note-create"`,
		`id="knowledge-note-gap-context"`,
		`id="knowledge-status"`,
		"Chunk preview",
		"Health summary",
		"Knowledge import",
		"Recent imports",
		"Knowledge filters",
		"Document detail",
		"Source relationships",
		"Edit document",
		"Retrieval debug",
		"Knowledge gaps",
		"Knowledge note",
		"Run diagnostics",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("admin.html missing %q", want)
		}
	}
}
