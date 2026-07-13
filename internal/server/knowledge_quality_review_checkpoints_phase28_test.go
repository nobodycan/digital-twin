package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/observability"
)

func TestHandlerQualityReviewCheckpointCreatesAndReplays(t *testing.T) {
	checkpointService := qualityReviewCheckpointServiceForHandlerTest(t)
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), QualityReviewsAdmin: &checkpointService, DefaultTenantID: "tenant-a"})
	body := `{"idempotency_key":"checkpoint-request-123","from":"2026-07-10","to":"2026-07-12","outcome":"observe"}`

	created := httptest.NewRecorder()
	handler.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/admin/knowledge/quality-review-checkpoints", strings.NewReader(body)))
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"created":true`) || !strings.Contains(created.Body.String(), `"id":"checkpoint-1"`) {
		t.Fatalf("create response = %d, body=%s", created.Code, created.Body.String())
	}

	replayed := httptest.NewRecorder()
	handler.ServeHTTP(replayed, httptest.NewRequest(http.MethodPost, "/admin/knowledge/quality-review-checkpoints", strings.NewReader(body)))
	if replayed.Code != http.StatusOK || !strings.Contains(replayed.Body.String(), `"created":false`) || !strings.Contains(replayed.Body.String(), `"id":"checkpoint-1"`) {
		t.Fatalf("replay response = %d, body=%s", replayed.Code, replayed.Body.String())
	}
}

func TestHandlerQualityReviewCheckpointListsExactScope(t *testing.T) {
	checkpointService := qualityReviewCheckpointServiceForHandlerTest(t)
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), QualityReviewsAdmin: &checkpointService, DefaultTenantID: "tenant-a"})
	body := `{"idempotency_key":"checkpoint-request-789","from":"2026-07-10","to":"2026-07-12","outcome":"observe"}`
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/admin/knowledge/quality-review-checkpoints", strings.NewReader(body)))
	if created.Code != http.StatusCreated {
		t.Fatalf("create response = %d, body=%s", created.Code, created.Body.String())
	}

	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/admin/knowledge/quality-review-checkpoints?limit=20", nil))
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"checkpoints":[`) || !strings.Contains(listed.Body.String(), `"id":"checkpoint-1"`) {
		t.Fatalf("list response = %d, body=%s", listed.Code, listed.Body.String())
	}
}

func TestHandlerQualityReviewCheckpointRejectsOversizedRequestAndIdempotencyConflict(t *testing.T) {
	checkpointService := qualityReviewCheckpointServiceForHandlerTest(t)
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), QualityReviewsAdmin: &checkpointService, DefaultTenantID: "tenant-a"})
	oversized := httptest.NewRecorder()
	handler.ServeHTTP(oversized, httptest.NewRequest(http.MethodPost, "/admin/knowledge/quality-review-checkpoints", strings.NewReader(`{"rationale":"`+strings.Repeat("x", 17<<10)+`"}`)))
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, body=%s", oversized.Code, oversized.Body.String())
	}

	body := `{"idempotency_key":"checkpoint-http-conflict-001","from":"2026-07-10","to":"2026-07-12","outcome":"observe","rationale":"first"}`
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/admin/knowledge/quality-review-checkpoints", strings.NewReader(body)))
	conflict := httptest.NewRecorder()
	handler.ServeHTTP(conflict, httptest.NewRequest(http.MethodPost, "/admin/knowledge/quality-review-checkpoints", strings.NewReader(strings.Replace(body, `"first"`, `"changed"`, 1))))
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "quality_review_idempotency_conflict") {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}

func qualityReviewCheckpointServiceForHandlerTest(t *testing.T) admin.QualityReviewCheckpointService {
	t.Helper()
	gapService := admin.NewKnowledgeGapService(admin.NewInMemoryKnowledgeGapStore())
	knowledgeService := admin.NewKnowledgeService(admin.NewInMemoryKnowledgeStore())
	verification := admin.NewRepairVerificationService(admin.NewInMemoryRepairVerificationStore(), gapService, knowledgeService, nil)
	trends := admin.NewQualityTrendService(admin.QualityTrendDependencies{
		Gaps: gapService, Knowledge: knowledgeService, Verification: verification,
		Recurrences: admin.NewInMemoryRepairRecurrenceStore(), Observations: admin.NewInMemoryRepairEvalObservationStore(),
		Now: func() time.Time { return time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC) }, Location: time.UTC,
	})
	return admin.NewQualityReviewCheckpointService(admin.QualityReviewCheckpointDependencies{
		Trends: trends, Gaps: gapService, Store: admin.NewInMemoryQualityReviewCheckpointStore(),
		Now:   func() time.Time { return time.Date(2026, 7, 13, 12, 1, 0, 0, time.UTC) },
		NewID: func() (string, error) { return "checkpoint-1", nil }, Location: time.UTC,
	})
}
