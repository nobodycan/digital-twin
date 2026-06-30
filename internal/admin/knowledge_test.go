package admin

import (
	"encoding/json"
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
	if doc.Metadata[KnowledgeMetadataLexicalReady] != "true" {
		t.Fatalf("lexical_ready = %q, want true", doc.Metadata[KnowledgeMetadataLexicalReady])
	}
	if doc.Metadata[KnowledgeMetadataVectorStatus] != KnowledgeVectorMissing {
		t.Fatalf("vector_status = %q, want %q", doc.Metadata[KnowledgeMetadataVectorStatus], KnowledgeVectorMissing)
	}
	if doc.Metadata[KnowledgeMetadataIndexedAt] == "" {
		t.Fatalf("indexed_at should be set")
	}
	if len(doc.Chunks) > 0 && doc.Chunks[0].Ordinal != 1 {
		t.Fatalf("first chunk ordinal = %d, want 1", doc.Chunks[0].Ordinal)
	}
	if len(doc.Chunks) > 0 && doc.Chunks[0].DocumentID != doc.ID {
		t.Fatalf("first chunk document id = %q, want %q", doc.Chunks[0].DocumentID, doc.ID)
	}
	if len(doc.Chunks) > 0 && doc.Chunks[0].Metadata[KnowledgeMetadataVectorStatus] != KnowledgeVectorMissing {
		t.Fatalf("chunk vector_status = %q, want %q", doc.Chunks[0].Metadata[KnowledgeMetadataVectorStatus], KnowledgeVectorMissing)
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

func TestKnowledgeServiceCreatesDefaultSpaceAndAssignsUploads(t *testing.T) {
	service := NewKnowledgeService(NewInMemoryKnowledgeStore())

	spaces, err := service.ListSpaces("tenant-1")
	if err != nil {
		t.Fatalf("ListSpaces returned error: %v", err)
	}
	if len(spaces) != 1 {
		t.Fatalf("space count = %d, want 1", len(spaces))
	}
	if spaces[0].ID != DefaultKnowledgeSpaceID {
		t.Fatalf("default space id = %q, want %q", spaces[0].ID, DefaultKnowledgeSpaceID)
	}
	if spaces[0].Status != KnowledgeSpaceActive {
		t.Fatalf("default space status = %q, want %q", spaces[0].Status, KnowledgeSpaceActive)
	}

	doc, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-default",
		Name:    "default.md",
		Content: "default space content",
	})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if doc.SpaceID != DefaultKnowledgeSpaceID {
		t.Fatalf("SpaceID = %q, want %q", doc.SpaceID, DefaultKnowledgeSpaceID)
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

func TestKnowledgeServiceListsDocumentsBySpace(t *testing.T) {
	store := NewInMemoryKnowledgeStore()
	service := NewKnowledgeService(store)

	space, err := service.CreateSpace("tenant-1", KnowledgeSpaceInput{
		ID:          "product",
		Name:        "Product",
		Description: "Product docs",
	})
	if err != nil {
		t.Fatalf("CreateSpace returned error: %v", err)
	}

	for _, upload := range []KnowledgeUpload{
		{ID: "kb-default", Name: "default.md", Content: "default content"},
		{ID: "kb-product", Name: "product.md", Content: "product content", SpaceID: space.ID},
	} {
		if _, err := service.Upload("tenant-1", upload); err != nil {
			t.Fatalf("Upload(%s) returned error: %v", upload.ID, err)
		}
	}

	defaultDocuments, err := service.ListBySpace("tenant-1", DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("ListBySpace(default) returned error: %v", err)
	}
	if len(defaultDocuments) != 1 || defaultDocuments[0].ID != "kb-default" {
		t.Fatalf("default documents = %#v, want kb-default only", defaultDocuments)
	}

	productDocuments, err := service.ListBySpace("tenant-1", "product")
	if err != nil {
		t.Fatalf("ListBySpace(product) returned error: %v", err)
	}
	if len(productDocuments) != 1 || productDocuments[0].ID != "kb-product" {
		t.Fatalf("product documents = %#v, want kb-product only", productDocuments)
	}
}

func TestKnowledgeServiceRejectsUploadIntoDisabledSpace(t *testing.T) {
	service := NewKnowledgeService(NewInMemoryKnowledgeStore())

	if _, err := service.CreateSpace("tenant-1", KnowledgeSpaceInput{
		ID:   "ops",
		Name: "Ops",
	}); err != nil {
		t.Fatalf("CreateSpace returned error: %v", err)
	}
	if _, err := service.DisableSpace("tenant-1", "ops"); err != nil {
		t.Fatalf("DisableSpace returned error: %v", err)
	}

	if _, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-ops",
		Name:    "ops.md",
		Content: "ops content",
		SpaceID: "ops",
	}); err == nil {
		t.Fatalf("expected upload into disabled space to fail")
	}
}

func TestKnowledgeServiceCanMoveDocumentBetweenSpaces(t *testing.T) {
	service := NewKnowledgeService(NewInMemoryKnowledgeStore())

	if _, err := service.CreateSpace("tenant-1", KnowledgeSpaceInput{ID: "alpha", Name: "Alpha"}); err != nil {
		t.Fatalf("CreateSpace(alpha) returned error: %v", err)
	}
	if _, err := service.CreateSpace("tenant-1", KnowledgeSpaceInput{ID: "beta", Name: "Beta"}); err != nil {
		t.Fatalf("CreateSpace(beta) returned error: %v", err)
	}
	if _, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-move",
		Name:    "move.md",
		Content: "move me",
		SpaceID: "alpha",
	}); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	moved, err := service.MoveDocument("tenant-1", "kb-move", "beta")
	if err != nil {
		t.Fatalf("MoveDocument returned error: %v", err)
	}
	if moved.SpaceID != "beta" {
		t.Fatalf("SpaceID after move = %q, want beta", moved.SpaceID)
	}

	alphaDocuments, err := service.ListBySpace("tenant-1", "alpha")
	if err != nil {
		t.Fatalf("ListBySpace(alpha) returned error: %v", err)
	}
	if len(alphaDocuments) != 0 {
		t.Fatalf("alpha documents = %#v, want none", alphaDocuments)
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
	if loaded.Metadata[KnowledgeMetadataLexicalReady] != "true" {
		t.Fatalf("lexical_ready after reopen = %q, want true", loaded.Metadata[KnowledgeMetadataLexicalReady])
	}
	if loaded.Metadata[KnowledgeMetadataVectorStatus] != KnowledgeVectorMissing {
		t.Fatalf("vector_status after reopen = %q, want %q", loaded.Metadata[KnowledgeMetadataVectorStatus], KnowledgeVectorMissing)
	}

	if err := second.Delete("tenant-1", doc.ID); err != nil {
		t.Fatalf("Delete after reopen returned error: %v", err)
	}

	third := NewKnowledgeService(NewFileKnowledgeStore(dir))
	if _, err := third.Get("tenant-1", doc.ID); err == nil {
		t.Fatalf("expected deleted document to stay deleted after reopen")
	}
}

func TestFileKnowledgeStoreLoadsLegacyDocumentWithoutIndexMetadata(t *testing.T) {
	dir := t.TempDir()
	legacy := `[{"id":"kb-legacy","tenant_id":"tenant-1","name":"legacy.md","source_type":"markdown","status":"ready","content_hash":"abc","chunk_count":1,"chunks":[{"id":"kb-legacy:chunk-0001","document_id":"kb-legacy","ordinal":1,"text":"legacy content"}],"created_at":"2026-06-30T00:00:00Z","updated_at":"2026-06-30T00:00:00Z"}]`
	if err := os.WriteFile(filepath.Join(dir, "knowledge.json"), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy knowledge.json: %v", err)
	}

	service := NewKnowledgeService(NewFileKnowledgeStore(dir))
	document, err := service.Get("tenant-1", "kb-legacy")
	if err != nil {
		t.Fatalf("Get legacy document: %v", err)
	}
	if document.Metadata != nil && document.Metadata[KnowledgeMetadataVectorStatus] != "" {
		t.Fatalf("legacy vector_status = %q, want empty", document.Metadata[KnowledgeMetadataVectorStatus])
	}
	if len(document.Chunks) != 1 {
		t.Fatalf("legacy chunk count = %d, want 1", len(document.Chunks))
	}
	if document.SpaceID != DefaultKnowledgeSpaceID {
		t.Fatalf("legacy SpaceID = %q, want %q", document.SpaceID, DefaultKnowledgeSpaceID)
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

func TestFileKnowledgeStoreMigratesLegacyArrayToEnvelopeOnSave(t *testing.T) {
	dir := t.TempDir()
	legacy := `[{"id":"kb-legacy","tenant_id":"tenant-1","name":"legacy.md","source_type":"markdown","status":"ready","content_hash":"abc","chunk_count":1,"chunks":[{"id":"kb-legacy:chunk-0001","document_id":"kb-legacy","ordinal":1,"text":"legacy content"}],"created_at":"2026-06-30T00:00:00Z","updated_at":"2026-06-30T00:00:00Z"}]`
	if err := os.WriteFile(filepath.Join(dir, "knowledge.json"), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy knowledge.json: %v", err)
	}

	service := NewKnowledgeService(NewFileKnowledgeStore(dir))
	if _, err := service.Reindex("tenant-1", "kb-legacy", "legacy content updated"); err != nil {
		t.Fatalf("Reindex returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "knowledge.json"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var envelope struct {
		Spaces    []KnowledgeSpace    `json:"spaces"`
		Documents []KnowledgeDocument `json:"documents"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("Unmarshal migrated envelope: %v", err)
	}
	if len(envelope.Spaces) != 1 || envelope.Spaces[0].ID != DefaultKnowledgeSpaceID {
		t.Fatalf("spaces = %#v, want default space", envelope.Spaces)
	}
	if len(envelope.Documents) != 1 || envelope.Documents[0].SpaceID != DefaultKnowledgeSpaceID {
		t.Fatalf("documents = %#v, want legacy doc assigned to default space", envelope.Documents)
	}
}
