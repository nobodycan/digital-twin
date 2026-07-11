package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/observability"
)

func TestHandlerKnowledgeRepairPromotionPostAndList(t *testing.T) {
	knowledgeStore := admin.NewInMemoryKnowledgeStore()
	knowledgeService := admin.NewKnowledgeService(knowledgeStore)
	if _, err := knowledgeService.Upload("tenant-a", admin.KnowledgeUpload{ID: "doc-a", Name: "Start", Content: "How do I start? Run the app.", ReviewStatus: admin.KnowledgeReviewActive}); err != nil {
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
	attempt, err := verification.Verify(context.Background(), "tenant-a", gap.ID)
	if err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	promotionStore := admin.NewInMemoryRepairEvalPromotionStore()
	promotion := admin.NewRepairEvalPromotionService(promotionStore, gapService, knowledgeService, verification, nil)
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), PromotionAdmin: &promotion, PromotionStore: promotionStore, DefaultTenantID: "tenant-a"})

	post := httptest.NewRecorder()
	body := `{"gap_id":"` + gap.ID + `","verification_attempt_id":"` + attempt.ID + `","minimum_support_state":"grounded","required_document_ids":["doc-a"],"promoted_by":"operator-a"}`
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/admin/knowledge/repairs/promotions", strings.NewReader(body)))
	if post.Code != http.StatusOK || !strings.Contains(post.Body.String(), `"revision":1`) || !strings.Contains(post.Body.String(), `"applied":true`) {
		t.Fatalf("post status = %d, body = %s", post.Code, post.Body.String())
	}

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/admin/knowledge/repairs/promotions?gap_id="+gap.ID+"&active_only=true", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"case_id":"repair-eval-tenant-a-`+gap.ID) {
		t.Fatalf("list status = %d, body = %s", list.Code, list.Body.String())
	}
}
