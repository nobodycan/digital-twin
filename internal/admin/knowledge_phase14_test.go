package admin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKnowledgeServiceHealthSummaryEmptySpace(t *testing.T) {
	service := NewKnowledgeService(NewInMemoryKnowledgeStore())

	summary, err := service.HealthSummary("tenant-1", DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("HealthSummary returned error: %v", err)
	}
	if summary.Status != KnowledgeHealthEmpty {
		t.Fatalf("status = %q, want %q", summary.Status, KnowledgeHealthEmpty)
	}
	if summary.ActiveDocumentCount != 0 || summary.ChunkCount != 0 {
		t.Fatalf("summary = %#v, want zero counts", summary)
	}
	if len(summary.AttentionReasons) == 0 || summary.AttentionReasons[0] != "no_active_documents" {
		t.Fatalf("attention reasons = %#v, want no_active_documents", summary.AttentionReasons)
	}
}

func TestKnowledgeServiceHealthSummaryNeedsAttention(t *testing.T) {
	store := NewInMemoryKnowledgeStore()
	service := NewKnowledgeService(store)

	if _, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-ready",
		Name:    "ready.md",
		Content: "ready content",
	}); err != nil {
		t.Fatalf("Upload(ready) returned error: %v", err)
	}
	if _, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-failed",
		Name:    "failed.md",
		Content: "failed content",
	}); err != nil {
		t.Fatalf("Upload(failed) returned error: %v", err)
	}
	failed, err := service.Get("tenant-1", "kb-failed")
	if err != nil {
		t.Fatalf("Get(failed) returned error: %v", err)
	}
	failed.Status = KnowledgeFailed
	applyIndexMetadata(&failed, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), KnowledgeVectorFailed, "embed_failed")
	if _, err := store.SaveKnowledge(failed); err != nil {
		t.Fatalf("SaveKnowledge(failed) returned error: %v", err)
	}

	summary, err := service.HealthSummary("tenant-1", DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("HealthSummary returned error: %v", err)
	}
	if summary.Status != KnowledgeHealthNeedsAttention {
		t.Fatalf("status = %q, want %q", summary.Status, KnowledgeHealthNeedsAttention)
	}
	if summary.ActiveDocumentCount != 1 || summary.FailedDocumentCount != 1 {
		t.Fatalf("summary counts = %#v", summary)
	}
	if summary.ChunkCount != 2 {
		t.Fatalf("chunk count = %d, want 2", summary.ChunkCount)
	}
	if !containsString(summary.AttentionReasons, "failed_documents_present") {
		t.Fatalf("attention reasons = %#v, want failed_documents_present", summary.AttentionReasons)
	}
}

func TestKnowledgeServiceDocumentDetailFlagsQualitySignals(t *testing.T) {
	store := NewInMemoryKnowledgeStore()
	service := NewKnowledgeService(store)

	for _, upload := range []KnowledgeUpload{
		{ID: "kb-a", Name: "a.md", Content: "shared content"},
		{ID: "kb-b", Name: "b.md", Content: "shared content"},
	} {
		if _, err := service.Upload("tenant-1", upload); err != nil {
			t.Fatalf("Upload(%s) returned error: %v", upload.ID, err)
		}
	}

	document, err := service.Get("tenant-1", "kb-a")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	document.Status = KnowledgeDisabled
	applyIndexMetadata(&document, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), KnowledgeVectorFailed, "embed_failed")
	if _, err := store.SaveKnowledge(document); err != nil {
		t.Fatalf("SaveKnowledge returned error: %v", err)
	}

	detail, err := service.DocumentDetail("tenant-1", "kb-a")
	if err != nil {
		t.Fatalf("DocumentDetail returned error: %v", err)
	}
	for _, want := range []string{"disabled", "index_failed", "duplicate_content_hash", "last_error_present"} {
		if !containsString(detail.QualityFlags, want) {
			t.Fatalf("quality flags = %#v, want %q", detail.QualityFlags, want)
		}
	}
	if detail.Document.ID != "kb-a" {
		t.Fatalf("document id = %q, want kb-a", detail.Document.ID)
	}
}

func TestKnowledgeServiceDocumentDetailFlagsMissingSourceLabelShortAndReviewGated(t *testing.T) {
	store := NewInMemoryKnowledgeStore()
	service := NewKnowledgeService(store)

	document, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-short",
		Name:    "short.md",
		Content: "tiny",
		Metadata: map[string]string{
			"source_type": "workbench_note",
		},
		ReviewStatus: KnowledgeReviewPending,
	})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	detail, err := service.DocumentDetail("tenant-1", document.ID)
	if err != nil {
		t.Fatalf("DocumentDetail returned error: %v", err)
	}
	for _, want := range []string{"review_gated", "missing_source_label", "short_content"} {
		if !containsString(detail.QualityFlags, want) {
			t.Fatalf("quality flags = %#v, want %q", detail.QualityFlags, want)
		}
	}
}

func TestKnowledgeServiceDocumentDetailIncludesSourceAndResolvedGapRelations(t *testing.T) {
	knowledgeStore := NewInMemoryKnowledgeStore()
	knowledgeService := NewKnowledgeService(knowledgeStore)
	gapService := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())

	document, err := knowledgeService.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-note",
		Name:    "note.md",
		Content: "Source text for the refund policy.",
		Metadata: map[string]string{
			"source_type":   "workbench_note",
			"source_gap_id": "gap-source",
			"created_from":  "knowledge_workbench",
		},
	})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	sourceGap, err := gapService.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        document.SpaceID,
		Question:       "What is the refund policy?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("Create(source gap) returned error: %v", err)
	}
	sourceGap.ID = "gap-source"
	if _, err := gapService.store.SaveKnowledgeGap(sourceGap); err != nil {
		t.Fatalf("SaveKnowledgeGap(source gap) returned error: %v", err)
	}

	resolvedGap, err := gapService.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        document.SpaceID,
		Question:       "How long do refunds take?",
		NoSourceReason: "below_threshold",
	})
	if err != nil {
		t.Fatalf("Create(resolved gap) returned error: %v", err)
	}
	if _, err := gapService.UpdateStatus("tenant-1", resolvedGap.ID, KnowledgeGapResolved, document.ID, "Covered by the note."); err != nil {
		t.Fatalf("UpdateStatus(resolved) returned error: %v", err)
	}

	detail, err := knowledgeService.DocumentDetailWithRelations("tenant-1", document.ID, gapService)
	if err != nil {
		t.Fatalf("DocumentDetailWithRelations returned error: %v", err)
	}
	if detail.Relations.SourceGap == nil || detail.Relations.SourceGap.ID != "gap-source" {
		t.Fatalf("source gap = %#v, want gap-source", detail.Relations.SourceGap)
	}
	if len(detail.Relations.ResolvedGaps) != 1 || detail.Relations.ResolvedGaps[0].ID != resolvedGap.ID {
		t.Fatalf("resolved gaps = %#v, want resolved gap %q", detail.Relations.ResolvedGaps, resolvedGap.ID)
	}
}

func TestKnowledgeServiceDocumentDetailWithRelationsWorksWithoutGapService(t *testing.T) {
	service := NewKnowledgeService(NewInMemoryKnowledgeStore())
	document, err := service.Upload("tenant-1", KnowledgeUpload{
		ID:      "kb-note",
		Name:    "note.md",
		Content: "Standalone source text.",
	})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	detail, err := service.DocumentDetailWithRelations("tenant-1", document.ID, KnowledgeGapService{})
	if err != nil {
		t.Fatalf("DocumentDetailWithRelations returned error: %v", err)
	}
	if detail.Document.ID != document.ID {
		t.Fatalf("document id = %q, want %q", detail.Document.ID, document.ID)
	}
	if detail.Relations.SourceGap != nil {
		t.Fatalf("source gap = %#v, want nil", detail.Relations.SourceGap)
	}
	if len(detail.Relations.ResolvedGaps) != 0 {
		t.Fatalf("resolved gaps = %#v, want none", detail.Relations.ResolvedGaps)
	}
}

func TestKnowledgeGapServiceLifecycleWithFileStore(t *testing.T) {
	dir := t.TempDir()
	service := NewKnowledgeGapService(NewFileKnowledgeGapStore(dir))

	created, err := service.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "What is our refund window?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Status != KnowledgeGapOpen {
		t.Fatalf("status = %q, want %q", created.Status, KnowledgeGapOpen)
	}

	listed, err := service.List("tenant-1", DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed = %#v, want one created gap", listed)
	}

	updated, err := service.UpdateStatus("tenant-1", created.ID, KnowledgeGapResolved, "kb-policy", "")
	if err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	if updated.Status != KnowledgeGapResolved || updated.ResolvedByDocumentID != "kb-policy" {
		t.Fatalf("updated = %#v", updated)
	}
	if updated.ResolutionNote != "" {
		t.Fatalf("resolution note = %q, want empty", updated.ResolutionNote)
	}

	reopened := NewKnowledgeGapService(NewFileKnowledgeGapStore(dir))
	reloaded, err := reopened.List("tenant-1", DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("List after reopen returned error: %v", err)
	}
	if len(reloaded) != 1 || reloaded[0].Status != KnowledgeGapResolved {
		t.Fatalf("reloaded = %#v", reloaded)
	}
}

func TestKnowledgeGapServiceSupportsInvestigatingAndResolutionNote(t *testing.T) {
	service := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	created, err := service.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "How do we run smoke tests?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	investigating, err := service.UpdateStatus("tenant-1", created.ID, KnowledgeGapInvestigating, "", "")
	if err != nil {
		t.Fatalf("UpdateStatus(investigating) returned error: %v", err)
	}
	if investigating.Status != KnowledgeGapInvestigating {
		t.Fatalf("status = %q, want %q", investigating.Status, KnowledgeGapInvestigating)
	}
	if investigating.ResolvedByDocumentID != "" || investigating.ResolutionNote != "" {
		t.Fatalf("investigating gap = %#v, want empty resolution fields", investigating)
	}

	resolved, err := service.UpdateStatus("tenant-1", created.ID, KnowledgeGapResolved, "kb-smoke", "Covered by the smoke checklist note.")
	if err != nil {
		t.Fatalf("UpdateStatus(resolved) returned error: %v", err)
	}
	if resolved.Status != KnowledgeGapResolved {
		t.Fatalf("status = %q, want %q", resolved.Status, KnowledgeGapResolved)
	}
	if resolved.ResolvedByDocumentID != "kb-smoke" {
		t.Fatalf("resolved_by_document_id = %q, want kb-smoke", resolved.ResolvedByDocumentID)
	}
	if resolved.ResolutionNote != "Covered by the smoke checklist note." {
		t.Fatalf("resolution_note = %q", resolved.ResolutionNote)
	}
}

func TestKnowledgeGapServiceClearsResolutionFieldsWhenLeavingResolved(t *testing.T) {
	service := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	created, err := service.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "What is our deployment flow?",
		NoSourceReason: "below_threshold",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := service.UpdateStatus("tenant-1", created.ID, KnowledgeGapResolved, "kb-deploy", "Resolved by deploy note."); err != nil {
		t.Fatalf("UpdateStatus(resolved) returned error: %v", err)
	}

	reopened, err := service.UpdateStatus("tenant-1", created.ID, KnowledgeGapOpen, "", "")
	if err != nil {
		t.Fatalf("UpdateStatus(open) returned error: %v", err)
	}
	if reopened.ResolvedByDocumentID != "" || reopened.ResolutionNote != "" {
		t.Fatalf("reopened gap = %#v, want cleared resolution fields", reopened)
	}
}

func TestKnowledgeGapServiceRejectsInvalidStatusIncludingUnknownWorkflowState(t *testing.T) {
	service := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	created, err := service.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "How does pricing work?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := service.UpdateStatus("tenant-1", created.ID, KnowledgeGapStatus("triaged"), "", ""); err == nil {
		t.Fatalf("UpdateStatus accepted invalid status")
	}
}

func TestKnowledgeGapServiceDedupesOpenGapByQuestionAndReason(t *testing.T) {
	service := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	times := []time.Time{
		time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 10, 0, 1, 0, time.UTC),
	}
	service.now = func() time.Time {
		current := times[0]
		if len(times) > 1 {
			times = times[1:]
		}
		return current
	}

	first, err := service.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "What is our refund window?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("first Create returned error: %v", err)
	}
	second, err := service.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "What is our refund window?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("second Create returned error: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("second id = %q, want %q", second.ID, first.ID)
	}
	listed, err := service.List("tenant-1", DefaultKnowledgeSpaceID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("gap count = %d, want 1", len(listed))
	}
}

func TestKnowledgeGapServiceGeneratesUniqueIDsWhenClockRepeats(t *testing.T) {
	service := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	fixed := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }
	first, err := service.Create("tenant-1", KnowledgeGapInput{SpaceID: DefaultKnowledgeSpaceID, Question: "First?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("first create returned error: %v", err)
	}
	if _, err := service.UpdateStatus("tenant-1", first.ID, KnowledgeGapResolved, "doc-1", "covered"); err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	second, err := service.Create("tenant-1", KnowledgeGapInput{SpaceID: DefaultKnowledgeSpaceID, Question: "Second?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("second create returned error: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("gap IDs collided: %q", first.ID)
	}
}

func TestFileKnowledgeGapStoreLeavesNoTemporaryFilesBehind(t *testing.T) {
	dir := t.TempDir()
	service := NewKnowledgeGapService(NewFileKnowledgeGapStore(dir))

	if _, err := service.Create("tenant-1", KnowledgeGapInput{
		SpaceID:        DefaultKnowledgeSpaceID,
		Question:       "Why is pricing missing?",
		NoSourceReason: "no_matching_chunks",
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}

	data, err := os.ReadFile(filepath.Join(dir, "knowledge_gaps.json"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var decoded []KnowledgeGap
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("gap count = %d, want 1", len(decoded))
	}
}

func TestKnowledgeGapServiceListsAllTenantSpacesForTrends(t *testing.T) {
	store := NewInMemoryKnowledgeGapStore()
	service := NewKnowledgeGapService(store)
	for _, gap := range []KnowledgeGap{
		{ID: "gap-default", TenantID: "tenant-a", SpaceID: DefaultKnowledgeSpaceID, Question: "default", NoSourceReason: "missing", Status: KnowledgeGapResolved},
		{ID: "gap-ops", TenantID: "tenant-a", SpaceID: "ops", Question: "ops", NoSourceReason: "missing", Status: KnowledgeGapResolved},
		{ID: "gap-other", TenantID: "tenant-b", SpaceID: "ops", Question: "other", NoSourceReason: "missing", Status: KnowledgeGapResolved},
	} {
		if _, err := store.SaveKnowledgeGap(gap); err != nil {
			t.Fatalf("save gap %s returned error: %v", gap.ID, err)
		}
	}
	items, err := service.ListAll("tenant-a")
	if err != nil || len(items) != 2 || items[0].TenantID != "tenant-a" || items[1].TenantID != "tenant-a" {
		t.Fatalf("all tenant gaps = %#v, err=%v", items, err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
