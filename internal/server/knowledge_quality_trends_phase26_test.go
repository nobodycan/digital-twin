package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/observability"
)

func TestHandlerKnowledgeQualityTrendsDerivesTenantAndValidatesDates(t *testing.T) {
	gapService := admin.NewKnowledgeGapService(admin.NewInMemoryKnowledgeGapStore())
	knowledgeService := admin.NewKnowledgeService(admin.NewInMemoryKnowledgeStore())
	verification := admin.NewRepairVerificationService(admin.NewInMemoryRepairVerificationStore(), gapService, knowledgeService, nil)
	recurrence := admin.NewInMemoryRepairRecurrenceStore()
	trends := admin.NewQualityTrendService(admin.QualityTrendDependencies{
		Gaps: gapService, Knowledge: knowledgeService, Verification: verification, Recurrences: recurrence,
		Observations: admin.NewInMemoryRepairEvalObservationStore(),
	})
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), QualityTrendsAdmin: &trends, DefaultTenantID: "tenant-a"})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/knowledge/quality-trends?from=2026-07-01&to=2026-07-11&space_id=default", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"space_id":"default"`) || !strings.Contains(response.Body.String(), `"verification"`) {
		t.Fatalf("body = %s", response.Body.String())
	}

	badResponse := httptest.NewRecorder()
	handler.ServeHTTP(badResponse, httptest.NewRequest(http.MethodGet, "/admin/knowledge/quality-trends?from=2026-01-01&to=2026-04-02", nil))
	if badResponse.Code != http.StatusBadRequest || !strings.Contains(badResponse.Body.String(), "invalid_quality_trend_filter") {
		t.Fatalf("bad response = %d, body = %s", badResponse.Code, badResponse.Body.String())
	}
}

func TestHandlerKnowledgeQualityTrendsReturnsUnavailableWhenNotConfigured(t *testing.T) {
	handler := NewHandler(Config{Metrics: observability.NewMemoryMetrics(), DefaultTenantID: "tenant-a"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/knowledge/quality-trends", nil))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "knowledge_quality_trends_unavailable") {
		t.Fatalf("response = %d, body = %s", response.Code, response.Body.String())
	}
}
