package admin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
	if doc.SourceType != KnowledgeSourceMarkdown {
		t.Fatalf("source type = %q, want %q", doc.SourceType, KnowledgeSourceMarkdown)
	}
	if doc.ContentHash == "" {
		t.Fatalf("content hash should be populated")
	}
	if doc.ChunkCount != 2 {
		t.Fatalf("chunk count metadata = %d, want 2", doc.ChunkCount)
	}
	if doc.UpdatedAt.IsZero() {
		t.Fatalf("updated at should be set")
	}
	if len(doc.Chunks) > 0 && doc.Chunks[0].Ordinal != 1 {
		t.Fatalf("first chunk ordinal = %d, want 1", doc.Chunks[0].Ordinal)
	}
	if len(doc.Chunks) > 0 && doc.Chunks[0].DocumentID != doc.ID {
		t.Fatalf("first chunk document id = %q, want %q", doc.Chunks[0].DocumentID, doc.ID)
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

func TestKnowledgeServiceSupportsDocumentLifecycle(t *testing.T) {
	store := NewInMemoryKnowledgeStore()
	service := NewKnowledgeService(store)

	doc, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-2",
		Name:    "ops.md",
		Content: "Runbook step one.\n\nRunbook step two.",
	})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	loaded, err := service.Get("tenant-1", doc.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if loaded.ID != doc.ID {
		t.Fatalf("loaded id = %q, want %q", loaded.ID, doc.ID)
	}

	disabled, err := service.Disable("tenant-1", doc.ID)
	if err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if disabled.Status != KnowledgeDisabled {
		t.Fatalf("disabled status = %q, want %q", disabled.Status, KnowledgeDisabled)
	}

	if _, err := service.CitationTest("tenant-1", "Runbook"); err == nil {
		t.Fatalf("expected disabled document to be excluded from citation test")
	}

	enabled, err := service.Enable("tenant-1", doc.ID)
	if err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}
	if enabled.Status != KnowledgeReady {
		t.Fatalf("enabled status = %q, want %q", enabled.Status, KnowledgeReady)
	}

	if _, err := service.Reindex("tenant-1", doc.ID, "Runbook step one.\n\nUpdated step two."); err != nil {
		t.Fatalf("Reindex returned error: %v", err)
	}

	if err := service.Delete("tenant-1", doc.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := service.Get("tenant-1", doc.ID); err == nil {
		t.Fatalf("expected deleted document lookup to fail")
	}
}

func TestKnowledgeServiceListsDocumentsInStableOrder(t *testing.T) {
	store := NewInMemoryKnowledgeStore()
	service := NewKnowledgeService(store)
	service.now = func() time.Time {
		return time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	}

	for _, upload := range []KnowledgeUpload{
		{ID: "kb-b", Name: "b.md", Content: "second"},
		{ID: "kb-a", Name: "a.md", Content: "first"},
	} {
		if _, err := service.Upload("tenant-1", upload); err != nil {
			t.Fatalf("Upload(%s) returned error: %v", upload.ID, err)
		}
	}

	documents, err := service.List("tenant-1")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(documents) != 2 {
		t.Fatalf("document count = %d, want 2", len(documents))
	}
	if documents[0].ID != "kb-a" || documents[1].ID != "kb-b" {
		t.Fatalf("document order = [%s %s], want [kb-a kb-b]", documents[0].ID, documents[1].ID)
	}
}

func TestFileKnowledgeStoreSupportsLifecycleAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	first := NewKnowledgeService(NewFileKnowledgeStore(dir))

	doc, err := first.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-3",
		Name:    "policy.md",
		Content: "Policy version one.",
	})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if _, err := first.Disable("tenant-1", doc.ID); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if _, err := first.Enable("tenant-1", doc.ID); err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}
	if _, err := first.Reindex("tenant-1", doc.ID, "Policy version two."); err != nil {
		t.Fatalf("Reindex returned error: %v", err)
	}

	second := NewKnowledgeService(NewFileKnowledgeStore(dir))
	loaded, err := second.Get("tenant-1", doc.ID)
	if err != nil {
		t.Fatalf("Get after reopen returned error: %v", err)
	}
	if loaded.ContentHash == "" {
		t.Fatalf("content hash should persist after reopen")
	}
	if loaded.ChunkCount != 1 {
		t.Fatalf("chunk count after reopen = %d, want 1", loaded.ChunkCount)
	}

	if err := second.Delete("tenant-1", doc.ID); err != nil {
		t.Fatalf("Delete after reopen returned error: %v", err)
	}

	third := NewKnowledgeService(NewFileKnowledgeStore(dir))
	if _, err := third.Get("tenant-1", doc.ID); err == nil {
		t.Fatalf("expected deleted document to stay deleted after reopen")
	}
}

func TestFileKnowledgeStoreRejectsUnsafeDocumentIDs(t *testing.T) {
	service := NewKnowledgeService(NewFileKnowledgeStore(t.TempDir()))

	_, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "../escape",
		Name:    "escape.md",
		Content: "nope",
	})
	if err == nil {
		t.Fatalf("expected unsafe document id to be rejected")
	}
}

func TestFileKnowledgeStoreLeavesNoTemporaryFilesBehind(t *testing.T) {
	dir := t.TempDir()
	service := NewKnowledgeService(NewFileKnowledgeStore(dir))

	if _, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-4",
		Name:    "ops.md",
		Content: "v1",
	}); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if _, err := service.Reindex("tenant-1", "kb-4", "v2"); err != nil {
		t.Fatalf("Reindex returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}

	data, err := os.ReadFile(filepath.Join(dir, "knowledge.json"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("knowledge.json should not be empty")
	}
}
