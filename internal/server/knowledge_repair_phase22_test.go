package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/knowledge"
	"github.com/nobodycan/digital-twin/internal/observability"
)

func TestHandlerKnowledgeRepairListReturnsProjectedItems(t *testing.T) {
	knowledgeStore := admin.NewInMemoryKnowledgeStore()
	knowledgeService := admin.NewKnowledgeService(knowledgeStore)
	gapService := admin.NewKnowledgeGapService(admin.NewInMemoryKnowledgeGapStore())
	auditService := admin.NewAuditService(admin.NewInMemoryAuditStore())
	now := time.Date(2026, 7, 9, 14, 0, 0, 0, time.UTC)
	if _, err := gapService.Create("tenant-1", admin.KnowledgeGapInput{
		SpaceID:        admin.DefaultKnowledgeSpaceID,
		Question:       "How should I verify DeepSeek startup?",
		NoSourceReason: "no_matching_chunks",
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	gaps, err := gapService.List("tenant-1", admin.DefaultKnowledgeSpaceID)
	if err != nil || len(gaps) != 1 {
		t.Fatalf("seed gaps = %#v, err = %v", gaps, err)
	}
	if _, err := auditService.Record("tenant-1", admin.AuditRecord{
		ID:                      "audit-repair-list",
		ConversationID:          "conv-repair-list",
		Status:                  admin.AuditStatusCompleted,
		QuestionSummary:         gaps[0].Question,
		KnowledgeSpaceID:        gaps[0].SpaceID,
		KnowledgeAnswerState:    "unsupported",
		KnowledgeNoSourceReason: gaps[0].NoSourceReason,
		CreatedAt:               now,
		KnowledgeEvidence: map[string]any{
			"answer_state": "unsupported",
			"summary":      "No supporting evidence recorded.",
			"gaps": []map[string]any{
				{"gap_id": gaps[0].ID, "status": "open", "reason": gaps[0].NoSourceReason},
			},
		},
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	handler := NewHandler(Config{
		Metrics:           observability.NewMemoryMetrics(),
		KnowledgeAdmin:    &knowledgeService,
		KnowledgeGapAdmin: &gapService,
		AuditAdmin:        &auditService,
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/knowledge/repairs?space_id=default&weak_only=true&unresolved_only=true&limit=10", nil)
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{`"gap_id":"` + gaps[0].ID + `"`, `"priority":"high"`, `"occurrence_count":1`, `"answer_state":"unsupported"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerKnowledgeRepairRetestReturnsImprovedSupportState(t *testing.T) {
	knowledgeStore := admin.NewInMemoryKnowledgeStore()
	knowledgeService := admin.NewKnowledgeService(knowledgeStore)
	retriever := knowledge.NewService(knowledgeStore)
	gapService := admin.NewKnowledgeGapService(admin.NewInMemoryKnowledgeGapStore())
	auditService := admin.NewAuditService(admin.NewInMemoryAuditStore())
	gap, err := gapService.Create("tenant-1", admin.KnowledgeGapInput{
		SpaceID:        admin.DefaultKnowledgeSpaceID,
		Question:       "How should I verify DeepSeek startup?",
		NoSourceReason: "no_matching_chunks",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := knowledgeService.Upload("tenant-1", admin.KnowledgeUpload{
		ID:      "kb-startup",
		Name:    "Startup Playbook",
		Content: "How should I verify DeepSeek startup?\nRun the smoke conversation script after boot.",
		Metadata: map[string]string{
			"source_gap_id": gap.ID,
			"source_type":   "workbench_note",
		},
	}); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if _, err := auditService.Record("tenant-1", admin.AuditRecord{
		ID:                      "audit-before-retest",
		ConversationID:          "conv-before-retest",
		Status:                  admin.AuditStatusCompleted,
		QuestionSummary:         gap.Question,
		KnowledgeSpaceID:        gap.SpaceID,
		KnowledgeAnswerState:    "unsupported",
		KnowledgeNoSourceReason: gap.NoSourceReason,
		CreatedAt:               time.Date(2026, 7, 9, 14, 10, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	handler := NewHandler(Config{
		Metrics:            observability.NewMemoryMetrics(),
		KnowledgeAdmin:     &knowledgeService,
		KnowledgeGapAdmin:  &gapService,
		KnowledgeRetriever: &retriever,
		AuditAdmin:         &auditService,
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/knowledge/repairs/retest", strings.NewReader(`{"gap_id":"`+gap.ID+`"}`))
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{`"gap_id":"` + gap.ID + `"`, `"before_state":"unsupported"`, `"after_state":"grounded"`, `"document_id":"kb-startup"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestHandlerKnowledgeRepairRetestRejectsInvalidLimitAndMissingServices(t *testing.T) {
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics()})

	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/admin/knowledge/repairs?limit=0", nil))
	if listResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("list status = %d, want 503; body = %s", listResponse.Code, listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), `knowledge_repair_admin_unavailable`) {
		t.Fatalf("list body = %s", listResponse.Body.String())
	}

	retestResponse := httptest.NewRecorder()
	handler.ServeHTTP(retestResponse, httptest.NewRequest(http.MethodPost, "/admin/knowledge/repairs/retest", strings.NewReader(`{"gap_id":"gap-1"}`)))
	if retestResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("retest status = %d, want 503; body = %s", retestResponse.Code, retestResponse.Body.String())
	}
	if !strings.Contains(retestResponse.Body.String(), `knowledge_repair_retest_unavailable`) {
		t.Fatalf("retest body = %s", retestResponse.Body.String())
	}
}

func TestHandlerKnowledgeRepairVerifyPersistsAttempt(t *testing.T) {
	knowledgeStore := admin.NewInMemoryKnowledgeStore()
	knowledgeService := admin.NewKnowledgeService(knowledgeStore)
	gapService := admin.NewKnowledgeGapService(admin.NewInMemoryKnowledgeGapStore())
	gap, err := gapService.Create("tenant-1", admin.KnowledgeGapInput{SpaceID: admin.DefaultKnowledgeSpaceID, Question: "How do I start?", NoSourceReason: "no_matching_chunks"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := knowledgeService.Upload("tenant-1", admin.KnowledgeUpload{ID: "doc-start", Name: "Start", Content: "How do I start? Run the app.", ReviewStatus: admin.KnowledgeReviewActive}); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	verification := admin.NewRepairVerificationService(admin.NewInMemoryRepairVerificationStore(), gapService, knowledgeService, func(context.Context, string, admin.RepairVerificationDiagnosticRequest) (admin.RepairVerificationDiagnosticResponse, error) {
		return admin.RepairVerificationDiagnosticResponse{Results: []admin.RepairVerificationDiagnosticResult{{DocumentID: "doc-start", DocumentName: "Start", ReviewStatus: string(admin.KnowledgeReviewActive), SourceType: "text", ChunkID: "doc-start-0", Rank: 1}}}, nil
	})
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), KnowledgeAdmin: &knowledgeService, KnowledgeGapAdmin: &gapService, VerificationAdmin: &verification})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/knowledge/repairs/verify", strings.NewReader(`{"gap_id":"`+gap.ID+`"}`)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"result":"passed"`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
