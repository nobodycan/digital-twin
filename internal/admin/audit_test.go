package admin

import (
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/governance"
)

func TestAuditServiceRecordsAndListsConversationAudit(t *testing.T) {
	service := NewAuditService(NewInMemoryAuditStore())

	record, err := service.Record("tenant-1", AuditRecord{
		ConversationID:       "conv-1",
		UserID:               "user-1",
		Status:               AuditStatusCompleted,
		AgentName:            "persona-agent",
		LatencyMS:            42,
		EventSummary:         []string{"assistant_text_delta", "audio_chunk", "done"},
		KnowledgeAnswerState: "grounded",
		KnowledgeSourceCount: 2,
		KnowledgeEvidence: map[string]any{
			"answer_state": "grounded",
			"summary":      "Grounded by 2 reviewed sources.",
		},
	})
	if err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	if record.ID == "" {
		t.Fatalf("expected audit ID")
	}

	recent, err := service.Recent("tenant-1")
	if err != nil {
		t.Fatalf("Recent returned error: %v", err)
	}
	if len(recent) != 1 || recent[0].ConversationID != "conv-1" {
		t.Fatalf("recent audit = %#v", recent)
	}
	if recent[0].KnowledgeAnswerState != "grounded" || recent[0].KnowledgeSourceCount != 2 {
		t.Fatalf("recent audit = %#v, want knowledge evidence fields", recent[0])
	}
}

func TestFileAuditStorePersistsRecords(t *testing.T) {
	dir := t.TempDir()
	first := NewAuditService(NewFileAuditStore(dir))
	if _, err := first.Record("tenant-1", AuditRecord{ConversationID: "conv-1", UserID: "user-1", Status: AuditStatusCompleted}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	second := NewAuditService(NewFileAuditStore(dir))
	recent, err := second.Recent("tenant-1")
	if err != nil {
		t.Fatalf("Recent after reopen returned error: %v", err)
	}
	if len(recent) != 1 || recent[0].ConversationID != "conv-1" {
		t.Fatalf("recent after reopen = %#v", recent)
	}
}

func TestDecisionAuditExporterRecordsGovernanceDecisions(t *testing.T) {
	audit := NewAuditService(NewInMemoryAuditStore())
	exporter := DecisionAuditExporter{Audit: audit}

	_, err := exporter.RecordDecision(governance.DecisionRecord{
		ID:        "release-candidate-1",
		TenantID:  "tenant-1",
		Type:      governance.DecisionRelease,
		ActorID:   "operator-1",
		CreatedAt: time.Now().UTC(),
		Evidence:  map[string]any{"decision": "blocked", "failed_case_ids": []string{"persona-disclosure"}},
	})
	if err != nil {
		t.Fatalf("RecordDecision returned error: %v", err)
	}

	recent, err := audit.Recent("tenant-1")
	if err != nil {
		t.Fatalf("Recent returned error: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("recent = %#v, want one audit record", recent)
	}
	record := recent[0]
	if record.ConversationID != "governance-release-candidate-1" || record.UserID != "operator-1" || record.AgentName != "governance" {
		t.Fatalf("audit record = %#v, want governance decision summary", record)
	}
	if len(record.EventSummary) == 0 || record.EventSummary[0] != "governance:release" {
		t.Fatalf("event summary = %#v", record.EventSummary)
	}
}

func TestAuditServiceTimelineProjectsEvidenceAndGap(t *testing.T) {
	service := NewAuditService(NewInMemoryAuditStore())
	service.now = func() time.Time { return time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC) }
	if _, err := service.Record("tenant-1", AuditRecord{
		ID:                      "audit-conv-1",
		ConversationID:          "conv-1",
		UserID:                  "user-1",
		Status:                  AuditStatusCompleted,
		AgentName:               "persona-agent",
		LatencyMS:               42,
		QuestionSummary:         "How should I verify DeepSeek startup?",
		KnowledgeSpaceID:        "default",
		KnowledgeNoSourceReason: "",
		KnowledgeAnswerState:    "grounded",
		KnowledgeSourceCount:    2,
		KnowledgeEvidence: map[string]any{
			"answer_state": "grounded",
			"summary":      "Grounded by 2 reviewed sources.",
			"citations": []map[string]any{
				{
					"document_id":   "kb-1",
					"title":         "Support Playbook",
					"review_status": "active",
					"source_type":   "local_text_file",
					"snippet":       "Run the smoke conversation script after boot.",
				},
				{
					"document_id":   "kb-2",
					"title":         "FAQ",
					"review_status": "active",
					"source_type":   "local_text_file",
					"snippet":       "Use fail_closed for verification.",
				},
			},
			"gaps": []map[string]any{
				{"gap_id": "gap-1", "status": "resolved", "reason": "no_matching_chunks"},
			},
			"diagnostics": map[string]any{
				"no_source_reason":   "",
				"review_gated_count": 0,
			},
		},
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	items, err := service.Timeline("tenant-1", AnswerAuditTimelineFilter{})
	if err != nil {
		t.Fatalf("Timeline returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("timeline count = %d, want 1", len(items))
	}
	item := items[0]
	if item.AuditID != "audit-conv-1" || item.QuestionSummary != "How should I verify DeepSeek startup?" {
		t.Fatalf("timeline item = %#v", item)
	}
	if item.AnswerState != "grounded" || item.SourceCount != 2 {
		t.Fatalf("timeline item = %#v, want grounded source count 2", item)
	}
	if len(item.TopSources) != 2 || item.TopSources[0].DocumentID != "kb-1" {
		t.Fatalf("top sources = %#v", item.TopSources)
	}
	if item.Gap == nil || item.Gap.GapID != "gap-1" || item.Gap.Status != string(KnowledgeGapResolved) {
		t.Fatalf("gap = %#v", item.Gap)
	}
}

func TestAuditServiceTimelineFiltersWeakOnlyAndDocumentID(t *testing.T) {
	service := NewAuditService(NewInMemoryAuditStore())
	service.now = func() time.Time { return time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC) }
	if _, err := service.Record("tenant-1", AuditRecord{
		ID:                   "audit-grounded",
		ConversationID:       "conv-1",
		Status:               AuditStatusCompleted,
		KnowledgeAnswerState: "grounded",
		KnowledgeEvidence: map[string]any{
			"citations": []map[string]any{{"document_id": "kb-1", "title": "Support Playbook"}},
		},
	}); err != nil {
		t.Fatalf("Record(grounded) returned error: %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC) }
	if _, err := service.Record("tenant-1", AuditRecord{
		ID:                      "audit-weak",
		ConversationID:          "conv-2",
		Status:                  AuditStatusCompleted,
		KnowledgeAnswerState:    "unsupported",
		KnowledgeNoSourceReason: "no_matching_chunks",
		KnowledgeEvidence: map[string]any{
			"answer_state": "unsupported",
			"summary":      "No supporting evidence recorded.",
			"citations":    []map[string]any{{"document_id": "kb-2", "title": "FAQ"}},
		},
	}); err != nil {
		t.Fatalf("Record(weak) returned error: %v", err)
	}

	weakOnly, err := service.Timeline("tenant-1", AnswerAuditTimelineFilter{WeakOnly: true})
	if err != nil {
		t.Fatalf("Timeline weak-only returned error: %v", err)
	}
	if len(weakOnly) != 1 || weakOnly[0].ConversationID != "conv-2" {
		t.Fatalf("weak-only = %#v", weakOnly)
	}

	filtered, err := service.Timeline("tenant-1", AnswerAuditTimelineFilter{DocumentID: "kb-1"})
	if err != nil {
		t.Fatalf("Timeline document filter returned error: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ConversationID != "conv-1" {
		t.Fatalf("document filtered = %#v", filtered)
	}
}

func TestAuditServiceTimelineUsesStableIDTieBreakAndLimit(t *testing.T) {
	service := NewAuditService(NewInMemoryAuditStore())
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	if _, err := service.Record("tenant-1", AuditRecord{ID: "audit-a", ConversationID: "conv-a", Status: AuditStatusCompleted, CreatedAt: now}); err != nil {
		t.Fatalf("Record(a) returned error: %v", err)
	}
	if _, err := service.Record("tenant-1", AuditRecord{ID: "audit-b", ConversationID: "conv-b", Status: AuditStatusCompleted, CreatedAt: now}); err != nil {
		t.Fatalf("Record(b) returned error: %v", err)
	}

	items, err := service.Timeline("tenant-1", AnswerAuditTimelineFilter{Limit: 1})
	if err != nil {
		t.Fatalf("Timeline returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("timeline count = %d, want 1", len(items))
	}
	if items[0].AuditID != "audit-b" {
		t.Fatalf("first item = %#v, want audit-b by stable ID tie-break", items[0])
	}
}

func TestAuditServiceTimelineConvertsReviewGatedDiagnosticsWithoutLeakingText(t *testing.T) {
	service := NewAuditService(NewInMemoryAuditStore())
	if _, err := service.Record("tenant-1", AuditRecord{
		ID:                      "audit-gated",
		ConversationID:          "conv-gated",
		Status:                  AuditStatusCompleted,
		KnowledgeAnswerState:    "review_gated",
		KnowledgeNoSourceReason: "review_gated_documents",
		KnowledgeEvidence: map[string]any{
			"answer_state": "review_gated",
			"summary":      "Matching knowledge needs review.",
			"diagnostics": map[string]any{
				"no_source_reason":   "review_gated_documents",
				"review_gated_count": 2,
				"gated_snippet":      "do not leak",
			},
		},
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	items, err := service.Timeline("tenant-1", AnswerAuditTimelineFilter{})
	if err != nil {
		t.Fatalf("Timeline returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("timeline count = %d, want 1", len(items))
	}
	if items[0].Diagnostics.NoSourceReason != "review_gated_documents" || items[0].Diagnostics.ReviewGatedCount != 2 {
		t.Fatalf("diagnostics = %#v", items[0].Diagnostics)
	}
	if items[0].Summary == "do not leak" {
		t.Fatalf("summary leaked gated text: %#v", items[0])
	}
}
