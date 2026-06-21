package admin

import "testing"

func TestKnowledgeServiceUploadsChunksAndRunsCitationTest(t *testing.T) {
	service := NewKnowledgeService(NewInMemoryKnowledgeStore())

	doc, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-1",
		Name:    "planning.md",
		Content: "Phase 4 adds a digital human UI.\n\nIt also adds admin controls for persona and memory.",
	})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if len(doc.Chunks) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(doc.Chunks))
	}
	if doc.Status != KnowledgeReady {
		t.Fatalf("status = %q, want ready", doc.Status)
	}

	citation, err := service.CitationTest("tenant-1", "digital human UI")
	if err != nil {
		t.Fatalf("CitationTest returned error: %v", err)
	}
	if citation.DocumentID != doc.ID || citation.ChunkID == "" {
		t.Fatalf("citation = %#v", citation)
	}
}

func TestKnowledgeServiceRejectsEmptyUpload(t *testing.T) {
	service := NewKnowledgeService(NewInMemoryKnowledgeStore())

	if _, err := service.Upload("tenant-1", KnowledgeUpload{ID: "empty", Name: "empty.md"}); err == nil {
		t.Fatalf("expected empty upload to be rejected")
	}
}

func TestFileKnowledgeStorePersistsDocuments(t *testing.T) {
	dir := t.TempDir()
	first := NewKnowledgeService(NewFileKnowledgeStore(dir))
	if _, err := first.Upload("tenant-1", KnowledgeUpload{ID: "kb-1", Name: "planning.md", Content: "Phase 4 adds admin controls."}); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	second := NewKnowledgeService(NewFileKnowledgeStore(dir))
	citation, err := second.CitationTest("tenant-1", "admin controls")
	if err != nil {
		t.Fatalf("CitationTest after reopen returned error: %v", err)
	}
	if citation.DocumentID != "kb-1" {
		t.Fatalf("citation document = %q, want kb-1", citation.DocumentID)
	}
}
