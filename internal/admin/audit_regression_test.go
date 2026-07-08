package admin

import (
	"testing"
	"time"
)

func TestAuditServiceRecentReturnsNewestRecordsFirst(t *testing.T) {
	service := NewAuditService(NewInMemoryAuditStore())
	service.now = func() time.Time { return time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC) }
	if _, err := service.Record("tenant-1", AuditRecord{ConversationID: "conv-old", Status: AuditStatusCompleted}); err != nil {
		t.Fatalf("Record(old) returned error: %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 7, 8, 11, 0, 0, 0, time.UTC) }
	if _, err := service.Record("tenant-1", AuditRecord{ConversationID: "conv-new", Status: AuditStatusCompleted}); err != nil {
		t.Fatalf("Record(new) returned error: %v", err)
	}

	recent, err := service.Recent("tenant-1")
	if err != nil {
		t.Fatalf("Recent returned error: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("recent count = %d, want 2", len(recent))
	}
	if recent[0].ConversationID != "conv-new" || recent[1].ConversationID != "conv-old" {
		t.Fatalf("recent order = %#v, want newest first", recent)
	}
}
