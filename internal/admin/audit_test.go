package admin

import "testing"

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
