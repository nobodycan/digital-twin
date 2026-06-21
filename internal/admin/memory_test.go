package admin

import "testing"

func TestMemoryServiceListsAndDisablesRecords(t *testing.T) {
	service := NewMemoryService(NewInMemoryMemoryStore())
	record, err := service.Save("tenant-1", MemoryRecord{
		ID:             "mem-1",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Summary:        "prefers concise plans",
		Status:         MemoryActive,
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	active, err := service.ActiveForRecall("tenant-1", "user-1")
	if err != nil {
		t.Fatalf("ActiveForRecall returned error: %v", err)
	}
	if len(active) != 1 || active[0].ID != record.ID {
		t.Fatalf("active memories = %#v", active)
	}

	disabled, err := service.Disable("tenant-1", record.ID)
	if err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if disabled.Status != MemoryDisabled {
		t.Fatalf("status = %q, want disabled", disabled.Status)
	}

	active, err = service.ActiveForRecall("tenant-1", "user-1")
	if err != nil {
		t.Fatalf("ActiveForRecall after disable returned error: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("disabled memory should not be active for recall: %#v", active)
	}
}

func TestFileMemoryStorePersistsDisabledStatus(t *testing.T) {
	dir := t.TempDir()
	first := NewMemoryService(NewFileMemoryStore(dir))
	if _, err := first.Save("tenant-1", MemoryRecord{ID: "mem-1", UserID: "user-1", Summary: "local fact", Status: MemoryActive}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if _, err := first.Disable("tenant-1", "mem-1"); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}

	second := NewMemoryService(NewFileMemoryStore(dir))
	active, err := second.ActiveForRecall("tenant-1", "user-1")
	if err != nil {
		t.Fatalf("ActiveForRecall after reopen returned error: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("disabled persisted memory should not be active: %#v", active)
	}
}
