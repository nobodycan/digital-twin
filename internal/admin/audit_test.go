package admin

import (
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/governance"
)

func TestAuditServiceRecordsAndListsConversationAudit(t *testing.T) {
	service := NewAuditService(NewInMemoryAuditStore())

	record, err := service.Record("tenant-1", AuditRecord{
		ConversationID: "conv-1",
		UserID:         "user-1",
		Status:         AuditStatusCompleted,
		AgentName:      "persona-agent",
		LatencyMS:      42,
		EventSummary:   []string{"assistant_text_delta", "audio_chunk", "done"},
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
