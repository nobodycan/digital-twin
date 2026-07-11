package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/observability"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestHandlerKnowledgeRecurrenceListAndDismiss(t *testing.T) {
	gapService, recurrenceService, gap, recurrence := recurrenceHandlerFixture(t)
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), KnowledgeGapAdmin: &gapService, RecurrenceAdmin: &recurrenceService, DefaultTenantID: "tenant-a"})

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/admin/knowledge/repairs/recurrences?gap_id="+gap.ID+"&limit=20", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), recurrence.ID) {
		t.Fatalf("list status = %d, body = %s", list.Code, list.Body.String())
	}
	all := httptest.NewRecorder()
	handler.ServeHTTP(all, httptest.NewRequest(http.MethodGet, "/admin/knowledge/repairs/recurrences", nil))
	if all.Code != http.StatusOK || !strings.Contains(all.Body.String(), recurrence.ID) {
		t.Fatalf("all list status = %d, body = %s", all.Code, all.Body.String())
	}

	dismiss := httptest.NewRecorder()
	handler.ServeHTTP(dismiss, httptest.NewRequest(http.MethodPost, "/admin/knowledge/repairs/recurrences/dismiss", strings.NewReader(`{"recurrence_id":"`+recurrence.ID+`","reason":"known transient issue"}`)))
	if dismiss.Code != http.StatusOK || !strings.Contains(dismiss.Body.String(), `"status":"dismissed"`) {
		t.Fatalf("dismiss status = %d, body = %s", dismiss.Code, dismiss.Body.String())
	}
}

func TestHandlerKnowledgeRecurrenceConfirmReopensGap(t *testing.T) {
	gapService, recurrenceService, gap, recurrence := recurrenceHandlerFixture(t)
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), KnowledgeGapAdmin: &gapService, RecurrenceAdmin: &recurrenceService, DefaultTenantID: "tenant-a"})
	confirm := httptest.NewRecorder()
	handler.ServeHTTP(confirm, httptest.NewRequest(http.MethodPost, "/admin/knowledge/repairs/recurrences/confirm", strings.NewReader(`{"recurrence_id":"`+recurrence.ID+`","confirmed_by":"operator"}`)))
	if confirm.Code != http.StatusOK || !strings.Contains(confirm.Body.String(), `"status":"confirmed"`) {
		t.Fatalf("confirm status = %d, body = %s", confirm.Code, confirm.Body.String())
	}
	reopened, err := gapService.Get("tenant-a", gap.ID)
	if err != nil || reopened.Status != admin.KnowledgeGapOpen {
		t.Fatalf("gap = %#v, err=%v", reopened, err)
	}
}

func TestHandlerKnowledgeRecurrenceRejectsInvalidDismissAndMissingService(t *testing.T) {
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics()})
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/admin/knowledge/repairs/recurrences?gap_id=gap-1", nil))
	if missing.Code != http.StatusServiceUnavailable || !strings.Contains(missing.Body.String(), "knowledge_recurrence_unavailable") {
		t.Fatalf("missing status = %d, body = %s", missing.Code, missing.Body.String())
	}
	gapService, recurrenceService, _, recurrence := recurrenceHandlerFixture(t)
	handler = NewHandler(Config{Metrics: observability.NewMemoryMetrics(), KnowledgeGapAdmin: &gapService, RecurrenceAdmin: &recurrenceService, DefaultTenantID: "tenant-a"})
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/admin/knowledge/repairs/recurrences/dismiss", strings.NewReader(`{"recurrence_id":"`+recurrence.ID+""+`"}`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalid.Code, invalid.Body.String())
	}
}

func TestHandlerRecurrenceDetectionSkipsLegacyGapCaptureWhenHandled(t *testing.T) {
	gapService, recurrenceService, gap, _ := recurrenceHandlerFixture(t)
	handler := &Handler{knowledgeGapAdmin: &gapService, recurrenceAdmin: &recurrenceService}
	conversation := types.Conversation{TenantID: "tenant-a", Messages: []types.Message{{Role: types.RoleUser, Content: gap.Question}}}
	result := types.AgentResult{Metadata: types.Metadata{
		"knowledge_answer_state":     "unsupported",
		"knowledge_no_source_reason": gap.NoSourceReason,
		"knowledge_space_id":         gap.SpaceID,
	}}
	handler.captureKnowledgeGapAfterAudit(conversation, result, admin.AuditRecord{ID: "audit-hook", QuestionSummary: gap.Question, KnowledgeSpaceID: gap.SpaceID, KnowledgeAnswerState: "unsupported", KnowledgeNoSourceReason: gap.NoSourceReason, CreatedAt: time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)})
	gaps, err := gapService.List("tenant-a", gap.SpaceID)
	if err != nil || len(gaps) != 1 || gaps[0].Status != admin.KnowledgeGapResolved {
		t.Fatalf("gaps = %#v, err=%v", gaps, err)
	}
}

func recurrenceHandlerFixture(t *testing.T) (admin.KnowledgeGapService, admin.RepairRecurrenceService, admin.KnowledgeGap, admin.RepairRecurrence) {
	t.Helper()
	knowledgeStore := admin.NewInMemoryKnowledgeStore()
	knowledgeService := admin.NewKnowledgeService(knowledgeStore)
	if _, err := knowledgeService.Upload("tenant-a", admin.KnowledgeUpload{ID: "doc-a", Name: "Repair", Content: "How do I start? Run the app.", ReviewStatus: admin.KnowledgeReviewActive}); err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	gapService := admin.NewKnowledgeGapService(admin.NewInMemoryKnowledgeGapStore())
	gap, err := gapService.Create("tenant-a", admin.KnowledgeGapInput{SpaceID: admin.DefaultKnowledgeSpaceID, Question: "How do I start?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("create gap returned error: %v", err)
	}
	if _, err := gapService.UpdateStatus("tenant-a", gap.ID, admin.KnowledgeGapResolved, "doc-a", "covered"); err != nil {
		t.Fatalf("resolve gap returned error: %v", err)
	}
	verification := admin.NewRepairVerificationService(admin.NewInMemoryRepairVerificationStore(), gapService, knowledgeService, func(context.Context, string, admin.RepairVerificationDiagnosticRequest) (admin.RepairVerificationDiagnosticResponse, error) {
		return admin.RepairVerificationDiagnosticResponse{Results: []admin.RepairVerificationDiagnosticResult{{DocumentID: "doc-a", ChunkID: "doc-a-0", Rank: 1, ReviewStatus: string(admin.KnowledgeReviewActive)}}}, nil
	})
	if _, err := verification.Verify(context.Background(), "tenant-a", gap.ID); err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	recurrence := admin.NewRepairRecurrenceService(admin.NewInMemoryRepairRecurrenceStore(), gapService, verification)
	record, handled, err := recurrence.Detect("tenant-a", admin.AuditRecord{ID: "audit-handler", QuestionSummary: gap.Question, KnowledgeSpaceID: gap.SpaceID, KnowledgeAnswerState: "unsupported", KnowledgeNoSourceReason: gap.NoSourceReason})
	if err != nil || !handled {
		t.Fatalf("detect record = %#v, handled=%v, err=%v", record, handled, err)
	}
	return gapService, recurrence, gap, record
}
