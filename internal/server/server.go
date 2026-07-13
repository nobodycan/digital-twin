package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/knowledge"
	"github.com/nobodycan/digital-twin/internal/observability"
	"github.com/nobodycan/digital-twin/internal/persona"
	"github.com/nobodycan/digital-twin/internal/presentation"
	"github.com/nobodycan/digital-twin/internal/runtime"
	"github.com/nobodycan/digital-twin/internal/voice"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type Config struct {
	Metrics              observability.Metrics
	Orchestrator         core.Orchestrator
	EventRecorder        *runtime.EventRecorder
	PresentationAdapter  presentation.Adapter
	ASR                  voice.ASRClient
	Readiness            ReadinessConfig
	RuntimeStatus        RuntimeStatus
	PersonaAdmin         *admin.PersonaService
	MemoryAdmin          *admin.MemoryService
	KnowledgeAdmin       *admin.KnowledgeService
	KnowledgeImportAdmin *admin.KnowledgeImportService
	KnowledgeGapAdmin    *admin.KnowledgeGapService
	KnowledgeRetriever   *knowledge.Service
	VerificationAdmin    *admin.RepairVerificationService
	RecurrenceAdmin      *admin.RepairRecurrenceService
	PromotionAdmin       *admin.RepairEvalPromotionService
	PromotionStore       admin.RepairEvalPromotionStore
	QualityTrendsAdmin   *admin.QualityTrendService
	QualityReviewsAdmin  *admin.QualityReviewCheckpointService
	ToolPolicyAdmin      *admin.ToolPolicyService
	AuditAdmin           *admin.AuditService
	StaticDir            string
	APIKeys              []string
	AdminAPIKeys         []string
	AllowAnonymousAdmin  bool
	RateLimitRequests    int
	DefaultTenantID      string
	DefaultUserID        string
}

type ReadinessConfig struct {
	DataDir           string
	ConfigSummary     string
	ConfigError       error
	ReleaseGateStatus string
	Redact            func(string) string
}

type RuntimeStatus struct {
	Environment        string `json:"environment"`
	Provider           string `json:"provider"`
	Model              string `json:"model"`
	FallbackPolicy     string `json:"fallback_policy"`
	GenerationModeHint string `json:"generation_mode_hint"`
	BaseURL            string `json:"base_url,omitempty"`
}

type Handler struct {
	mux                  *http.ServeMux
	metrics              observability.Metrics
	orchestrator         core.Orchestrator
	eventRecorder        *runtime.EventRecorder
	presentationAdapter  presentation.Adapter
	asr                  voice.ASRClient
	readiness            ReadinessConfig
	runtimeStatus        RuntimeStatus
	personaAdmin         *admin.PersonaService
	memoryAdmin          *admin.MemoryService
	knowledgeAdmin       *admin.KnowledgeService
	knowledgeImportAdmin *admin.KnowledgeImportService
	knowledgeGapAdmin    *admin.KnowledgeGapService
	knowledgeRetriever   *knowledge.Service
	verificationAdmin    *admin.RepairVerificationService
	recurrenceAdmin      *admin.RepairRecurrenceService
	promotionAdmin       *admin.RepairEvalPromotionService
	promotionStore       admin.RepairEvalPromotionStore
	qualityTrendsAdmin   *admin.QualityTrendService
	qualityReviewsAdmin  *admin.QualityReviewCheckpointService
	toolPolicyAdmin      *admin.ToolPolicyService
	auditAdmin           *admin.AuditService
	staticDir            string
	apiKeys              []string
	adminAPIKeys         []string
	allowAnonymousAdmin  bool
	rateLimitRequests    int
	defaultTenantID      string
	defaultUserID        string
	mu                   sync.Mutex
	requestCounts        map[string]int
}

func NewHandler(config Config) http.Handler {
	metrics := config.Metrics
	if metrics == nil {
		metrics = observability.NewMemoryMetrics()
	}
	handler := &Handler{
		mux:                  http.NewServeMux(),
		metrics:              metrics,
		orchestrator:         config.Orchestrator,
		eventRecorder:        config.EventRecorder,
		presentationAdapter:  config.PresentationAdapter,
		asr:                  config.ASR,
		readiness:            config.Readiness,
		runtimeStatus:        config.RuntimeStatus,
		personaAdmin:         config.PersonaAdmin,
		memoryAdmin:          config.MemoryAdmin,
		knowledgeAdmin:       config.KnowledgeAdmin,
		knowledgeImportAdmin: config.KnowledgeImportAdmin,
		knowledgeGapAdmin:    config.KnowledgeGapAdmin,
		knowledgeRetriever:   config.KnowledgeRetriever,
		verificationAdmin:    config.VerificationAdmin,
		recurrenceAdmin:      config.RecurrenceAdmin,
		promotionAdmin:       config.PromotionAdmin,
		promotionStore:       config.PromotionStore,
		qualityTrendsAdmin:   config.QualityTrendsAdmin,
		qualityReviewsAdmin:  config.QualityReviewsAdmin,
		toolPolicyAdmin:      config.ToolPolicyAdmin,
		auditAdmin:           config.AuditAdmin,
		staticDir:            config.StaticDir,
		apiKeys:              normalizedKeys(config.APIKeys),
		adminAPIKeys:         normalizedKeys(config.AdminAPIKeys),
		allowAnonymousAdmin:  config.AllowAnonymousAdmin || (len(config.APIKeys) == 0 && len(config.AdminAPIKeys) == 0),
		rateLimitRequests:    config.RateLimitRequests,
		defaultTenantID:      strings.TrimSpace(config.DefaultTenantID),
		defaultUserID:        strings.TrimSpace(config.DefaultUserID),
		requestCounts:        make(map[string]int),
	}
	handler.mux.HandleFunc("GET /health", handler.handleHealth)
	handler.mux.HandleFunc("GET /ready", handler.handleReady)
	handler.mux.HandleFunc("GET /metrics", handler.handleMetrics)
	handler.mux.HandleFunc("GET /runtime/status", handler.handleRuntimeStatus)
	handler.mux.HandleFunc("GET /admin-access", handler.handleAdminAccess)
	handler.mux.HandleFunc("GET /favicon.ico", handler.handleFavicon)
	handler.mux.HandleFunc("GET /app", handler.handleStaticHTML("app.html"))
	handler.mux.HandleFunc("GET /admin", handler.handleStaticHTML("admin.html"))
	if handler.staticDir != "" {
		handler.mux.HandleFunc("GET /web/", handler.handleStaticAsset)
	}
	handler.mux.HandleFunc("POST /chat", handler.handleChat)
	handler.mux.HandleFunc("POST /chat/stream", handler.handleChatStream)
	handler.mux.HandleFunc("POST /experience/stream", handler.handleExperienceStream)
	handler.mux.HandleFunc("POST /experience/mock-voice/stream", handler.handleMockVoiceStream)
	handler.mux.HandleFunc("POST /admin/persona/drafts", handler.handlePersonaDraft)
	handler.mux.HandleFunc("POST /admin/persona/publish", handler.handlePersonaPublish)
	handler.mux.HandleFunc("POST /admin/persona/rollback", handler.handlePersonaRollback)
	handler.mux.HandleFunc("GET /admin/persona/active", handler.handlePersonaActive)
	handler.mux.HandleFunc("GET /admin/memory", handler.handleMemoryList)
	handler.mux.HandleFunc("POST /admin/memory/disable", handler.handleMemoryDisable)
	handler.mux.HandleFunc("GET /admin/knowledge/spaces", handler.handleKnowledgeSpaceList)
	handler.mux.HandleFunc("POST /admin/knowledge/spaces/create", handler.handleKnowledgeSpaceCreate)
	handler.mux.HandleFunc("POST /admin/knowledge/spaces/update", handler.handleKnowledgeSpaceUpdate)
	handler.mux.HandleFunc("POST /admin/knowledge/spaces/disable", handler.handleKnowledgeSpaceDisable)
	handler.mux.HandleFunc("POST /admin/knowledge/spaces/enable", handler.handleKnowledgeSpaceEnable)
	handler.mux.HandleFunc("POST /admin/knowledge/spaces/archive", handler.handleKnowledgeSpaceArchive)
	handler.mux.HandleFunc("GET /admin/knowledge", handler.handleKnowledgeList)
	handler.mux.HandleFunc("GET /admin/knowledge/imports", handler.handleKnowledgeImportList)
	handler.mux.HandleFunc("GET /admin/knowledge/health", handler.handleKnowledgeHealth)
	handler.mux.HandleFunc("GET /admin/knowledge/gaps", handler.handleKnowledgeGapList)
	handler.mux.HandleFunc("GET /admin/knowledge/repairs", handler.handleKnowledgeRepairList)
	handler.mux.HandleFunc("GET /admin/knowledge/{documentID}", handler.handleKnowledgeGet)
	handler.mux.HandleFunc("GET /admin/knowledge/{documentID}/detail", handler.handleKnowledgeDetail)
	handler.mux.HandleFunc("POST /admin/knowledge/upload", handler.handleKnowledgeUpload)
	handler.mux.HandleFunc("POST /admin/knowledge/import", handler.handleKnowledgeImport)
	handler.mux.HandleFunc("POST /admin/knowledge/review", handler.handleKnowledgeReview)
	handler.mux.HandleFunc("POST /admin/knowledge/notes/create", handler.handleKnowledgeNoteCreate)
	handler.mux.HandleFunc("POST /admin/knowledge/disable", handler.handleKnowledgeDisable)
	handler.mux.HandleFunc("POST /admin/knowledge/enable", handler.handleKnowledgeEnable)
	handler.mux.HandleFunc("POST /admin/knowledge/delete", handler.handleKnowledgeDelete)
	handler.mux.HandleFunc("POST /admin/knowledge/update", handler.handleKnowledgeUpdate)
	handler.mux.HandleFunc("POST /admin/knowledge/gaps/update", handler.handleKnowledgeGapUpdate)
	handler.mux.HandleFunc("POST /admin/knowledge/repairs/retest", handler.handleKnowledgeRepairRetest)
	handler.mux.HandleFunc("POST /admin/knowledge/repairs/verify", handler.handleKnowledgeRepairVerify)
	handler.mux.HandleFunc("GET /admin/knowledge/repairs/verifications", handler.handleKnowledgeRepairVerifications)
	handler.mux.HandleFunc("GET /admin/knowledge/repairs/recurrences", handler.handleKnowledgeRecurrenceList)
	handler.mux.HandleFunc("POST /admin/knowledge/repairs/recurrences/confirm", handler.handleKnowledgeRecurrenceConfirm)
	handler.mux.HandleFunc("POST /admin/knowledge/repairs/recurrences/dismiss", handler.handleKnowledgeRecurrenceDismiss)
	handler.mux.HandleFunc("POST /admin/knowledge/repairs/promotions", handler.handleKnowledgeRepairPromotion)
	handler.mux.HandleFunc("GET /admin/knowledge/repairs/promotions", handler.handleKnowledgeRepairPromotionList)
	handler.mux.HandleFunc("GET /admin/knowledge/quality-trends", handler.handleKnowledgeQualityTrends)
	handler.mux.HandleFunc("POST /admin/knowledge/quality-review-checkpoints", handler.handleQualityReviewCheckpointCreate)
	handler.mux.HandleFunc("GET /admin/knowledge/quality-review-checkpoints", handler.handleQualityReviewCheckpointList)
	handler.mux.HandleFunc("POST /admin/knowledge/reindex", handler.handleKnowledgeReindex)
	handler.mux.HandleFunc("POST /admin/knowledge/citation-test", handler.handleKnowledgeCitationTest)
	handler.mux.HandleFunc("POST /admin/knowledge/retrieval-diagnostics", handler.handleKnowledgeRetrievalDiagnostics)
	handler.mux.HandleFunc("POST /admin/tools/policy", handler.handleToolPolicySave)
	handler.mux.HandleFunc("POST /admin/tools/authorize", handler.handleToolAuthorize)
	handler.mux.HandleFunc("GET /admin/audit", handler.handleAuditRecent)
	handler.mux.HandleFunc("GET /admin/audit/timeline", handler.handleAuditTimeline)
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := requestIDFrom(r)
	w.Header().Set("X-Request-ID", requestID)
	if adminRoute(r.URL.Path) {
		key, ok := h.authorizedKey(h.adminAPIKeys, h.allowAnonymousAdmin, r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		if !h.allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "rate_limited"})
			return
		}
	} else if runtimeProtectedRoute(r.URL.Path) {
		key, ok := h.authorizedKey(h.apiKeys, true, r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		if !h.allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "rate_limited"})
			return
		}
	}
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) handleAdminAccess(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"auth_required": !h.allowAnonymousAdmin})
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	h.metrics.IncCounter("requests_total", map[string]string{"route": "/health"})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) handleReady(w http.ResponseWriter, _ *http.Request) {
	h.metrics.IncCounter("requests_total", map[string]string{"route": "/ready"})
	checks := map[string]string{
		"config":       "ok",
		"data_dir":     "ok",
		"release_gate": h.readiness.releaseGateStatus(),
	}
	details := map[string]string{}
	ready := true

	if h.readiness.ConfigError != nil {
		ready = false
		checks["config"] = "failed"
		details["config"] = h.readiness.redact(h.readiness.ConfigError.Error())
	}
	if err := h.readiness.checkDataDir(); err != nil {
		ready = false
		checks["data_dir"] = "failed"
		details["data_dir"] = err.Error()
	}
	if checks["release_gate"] == "failed" {
		ready = false
	}
	if ready {
		h.metrics.SetGauge("readiness_status", 1, nil)
	} else {
		h.metrics.SetGauge("readiness_status", 0, nil)
	}

	status := http.StatusOK
	overall := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		overall = "failed"
	}
	writeJSON(w, status, map[string]any{
		"status":         overall,
		"checks":         checks,
		"details":        details,
		"config_summary": h.readiness.ConfigSummary,
	})
}

func (h *Handler) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	body, contentType, err := (observability.PrometheusExporter{}).Export(h.metrics.Snapshot())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "metrics_export_failed"})
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) handleRuntimeStatus(w http.ResponseWriter, _ *http.Request) {
	h.metrics.IncCounter("requests_total", map[string]string{"route": "/runtime/status"})
	writeJSON(w, http.StatusOK, h.runtimeStatus)
}

func (r ReadinessConfig) releaseGateStatus() string {
	if strings.TrimSpace(r.ReleaseGateStatus) == "" {
		return "skipped"
	}
	return r.ReleaseGateStatus
}

func (r ReadinessConfig) redact(text string) string {
	if r.Redact == nil {
		return text
	}
	return r.Redact(text)
}

func (r ReadinessConfig) checkDataDir() error {
	if strings.TrimSpace(r.DataDir) == "" {
		return nil
	}
	info, err := os.Stat(r.DataDir)
	if err != nil {
		return fmt.Errorf("data dir unavailable")
	}
	if !info.IsDir() {
		return fmt.Errorf("data dir is not a directory")
	}
	path := filepath.Join(r.DataDir, ".readiness")
	if err := os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0o600); err != nil {
		return fmt.Errorf("data dir is not writable")
	}
	_ = os.Remove(path)
	return nil
}

func (h *Handler) handleFavicon(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleStaticHTML(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if h.staticDir == "" {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "static_dir_unavailable"})
			return
		}
		body, err := os.ReadFile(filepath.Join(h.staticDir, name))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "static_asset_missing", "cause": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

func (h *Handler) handleStaticAsset(w http.ResponseWriter, r *http.Request) {
	asset := strings.TrimPrefix(r.URL.Path, "/web/")
	contentTypes := map[string]string{
		"app.css":  "text/css; charset=utf-8",
		"app.js":   "application/javascript; charset=utf-8",
		"admin.js": "application/javascript; charset=utf-8",
	}
	contentType, ok := contentTypes[asset]
	if h.staticDir == "" || !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "static_asset_missing"})
		return
	}
	body, err := os.ReadFile(filepath.Join(h.staticDir, asset))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "static_asset_missing", "cause": err.Error()})
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	if h.orchestrator == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "orchestrator_unavailable"})
		return
	}
	var conversation types.Conversation
	if err := json.NewDecoder(r.Body).Decode(&conversation); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	h.applyAuthoritativeConversationIdentity(&conversation)
	result, err := h.orchestrator.Handle(r.Context(), conversation)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "orchestrator_error", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handlePersonaDraft(w http.ResponseWriter, r *http.Request) {
	if h.personaAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "persona_admin_unavailable"})
		return
	}
	var draft persona.Persona
	if err := json.NewDecoder(r.Body).Decode(&draft); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	version, err := h.personaAdmin.SaveDraft(h.adminTenantID(), draft)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "persona_draft_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, version)
}

type personaVersionRequest struct {
	VersionID string `json:"version_id"`
}

func (h *Handler) handlePersonaPublish(w http.ResponseWriter, r *http.Request) {
	h.handlePersonaVersionAction(w, r, "publish", func(tenantID, versionID string) (admin.PersonaVersion, error) {
		return h.personaAdmin.Publish(tenantID, versionID)
	})
}

func (h *Handler) handlePersonaRollback(w http.ResponseWriter, r *http.Request) {
	h.handlePersonaVersionAction(w, r, "rollback", func(tenantID, versionID string) (admin.PersonaVersion, error) {
		return h.personaAdmin.Rollback(tenantID, versionID)
	})
}

func (h *Handler) handlePersonaVersionAction(w http.ResponseWriter, r *http.Request, action string, apply func(string, string) (admin.PersonaVersion, error)) {
	if h.personaAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "persona_admin_unavailable"})
		return
	}
	var request personaVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	version, err := apply(h.adminTenantID(), request.VersionID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "persona_" + action + "_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, version)
}

func (h *Handler) handlePersonaActive(w http.ResponseWriter, _ *http.Request) {
	if h.personaAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "persona_admin_unavailable"})
		return
	}
	version, err := h.personaAdmin.Active(h.adminTenantID())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "none"})
		return
	}
	writeJSON(w, http.StatusOK, version)
}

func (h *Handler) handleMemoryList(w http.ResponseWriter, _ *http.Request) {
	if h.memoryAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "memory_admin_unavailable"})
		return
	}
	records, err := h.memoryAdmin.List(h.adminTenantID())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "memory_list_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

type memoryDisableRequest struct {
	MemoryID string `json:"memory_id"`
}

func (h *Handler) handleMemoryDisable(w http.ResponseWriter, r *http.Request) {
	if h.memoryAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "memory_admin_unavailable"})
		return
	}
	var request memoryDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	record, err := h.memoryAdmin.Disable(h.adminTenantID(), request.MemoryID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "memory_disable_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) handleKnowledgeUpload(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	var upload admin.KnowledgeUpload
	if err := json.NewDecoder(r.Body).Decode(&upload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	document, err := h.knowledgeAdmin.Upload(h.adminTenantID(), upload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_upload_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, document)
}

type knowledgeNoteCreateRequest struct {
	SpaceID     string `json:"space_id,omitempty"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	SourceGapID string `json:"source_gap_id,omitempty"`
}

func (h *Handler) handleKnowledgeNoteCreate(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	var request knowledgeNoteCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	title := strings.TrimSpace(request.Title)
	body := strings.TrimSpace(request.Body)
	if title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_note_create_failed", "cause": "knowledge note title is required"})
		return
	}
	if body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_note_create_failed", "cause": "knowledge note body is required"})
		return
	}
	sourceGapID := strings.TrimSpace(request.SourceGapID)
	if sourceGapID != "" && !isSafeKnowledgeToken(sourceGapID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_note_create_failed", "cause": "invalid source_gap_id"})
		return
	}
	spaceID := strings.TrimSpace(request.SpaceID)
	if spaceID == "" {
		spaceID = admin.DefaultKnowledgeSpaceID
	}
	if sourceGapID != "" {
		if h.knowledgeGapAdmin == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_gap_admin_unavailable"})
			return
		}
		gap, err := h.knowledgeGapAdmin.Get(h.adminTenantID(), sourceGapID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_note_create_failed", "cause": err.Error()})
			return
		}
		if gap.SpaceID != spaceID {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_note_create_failed", "cause": "source_gap_id must belong to the selected knowledge space"})
			return
		}
	}
	metadata := map[string]string{
		"source_type":  "workbench_note",
		"created_from": "knowledge_workbench",
	}
	if sourceGapID != "" {
		metadata["source_gap_id"] = sourceGapID
	}
	document, err := h.knowledgeAdmin.Upload(h.adminTenantID(), admin.KnowledgeUpload{
		ID:       noteIDFromTitle(title),
		Name:     title + ".md",
		Content:  body,
		SpaceID:  spaceID,
		Metadata: metadata,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_note_create_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, document)
}

type knowledgeCitationRequest struct {
	Query string `json:"query"`
}

type knowledgeDiagnosticsRequest struct {
	Query    string                  `json:"query"`
	Mode     knowledge.RetrievalMode `json:"mode"`
	SpaceID  string                  `json:"space_id,omitempty"`
	Limit    int                     `json:"limit"`
	MinScore float64                 `json:"min_score"`
}

type knowledgeImportRequest struct {
	SpaceID     string                          `json:"space_id,omitempty"`
	SourceType  admin.KnowledgeImportSourceType `json:"source_type"`
	SourceLabel string                          `json:"source_label,omitempty"`
	Sources     []admin.KnowledgeImportSource   `json:"sources"`
}

type knowledgeDocumentRequest struct {
	DocumentID string `json:"document_id"`
	Content    string `json:"content,omitempty"`
	SpaceID    string `json:"space_id,omitempty"`
}

type knowledgeRepairRetestRequest struct {
	GapID string `json:"gap_id"`
}

type knowledgeReviewRequest struct {
	DocumentID   string                      `json:"document_id"`
	ReviewStatus admin.KnowledgeReviewStatus `json:"review_status"`
	Reason       string                      `json:"reason,omitempty"`
	ReviewedBy   string                      `json:"reviewed_by,omitempty"`
}

type knowledgeUpdateRequest struct {
	DocumentID  string `json:"document_id"`
	Name        string `json:"name"`
	Content     string `json:"content"`
	SourceLabel string `json:"source_label,omitempty"`
}

type knowledgeSpaceRequest struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name,omitempty"`
	Description          string   `json:"description,omitempty"`
	DefaultRetrievalMode string   `json:"default_retrieval_mode,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
}

func (h *Handler) handleKnowledgeList(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	spaceID := strings.TrimSpace(r.URL.Query().Get("space_id"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	status := admin.KnowledgeStatus(strings.TrimSpace(r.URL.Query().Get("status")))
	reviewStatus := admin.KnowledgeReviewStatus(strings.TrimSpace(r.URL.Query().Get("review_status")))
	sourceType := strings.TrimSpace(r.URL.Query().Get("source_type"))
	gapLinked := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("gap_linked")), "true")
	var (
		documents []admin.KnowledgeDocument
		err       error
	)
	if query != "" || status != "" || reviewStatus != "" || sourceType != "" || gapLinked {
		documents, err = h.knowledgeAdmin.ListFiltered(h.adminTenantID(), admin.KnowledgeDocumentFilter{
			SpaceID:       spaceID,
			Query:         query,
			Status:        status,
			ReviewStatus:  reviewStatus,
			SourceType:    sourceType,
			GapLinkedOnly: gapLinked,
		})
	} else if spaceID != "" {
		documents, err = h.knowledgeAdmin.ListBySpace(h.adminTenantID(), spaceID)
	} else {
		documents, err = h.knowledgeAdmin.List(h.adminTenantID())
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "knowledge_list_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, documents)
}

func (h *Handler) handleKnowledgeImportList(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeImportAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_import_admin_unavailable"})
		return
	}
	jobs, err := h.knowledgeImportAdmin.List(h.adminTenantID(), strings.TrimSpace(r.URL.Query().Get("space_id")))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_import_list_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (h *Handler) handleKnowledgeImport(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeImportAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_import_admin_unavailable"})
		return
	}
	var request knowledgeImportRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	job, err := h.knowledgeImportAdmin.Import(h.adminTenantID(), admin.KnowledgeImportRequest{
		SpaceID:     request.SpaceID,
		SourceType:  request.SourceType,
		SourceLabel: request.SourceLabel,
		Sources:     request.Sources,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_import_failed", "cause": err.Error(), "job": job})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) handleKnowledgeReview(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	var request knowledgeReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	document, err := h.knowledgeAdmin.Review(h.adminTenantID(), admin.KnowledgeReviewUpdate{
		DocumentID:   request.DocumentID,
		ReviewStatus: request.ReviewStatus,
		Reason:       request.Reason,
		ReviewedBy:   request.ReviewedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_review_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func (h *Handler) handleKnowledgeGet(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	document, err := h.knowledgeAdmin.Get(h.adminTenantID(), r.PathValue("documentID"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "knowledge_document_missing", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func (h *Handler) handleKnowledgeDetail(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	detail, err := h.knowledgeAdmin.DocumentDetailWithRelations(h.adminTenantID(), r.PathValue("documentID"), h.detailGapService())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_detail_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) handleKnowledgeCitationTest(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	var request knowledgeCitationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	citation, err := h.knowledgeAdmin.CitationTest(h.adminTenantID(), request.Query)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "knowledge_citation_missing", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, citation)
}

func (h *Handler) handleKnowledgeSpaceList(w http.ResponseWriter, _ *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	spaces, err := h.knowledgeAdmin.ListSpaces(h.adminTenantID())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "knowledge_space_list_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, spaces)
}

func (h *Handler) handleKnowledgeSpaceCreate(w http.ResponseWriter, r *http.Request) {
	h.handleKnowledgeSpaceMutation(w, r, "knowledge_space_create_failed", func(request knowledgeSpaceRequest) (any, error) {
		return h.knowledgeAdmin.CreateSpace(h.adminTenantID(), admin.KnowledgeSpaceInput{
			ID:                   request.ID,
			Name:                 request.Name,
			Description:          request.Description,
			DefaultRetrievalMode: request.DefaultRetrievalMode,
			Tags:                 request.Tags,
		})
	})
}

func (h *Handler) handleKnowledgeSpaceUpdate(w http.ResponseWriter, r *http.Request) {
	h.handleKnowledgeSpaceMutation(w, r, "knowledge_space_update_failed", func(request knowledgeSpaceRequest) (any, error) {
		return h.knowledgeAdmin.UpdateSpace(h.adminTenantID(), admin.KnowledgeSpaceInput{
			ID:                   request.ID,
			Name:                 request.Name,
			Description:          request.Description,
			DefaultRetrievalMode: request.DefaultRetrievalMode,
			Tags:                 request.Tags,
		})
	})
}

func (h *Handler) handleKnowledgeSpaceDisable(w http.ResponseWriter, r *http.Request) {
	h.handleKnowledgeSpaceMutation(w, r, "knowledge_space_disable_failed", func(request knowledgeSpaceRequest) (any, error) {
		return h.knowledgeAdmin.DisableSpace(h.adminTenantID(), request.ID)
	})
}

func (h *Handler) handleKnowledgeSpaceEnable(w http.ResponseWriter, r *http.Request) {
	h.handleKnowledgeSpaceMutation(w, r, "knowledge_space_enable_failed", func(request knowledgeSpaceRequest) (any, error) {
		return h.knowledgeAdmin.EnableSpace(h.adminTenantID(), request.ID)
	})
}

func (h *Handler) handleKnowledgeSpaceArchive(w http.ResponseWriter, r *http.Request) {
	h.handleKnowledgeSpaceMutation(w, r, "knowledge_space_archive_failed", func(request knowledgeSpaceRequest) (any, error) {
		return h.knowledgeAdmin.ArchiveSpace(h.adminTenantID(), request.ID)
	})
}

func (h *Handler) handleKnowledgeHealth(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	summary, err := h.knowledgeAdmin.HealthSummary(h.adminTenantID(), r.URL.Query().Get("space_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_health_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) handleKnowledgeRetrievalDiagnostics(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeRetriever == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_retriever_unavailable"})
		return
	}
	var request knowledgeDiagnosticsRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	response, err := h.knowledgeRetriever.Diagnostics(r.Context(), h.adminTenantID(), knowledge.SearchRequest{
		Query:    request.Query,
		Limit:    request.Limit,
		Mode:     request.Mode,
		SpaceID:  request.SpaceID,
		MinScore: request.MinScore,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "knowledge_diagnostics_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

type knowledgeGapRequest struct {
	GapID                string                   `json:"gap_id"`
	Status               admin.KnowledgeGapStatus `json:"status"`
	ResolvedByDocumentID string                   `json:"resolved_by_document_id,omitempty"`
	ResolutionNote       string                   `json:"resolution_note,omitempty"`
}

func noteIDFromTitle(title string) string {
	slug := strings.ToLower(strings.TrimSpace(title))
	slug = strings.ReplaceAll(slug, " ", "-")
	var builder strings.Builder
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		}
	}
	normalized := strings.Trim(builder.String(), "-.")
	if normalized == "" {
		normalized = "note"
	}
	return fmt.Sprintf("%s-%d", normalized, time.Now().UnixNano())
}

func isSafeKnowledgeToken(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

func (h *Handler) handleKnowledgeGapList(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeGapAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_gap_admin_unavailable"})
		return
	}
	records, err := h.knowledgeGapAdmin.List(h.adminTenantID(), r.URL.Query().Get("space_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_gap_list_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *Handler) handleKnowledgeRepairList(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil || h.knowledgeGapAdmin == nil || h.auditAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_repair_admin_unavailable"})
		return
	}
	filter, err := parseKnowledgeRepairFilter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_limit", "cause": err.Error()})
		return
	}
	service := admin.NewKnowledgeRepairServiceWithRecurrenceAndPromotion(*h.knowledgeGapAdmin, *h.auditAdmin, *h.knowledgeAdmin, h.verificationAdmin, h.recurrenceAdmin, h.promotionStore)
	items, err := service.List(h.adminTenantID(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "knowledge_repair_list_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type knowledgeRepairPromotionRequest struct {
	GapID                 string                       `json:"gap_id"`
	VerificationAttemptID string                       `json:"verification_attempt_id"`
	MinimumSupportState   admin.RepairEvalSupportState `json:"minimum_support_state"`
	RequiredDocumentIDs   []string                     `json:"required_document_ids,omitempty"`
	PromotedBy            string                       `json:"promoted_by"`
}

func (h *Handler) handleKnowledgeRepairPromotion(w http.ResponseWriter, r *http.Request) {
	if h.promotionAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_repair_promotion_unavailable"})
		return
	}
	var request knowledgeRepairPromotionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	if !isSafeKnowledgeToken(strings.TrimSpace(request.GapID)) || !isSafeKnowledgeToken(strings.TrimSpace(request.VerificationAttemptID)) || strings.TrimSpace(request.PromotedBy) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_knowledge_repair_promotion"})
		return
	}
	record, applied, err := h.promotionAdmin.Promote(r.Context(), h.adminTenantID(), admin.RepairEvalPromotionRequest{
		GapID: request.GapID, VerificationAttemptID: request.VerificationAttemptID, MinimumSupportState: request.MinimumSupportState,
		RequiredDocumentIDs: request.RequiredDocumentIDs, PromotedBy: request.PromotedBy,
	})
	if err != nil {
		statusCode, errorCode := promotionErrorResponse(err)
		writeJSON(w, statusCode, map[string]any{"error": errorCode})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promotion": record, "applied": applied})
}

func (h *Handler) handleKnowledgeRepairPromotionList(w http.ResponseWriter, r *http.Request) {
	if h.promotionStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_repair_promotion_unavailable"})
		return
	}
	gapID := strings.TrimSpace(r.URL.Query().Get("gap_id"))
	if gapID != "" && !isSafeKnowledgeToken(gapID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_gap_id"})
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_limit"})
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}
	activeOnly := r.URL.Query().Get("active_only") == "true" || r.URL.Query().Get("active") == "true"
	items, err := h.promotionStore.ListRepairEvalPromotions(h.adminTenantID(), gapID, activeOnly, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "knowledge_repair_promotion_list_failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleKnowledgeQualityTrends(w http.ResponseWriter, r *http.Request) {
	if h.qualityTrendsAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_quality_trends_unavailable"})
		return
	}
	projection, err := h.qualityTrendsAdmin.Project(h.adminTenantID(), admin.QualityTrendRequest{
		From: r.URL.Query().Get("from"), To: r.URL.Query().Get("to"), SpaceID: r.URL.Query().Get("space_id"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "date") || strings.Contains(err.Error(), "from and to") {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_quality_trend_filter"})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_quality_trends_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": h.adminTenantID(), "projection": projection})
}

type qualityReviewCheckpointResponse struct {
	ID        string                        `json:"id"`
	CreatedAt time.Time                     `json:"created_at"`
	Filter    admin.QualityReviewFilter     `json:"filter"`
	Outcome   admin.QualityReviewOutcome    `json:"outcome"`
	Rationale string                        `json:"rationale,omitempty"`
	GapIDs    []string                      `json:"gap_ids,omitempty"`
	Snapshot  admin.QualityReviewSnapshotV1 `json:"snapshot"`
}

func (h *Handler) handleQualityReviewCheckpointCreate(w http.ResponseWriter, r *http.Request) {
	if h.qualityReviewsAdmin == nil {
		writeQualityReviewError(w, http.StatusServiceUnavailable, "quality_review_service_unavailable", "Quality review checkpoints are unavailable.", "Retry after the service is configured.")
		return
	}
	var request admin.QualityReviewCheckpointRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeQualityReviewError(w, http.StatusRequestEntityTooLarge, "quality_review_request_too_large", "The review request is too large.", "Shorten the rationale or selected IDs and retry.")
			return
		}
		writeQualityReviewError(w, http.StatusBadRequest, "invalid_quality_review_request", "The review request is malformed.", "Send one JSON object with only documented fields.")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeQualityReviewError(w, http.StatusBadRequest, "invalid_quality_review_request", "The review request must contain one JSON object.", "Remove trailing JSON and retry.")
		return
	}
	result, err := h.qualityReviewsAdmin.Create(h.adminTenantID(), request)
	if err != nil {
		status, code, message, hint := qualityReviewErrorResponse(err)
		writeQualityReviewError(w, status, code, message, hint)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"checkpoint": qualityReviewCheckpointResponseFromRecord(result.Checkpoint), "created": result.Created})
}

func (h *Handler) handleQualityReviewCheckpointList(w http.ResponseWriter, r *http.Request) {
	if h.qualityReviewsAdmin == nil {
		writeQualityReviewError(w, http.StatusServiceUnavailable, "quality_review_service_unavailable", "Quality review checkpoints are unavailable.", "Retry after the service is configured.")
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeQualityReviewError(w, http.StatusBadRequest, "invalid_quality_review_request", "The history limit is invalid.", "Use a whole number from 1 through 20.")
			return
		}
		limit = parsed
	}
	history, err := h.qualityReviewsAdmin.List(h.adminTenantID(), r.URL.Query().Get("space_id"), limit)
	if err != nil {
		status, code, message, hint := qualityReviewErrorResponse(err)
		writeQualityReviewError(w, status, code, message, hint)
		return
	}
	items := make([]qualityReviewCheckpointResponse, 0, len(history.Checkpoints))
	for _, record := range history.Checkpoints {
		items = append(items, qualityReviewCheckpointResponseFromRecord(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"checkpoints": items, "excluded_records": history.ExcludedRecords, "comparison": history.Comparison})
}

func qualityReviewCheckpointResponseFromRecord(record admin.QualityReviewCheckpoint) qualityReviewCheckpointResponse {
	return qualityReviewCheckpointResponse{
		ID: record.ID, CreatedAt: record.CreatedAt, Filter: record.Filter, Outcome: record.Outcome,
		Rationale: record.Rationale, GapIDs: append([]string(nil), record.GapIDs...), Snapshot: record.Snapshot,
	}
}

func writeQualityReviewError(w http.ResponseWriter, status int, code, message, hint string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message, "hint": hint})
}

func qualityReviewErrorResponse(err error) (int, string, string, string) {
	switch {
	case errors.Is(err, admin.ErrQualityReviewIdempotencyConflict):
		return http.StatusConflict, "quality_review_idempotency_conflict", "The idempotency key was already used for a different review decision.", "Reuse a key only for the identical request, or generate a new key after changing the draft."
	case errors.Is(err, admin.ErrQualityReviewGapNotEligible):
		return http.StatusConflict, "quality_review_gap_not_eligible", "One or more selected gaps are no longer eligible for this evidence window.", "Refresh Quality Trends and select gaps from the refreshed candidate list."
	case errors.Is(err, admin.ErrQualityReviewInvalid):
		return http.StatusBadRequest, "invalid_quality_review_request", "The review request is invalid.", "Correct the date range, outcome, rationale, idempotency key, or selected IDs and retry."
	case errors.Is(err, admin.ErrQualityReviewCapacity):
		return http.StatusServiceUnavailable, "quality_review_storage_unavailable", "Checkpoint storage cannot accept another record safely.", "Preserve the draft and contact the local service operator with the request ID."
	default:
		return http.StatusServiceUnavailable, "quality_review_storage_unavailable", "The checkpoint could not be saved safely.", "Preserve the draft, retry, and use the request ID to inspect service health."
	}
}

func promotionErrorResponse(err error) (int, string) {
	switch {
	case errors.Is(err, admin.ErrRepairEvalPromotionUnavailable):
		return http.StatusServiceUnavailable, "knowledge_repair_promotion_unavailable"
	case errors.Is(err, admin.ErrRepairEvalPromotionInvalidRequest), errors.Is(err, admin.ErrRepairEvalPromotionDocumentNotEligible):
		return http.StatusBadRequest, "invalid_knowledge_repair_promotion"
	case errors.Is(err, admin.ErrRepairEvalPromotionGapNotEligible), errors.Is(err, admin.ErrRepairEvalPromotionVerificationStale), errors.Is(err, admin.ErrRepairEvalPromotionRecurrencePending):
		return http.StatusConflict, "knowledge_repair_promotion_not_eligible"
	case errors.Is(err, admin.ErrKnowledgeDocumentNotFound), errors.Is(err, admin.ErrKnowledgeSpaceNotFound):
		return http.StatusNotFound, "knowledge_repair_promotion_target_not_found"
	default:
		return http.StatusInternalServerError, "knowledge_repair_promotion_failed"
	}
}

type knowledgeRepairVerifyRequest struct {
	GapID string `json:"gap_id"`
}

func (h *Handler) handleKnowledgeRepairVerify(w http.ResponseWriter, r *http.Request) {
	if h.verificationAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_repair_verification_unavailable"})
		return
	}
	var request knowledgeRepairVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	gapID := strings.TrimSpace(request.GapID)
	if !isSafeKnowledgeToken(gapID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_repair_verify_failed", "cause": "invalid gap_id"})
		return
	}
	attempt, err := h.verificationAdmin.Verify(r.Context(), h.adminTenantID(), gapID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_repair_verify_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, attempt)
}

func (h *Handler) handleKnowledgeRepairVerifications(w http.ResponseWriter, r *http.Request) {
	if h.verificationAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_repair_verification_unavailable"})
		return
	}
	gapID := strings.TrimSpace(r.URL.Query().Get("gap_id"))
	if gapID != "" && !isSafeKnowledgeToken(gapID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_gap_id"})
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_limit"})
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}
	items, err := h.verificationAdmin.List(h.adminTenantID(), gapID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "knowledge_repair_verification_list_failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleKnowledgeRecurrenceList(w http.ResponseWriter, r *http.Request) {
	if h.recurrenceAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_recurrence_unavailable"})
		return
	}
	gapID := strings.TrimSpace(r.URL.Query().Get("gap_id"))
	if gapID != "" && !isSafeKnowledgeToken(gapID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_gap_id"})
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_limit"})
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}
	items, err := h.recurrenceAdmin.List(h.adminTenantID(), gapID, strings.TrimSpace(r.URL.Query().Get("status")), limit)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "knowledge_recurrence_list_failed"
		if errors.Is(err, admin.ErrRepairRecurrenceInvalidStatus) {
			statusCode = http.StatusBadRequest
			errorCode = "invalid_recurrence_status"
		}
		writeJSON(w, statusCode, map[string]any{"error": errorCode})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type knowledgeRecurrenceActionRequest struct {
	RecurrenceID string `json:"recurrence_id"`
	ConfirmedBy  string `json:"confirmed_by,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

func (h *Handler) handleKnowledgeRecurrenceConfirm(w http.ResponseWriter, r *http.Request) {
	if h.recurrenceAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_recurrence_unavailable"})
		return
	}
	var request knowledgeRecurrenceActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	if !isSafeKnowledgeToken(strings.TrimSpace(request.RecurrenceID)) || strings.TrimSpace(request.ConfirmedBy) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_recurrence_confirmation"})
		return
	}
	record, err := h.recurrenceAdmin.Confirm(h.adminTenantID(), strings.TrimSpace(request.RecurrenceID), request.ConfirmedBy)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_recurrence_confirm_failed"})
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) handleKnowledgeRecurrenceDismiss(w http.ResponseWriter, r *http.Request) {
	if h.recurrenceAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_recurrence_unavailable"})
		return
	}
	var request knowledgeRecurrenceActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	if !isSafeKnowledgeToken(strings.TrimSpace(request.RecurrenceID)) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_recurrence_dismissal"})
		return
	}
	record, err := h.recurrenceAdmin.Dismiss(h.adminTenantID(), strings.TrimSpace(request.RecurrenceID), request.Reason)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_recurrence_dismiss_failed"})
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) handleKnowledgeGapUpdate(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeGapAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_gap_admin_unavailable"})
		return
	}
	var request knowledgeGapRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	record, err := h.knowledgeGapAdmin.UpdateStatus(
		h.adminTenantID(),
		request.GapID,
		request.Status,
		request.ResolvedByDocumentID,
		request.ResolutionNote,
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_gap_update_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) handleKnowledgeRepairRetest(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeGapAdmin == nil || h.knowledgeRetriever == nil || h.auditAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_repair_retest_unavailable"})
		return
	}
	var request knowledgeRepairRetestRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	gapID := strings.TrimSpace(request.GapID)
	if !isSafeKnowledgeToken(gapID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_repair_retest_failed", "cause": "invalid gap_id"})
		return
	}
	gap, err := h.knowledgeGapAdmin.Get(h.adminTenantID(), gapID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_repair_retest_failed", "cause": err.Error()})
		return
	}
	beforeState := "unknown"
	if h.knowledgeAdmin != nil {
		service := admin.NewKnowledgeRepairService(*h.knowledgeGapAdmin, *h.auditAdmin, *h.knowledgeAdmin)
		items, err := service.List(h.adminTenantID(), admin.KnowledgeRepairFilter{SpaceID: gap.SpaceID})
		if err == nil {
			for _, item := range items {
				if item.GapID == gap.ID {
					beforeState = item.AnswerState
					break
				}
			}
		}
	}
	diagnostics, err := h.knowledgeRetriever.Diagnostics(r.Context(), h.adminTenantID(), knowledge.SearchRequest{
		Query:   gap.Question,
		Mode:    knowledge.RetrievalModeLexical,
		SpaceID: gap.SpaceID,
		Limit:   3,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "knowledge_repair_retest_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"gap_id":           gap.ID,
		"question_summary": gap.Question,
		"before_state":     beforeState,
		"after_state":      repairRetestState(diagnostics),
		"source_count":     len(diagnostics.Results),
		"top_sources":      repairRetestSources(diagnostics.Results),
		"next_action":      repairRetestNextAction(repairRetestState(diagnostics)),
	})
}

func (h *Handler) handleKnowledgeDisable(w http.ResponseWriter, r *http.Request) {
	h.handleKnowledgeMutation(w, r, "knowledge_disable_failed", func(request knowledgeDocumentRequest) (any, error) {
		return h.knowledgeAdmin.Disable(h.adminTenantID(), request.DocumentID)
	})
}

func (h *Handler) handleKnowledgeEnable(w http.ResponseWriter, r *http.Request) {
	h.handleKnowledgeMutation(w, r, "knowledge_enable_failed", func(request knowledgeDocumentRequest) (any, error) {
		return h.knowledgeAdmin.Enable(h.adminTenantID(), request.DocumentID)
	})
}

func (h *Handler) handleKnowledgeDelete(w http.ResponseWriter, r *http.Request) {
	h.handleKnowledgeMutation(w, r, "knowledge_delete_failed", func(request knowledgeDocumentRequest) (any, error) {
		if err := h.knowledgeAdmin.Delete(h.adminTenantID(), request.DocumentID); err != nil {
			return nil, err
		}
		return map[string]any{"status": "deleted", "document_id": request.DocumentID}, nil
	})
}

func (h *Handler) handleKnowledgeUpdate(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	var request knowledgeUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	document, err := h.knowledgeAdmin.Update(h.adminTenantID(), admin.KnowledgeUpdate{
		DocumentID:  request.DocumentID,
		Name:        request.Name,
		Content:     request.Content,
		SourceLabel: request.SourceLabel,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_update_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func (h *Handler) handleKnowledgeReindex(w http.ResponseWriter, r *http.Request) {
	h.handleKnowledgeMutation(w, r, "knowledge_reindex_failed", func(request knowledgeDocumentRequest) (any, error) {
		return h.knowledgeAdmin.Reindex(h.adminTenantID(), request.DocumentID, request.Content)
	})
}

func (h *Handler) handleKnowledgeMutation(w http.ResponseWriter, r *http.Request, code string, apply func(knowledgeDocumentRequest) (any, error)) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	var request knowledgeDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	result, err := apply(request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": code, "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) detailGapService() admin.KnowledgeGapService {
	if h.knowledgeGapAdmin == nil {
		return admin.KnowledgeGapService{}
	}
	return *h.knowledgeGapAdmin
}

func (h *Handler) handleKnowledgeSpaceMutation(w http.ResponseWriter, r *http.Request, code string, apply func(knowledgeSpaceRequest) (any, error)) {
	if h.knowledgeAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "knowledge_admin_unavailable"})
		return
	}
	var request knowledgeSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	result, err := apply(request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": code, "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleToolPolicySave(w http.ResponseWriter, r *http.Request) {
	if h.toolPolicyAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "tool_policy_admin_unavailable"})
		return
	}
	var policy admin.ToolPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	saved, err := h.toolPolicyAdmin.Save(h.adminTenantID(), policy)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "tool_policy_save_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

type toolAuthorizeRequest struct {
	PersonaID string `json:"persona_id"`
	ToolName  string `json:"tool_name"`
}

func (h *Handler) handleToolAuthorize(w http.ResponseWriter, r *http.Request) {
	if h.toolPolicyAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "tool_policy_admin_unavailable"})
		return
	}
	var request toolAuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	if err := h.toolPolicyAdmin.Authorize(h.adminTenantID(), request.PersonaID, request.ToolName); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "tool_denied", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "allowed"})
}

func (h *Handler) handleAuditRecent(w http.ResponseWriter, _ *http.Request) {
	if h.auditAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "audit_admin_unavailable"})
		return
	}
	records, err := h.auditAdmin.Recent(h.adminTenantID())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "audit_recent_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *Handler) handleAuditTimeline(w http.ResponseWriter, r *http.Request) {
	if h.auditAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "audit_timeline_unavailable"})
		return
	}
	filter, err := parseAuditTimelineFilter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_limit", "cause": err.Error()})
		return
	}
	items, err := h.auditAdmin.Timeline(h.adminTenantID(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "audit_timeline_failed", "cause": err.Error()})
		return
	}
	h.attachTimelineGaps(items)
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if h.orchestrator == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "orchestrator_unavailable"})
		return
	}
	if streaming, ok := h.orchestrator.(core.StreamingOrchestrator); ok {
		var request types.TurnRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
			return
		}
		h.applyAuthoritativeTurnIdentity(&request)
		if err := request.Validate(); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_turn_request", "cause": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		result, err := streaming.Stream(r.Context(), request, httpStreamSink{writer: w})
		if err != nil {
			writeSSEJSON(w, string(types.StreamEventError), map[string]any{"error": "orchestrator_error", "cause": err.Error()})
			writeSSE(w, string(types.StreamEventDone), `{"status":"failed"}`)
			return
		}
		_ = result
		return
	}

	var conversation types.Conversation
	if err := json.NewDecoder(r.Body).Decode(&conversation); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	beforeEventCount := len(h.eventRecorder.Events())
	result, err := h.orchestrator.Handle(r.Context(), conversation)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "orchestrator_error", "cause": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	for _, event := range h.recordedEventsSince(beforeEventCount, conversation.ID) {
		writeSSEJSON(w, event.Topic, event)
	}
	writeSSE(w, "message_completed", result.Message.Content)
	writeSSE(w, "done", "ok")
}

func (h *Handler) handleExperienceStream(w http.ResponseWriter, r *http.Request) {
	if h.orchestrator == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "orchestrator_unavailable"})
		return
	}
	var conversation types.Conversation
	if err := json.NewDecoder(r.Body).Decode(&conversation); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	h.applyAuthoritativeConversationIdentity(&conversation)
	if streaming, ok := h.orchestrator.(core.StreamingOrchestrator); ok {
		request, err := turnRequestFromConversation(conversation)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_turn_request", "cause": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		presentationSink := &httpPresentationSink{writer: w}
		result, err := streaming.Stream(r.Context(), request, h.presentationAdapter.NewStreamSink(presentationSink))
		if err != nil {
			writeSSEJSON(w, string(presentation.EventError), map[string]any{"problem": "orchestrator_error", "cause": err.Error(), "fix": "retry"})
			writeSSEJSON(w, string(presentation.EventDone), map[string]any{"status": "failed"})
			return
		}
		auditRecord, _ := h.recordAudit(conversation, result, presentationSink.events, admin.AuditStatusCompleted, 0)
		h.captureKnowledgeGapAfterAudit(conversation, result, auditRecord)
		return
	}
	result, err := h.orchestrator.Handle(r.Context(), conversation)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "orchestrator_error", "cause": err.Error()})
		return
	}
	events, err := h.presentationAdapter.Adapt(presentation.AdaptRequest{
		Context: presentation.EventContext{
			TenantID:       conversation.TenantID,
			UserID:         conversation.UserID,
			ConversationID: conversation.ID,
			RequestID:      "req-1",
		},
		Result: result,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "presentation_error", "cause": err.Error()})
		return
	}
	auditRecord, _ := h.recordAudit(conversation, result, events, admin.AuditStatusCompleted, 0)
	h.captureKnowledgeGapAfterAudit(conversation, result, auditRecord)
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	for _, event := range events {
		writeSSEJSON(w, string(event.Name), event)
	}
}

type mockVoiceRequest struct {
	AudioText string `json:"audio_text"`
}

func (h *Handler) handleMockVoiceStream(w http.ResponseWriter, r *http.Request) {
	if h.orchestrator == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "orchestrator_unavailable"})
		return
	}
	asr := h.asr
	if asr == nil {
		asr = voice.MockASRClient{}
	}
	var request mockVoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	asrResult, err := asr.Transcribe(r.Context(), voice.ASRRequest{Audio: []byte(request.AudioText)})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "asr_error", "cause": err.Error()})
		return
	}
	now := timeNowUTC()
	conversation := types.Conversation{
		ID:       "mock-voice-session",
		TenantID: h.authoritativeTenantID("tenant-1"),
		UserID:   h.authoritativeUserID("user-1"),
		Messages: []types.Message{{
			ID:        "mock-voice-user",
			Role:      types.RoleUser,
			Content:   asrResult.Text,
			CreatedAt: now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	result, err := h.orchestrator.Handle(r.Context(), conversation)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "orchestrator_error", "cause": err.Error()})
		return
	}
	events, err := h.presentationAdapter.Adapt(presentation.AdaptRequest{
		Context: presentation.EventContext{
			TenantID:       conversation.TenantID,
			UserID:         conversation.UserID,
			ConversationID: conversation.ID,
			RequestID:      "req-1",
		},
		Result: result,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "presentation_error", "cause": err.Error()})
		return
	}
	auditRecord, _ := h.recordAudit(conversation, result, events, admin.AuditStatusCompleted, 0)
	h.captureKnowledgeGapAfterAudit(conversation, result, auditRecord)

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	writeSSEJSON(w, string(presentation.EventASRFinal), presentation.NewEvent(presentation.EventASRFinal, presentation.EventContext{
		TenantID:       conversation.TenantID,
		UserID:         conversation.UserID,
		ConversationID: conversation.ID,
		RequestID:      "req-1",
		Sequence:       1,
		OccurredAt:     now,
	}, map[string]any{
		"text":     asrResult.Text,
		"segments": asrResult.Segments,
	}, nil))
	for _, event := range events {
		event.Sequence++
		writeSSEJSON(w, string(event.Name), event)
	}
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

func (h *Handler) recordedEventsSince(start int, conversationID string) []runtime.RuntimeEvent {
	events := h.eventRecorder.Events()
	if start > len(events) {
		start = len(events)
	}
	filtered := make([]runtime.RuntimeEvent, 0, len(events)-start)
	for _, event := range events[start:] {
		if event.ConversationID == conversationID {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func (h *Handler) recordAudit(conversation types.Conversation, result types.AgentResult, events []presentation.Event, status admin.AuditStatus, latencyMS int64) (admin.AuditRecord, bool) {
	if h.auditAdmin == nil {
		return admin.AuditRecord{}, false
	}
	summary := make([]string, 0, len(events))
	for _, event := range events {
		summary = append(summary, string(event.Name))
	}
	record := admin.AuditRecord{
		ConversationID:  conversation.ID,
		UserID:          conversation.UserID,
		Status:          status,
		AgentName:       result.AgentName,
		LatencyMS:       latencyMS,
		EventSummary:    summary,
		QuestionSummary: summarizeAuditQuestion(conversation),
	}
	if result.Metadata != nil {
		record.KnowledgeSpaceID, _ = result.Metadata["knowledge_space_id"].(string)
		record.KnowledgeNoSourceReason, _ = result.Metadata["knowledge_no_source_reason"].(string)
		record.KnowledgeAnswerState, _ = result.Metadata["knowledge_answer_state"].(string)
		record.KnowledgeSourceCount, _ = result.Metadata["knowledge_result_count"].(int)
		record.KnowledgeEvidence = cloneEvidenceMetadata(result.Metadata["knowledge_evidence"])
	}
	if strings.TrimSpace(record.KnowledgeSpaceID) == "" && conversation.Metadata != nil {
		record.KnowledgeSpaceID, _ = conversation.Metadata["knowledge_space_id"].(string)
	}
	saved, err := h.auditAdmin.Record(conversation.TenantID, record)
	return saved, err == nil
}

func (h *Handler) captureKnowledgeGapAfterAudit(conversation types.Conversation, result types.AgentResult, audit admin.AuditRecord) {
	if h.recurrenceAdmin != nil && strings.TrimSpace(audit.ID) != "" {
		if _, handled, err := h.recurrenceAdmin.Detect(conversation.TenantID, audit); err == nil && handled {
			return
		}
	}
	h.captureKnowledgeGap(conversation, result)
}

func (h *Handler) captureKnowledgeGap(conversation types.Conversation, result types.AgentResult) {
	if h.knowledgeGapAdmin == nil {
		return
	}
	if result.Metadata == nil {
		return
	}
	answerState, _ := result.Metadata["knowledge_answer_state"].(string)
	switch strings.TrimSpace(answerState) {
	case "unsupported", "partially_supported", "review_gated":
	default:
		return
	}
	if result.Metadata["knowledge_used"] == true {
		return
	}
	reason, _ := result.Metadata["knowledge_no_source_reason"].(string)
	if strings.TrimSpace(reason) == "" {
		return
	}
	spaceID, _ := result.Metadata["knowledge_space_id"].(string)
	if strings.TrimSpace(spaceID) == "" && conversation.Metadata != nil {
		spaceID, _ = conversation.Metadata["knowledge_space_id"].(string)
	}
	question := lastUserQuestion(conversation)
	if strings.TrimSpace(question) == "" {
		return
	}
	_, _ = h.knowledgeGapAdmin.Create(conversation.TenantID, admin.KnowledgeGapInput{
		SpaceID:        spaceID,
		Question:       question,
		NoSourceReason: reason,
	})
}

func lastUserQuestion(conversation types.Conversation) string {
	for i := len(conversation.Messages) - 1; i >= 0; i-- {
		if conversation.Messages[i].Role == types.RoleUser {
			return strings.TrimSpace(conversation.Messages[i].Content)
		}
	}
	return ""
}

func summarizeAuditQuestion(conversation types.Conversation) string {
	question := lastUserQuestion(conversation)
	if len(question) <= 160 {
		return question
	}
	trimmed := strings.TrimSpace(question[:157])
	trimmed = strings.TrimRight(trimmed, " ,.;:")
	return trimmed + "..."
}

func (h *Handler) attachTimelineGaps(items []admin.AnswerAuditTimelineItem) {
	if h.knowledgeGapAdmin == nil {
		return
	}
	for i := range items {
		if items[i].Gap != nil {
			continue
		}
		if !isTimelineGapCandidate(items[i]) {
			continue
		}
		gap, ok := h.matchTimelineGap(items[i])
		if !ok {
			continue
		}
		items[i].Gap = &admin.AnswerAuditTimelineGap{
			GapID:  gap.ID,
			Status: string(gap.Status),
			Reason: gap.NoSourceReason,
		}
	}
}

func isTimelineGapCandidate(item admin.AnswerAuditTimelineItem) bool {
	switch strings.TrimSpace(item.AnswerState) {
	case "unsupported", "partially_supported", "review_gated":
		return strings.TrimSpace(item.QuestionSummary) != "" && strings.TrimSpace(item.Diagnostics.NoSourceReason) != ""
	default:
		return false
	}
}

func (h *Handler) matchTimelineGap(item admin.AnswerAuditTimelineItem) (admin.KnowledgeGap, bool) {
	records, err := h.knowledgeGapAdmin.List(h.adminTenantID(), item.KnowledgeSpaceID)
	if err != nil {
		return admin.KnowledgeGap{}, false
	}
	var matches []admin.KnowledgeGap
	for _, gap := range records {
		if strings.TrimSpace(gap.Question) != strings.TrimSpace(item.QuestionSummary) {
			continue
		}
		if strings.TrimSpace(gap.NoSourceReason) != strings.TrimSpace(item.Diagnostics.NoSourceReason) {
			continue
		}
		matches = append(matches, gap)
	}
	if len(matches) != 1 {
		return admin.KnowledgeGap{}, false
	}
	return matches[0], true
}

func parseAuditTimelineFilter(r *http.Request) (admin.AnswerAuditTimelineFilter, error) {
	query := r.URL.Query()
	filter := admin.AnswerAuditTimelineFilter{
		State:          strings.TrimSpace(query.Get("state")),
		WeakOnly:       strings.EqualFold(strings.TrimSpace(query.Get("weak_only")), "true"),
		DocumentID:     strings.TrimSpace(query.Get("document_id")),
		ConversationID: strings.TrimSpace(query.Get("conversation_id")),
		Limit:          50,
	}
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return admin.AnswerAuditTimelineFilter{}, fmt.Errorf("limit must be between 1 and 100")
		}
		if limit > 100 {
			limit = 100
		}
		filter.Limit = limit
	}
	return filter, nil
}

func parseKnowledgeRepairFilter(r *http.Request) (admin.KnowledgeRepairFilter, error) {
	query := r.URL.Query()
	filter := admin.KnowledgeRepairFilter{
		SpaceID:        strings.TrimSpace(query.Get("space_id")),
		Status:         strings.TrimSpace(query.Get("status")),
		Reason:         strings.TrimSpace(query.Get("reason")),
		WeakOnly:       strings.EqualFold(strings.TrimSpace(query.Get("weak_only")), "true"),
		UnresolvedOnly: strings.EqualFold(strings.TrimSpace(query.Get("unresolved_only")), "true"),
		LinkedEvidence: strings.EqualFold(strings.TrimSpace(query.Get("linked_evidence")), "true"),
		Limit:          50,
	}
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return admin.KnowledgeRepairFilter{}, fmt.Errorf("limit must be between 1 and 100")
		}
		if limit > 100 {
			limit = 100
		}
		filter.Limit = limit
	}
	if filter.SpaceID == "" {
		filter.SpaceID = admin.DefaultKnowledgeSpaceID
	}
	return filter, nil
}

func repairRetestState(response knowledge.SearchResponse) string {
	if len(response.Results) > 0 {
		return "grounded"
	}
	if strings.TrimSpace(response.NoSourceReason) == "no_review_active_documents" || response.ReviewGatedCount > 0 {
		return "review_gated"
	}
	return "unsupported"
}

func repairRetestSources(results []knowledge.Result) []admin.AnswerAuditTimelineSource {
	sources := make([]admin.AnswerAuditTimelineSource, 0, len(results))
	for _, result := range results {
		sources = append(sources, admin.AnswerAuditTimelineSource{
			DocumentID:   result.DocumentID,
			Title:        result.DocumentName,
			ReviewStatus: result.ReviewStatus,
			SourceType:   result.SourceType,
			Snippet:      result.Snippet,
		})
	}
	return sources
}

func repairRetestNextAction(afterState string) string {
	switch strings.TrimSpace(afterState) {
	case "grounded":
		return "Active reviewed source found. Resolve the gap if the source is sufficient."
	case "review_gated":
		return "Support improved only after review. Activate or review a source before resolving the gap."
	default:
		return "No supporting source found yet. Create or review evidence, then retest again."
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func cloneEvidenceMetadata(value any) map[string]any {
	source, ok := value.(map[string]any)
	if !ok || len(source) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(source))
	for key, entry := range source {
		cloned[key] = cloneEvidenceValue(entry)
	}
	return cloned
}

func cloneEvidenceValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneEvidenceMetadata(typed)
	case []map[string]any:
		items := make([]map[string]any, 0, len(typed))
		for _, entry := range typed {
			items = append(items, cloneEvidenceMetadata(entry))
		}
		return items
	case []string:
		return append([]string(nil), typed...)
	case []any:
		items := make([]any, 0, len(typed))
		for _, entry := range typed {
			items = append(items, cloneEvidenceValue(entry))
		}
		return items
	default:
		return value
	}
}

func writeSSE(w http.ResponseWriter, event, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	normalized := strings.ReplaceAll(data, "\r\n", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
}

func writeSSEJSON(w http.ResponseWriter, event string, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		writeSSE(w, event, `{"error":"encode_event_failed"}`)
		return
	}
	writeSSE(w, event, string(body))
}

func normalizedKeys(keys []string) []string {
	set := make([]string, 0, len(keys))
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			set = append(set, trimmed)
		}
	}
	return set
}

func runtimeProtectedRoute(path string) bool {
	return path == "/chat" ||
		path == "/chat/stream" ||
		path == "/experience/stream" ||
		path == "/experience/mock-voice/stream"
}

func adminRoute(path string) bool {
	return path == "/admin/" || strings.HasPrefix(path, "/admin/")
}

func (h *Handler) authorizedKey(keys []string, allowAnonymous bool, r *http.Request) (string, bool) {
	if len(keys) == 0 && allowAnonymous {
		return "anonymous", true
	}
	key, present := requestKey(r)
	if !present {
		return "", false
	}
	matched := 0
	for _, candidate := range keys {
		matched |= subtle.ConstantTimeCompare([]byte(key), []byte(candidate))
	}
	return key, matched == 1
}

func requestKey(r *http.Request) (string, bool) {
	authorization := r.Header.Get("Authorization")
	if authorization != "" {
		if !strings.HasPrefix(authorization, "Bearer ") {
			return "", false
		}
		key := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		return key, key != ""
	}
	key := strings.TrimSpace(r.Header.Get("X-API-Key"))
	return key, key != ""
}

func (h *Handler) allow(key string) bool {
	if h.rateLimitRequests <= 0 {
		return true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.requestCounts[key]++
	return h.requestCounts[key] <= h.rateLimitRequests
}

func requestIDFrom(r *http.Request) string {
	if requestID := strings.TrimSpace(r.Header.Get("X-Request-ID")); requestID != "" {
		return requestID
	}
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return "req-" + hex.EncodeToString(bytes[:])
}

func (h *Handler) authoritativeTenantID(fallback string) string {
	if h.defaultTenantID != "" {
		return h.defaultTenantID
	}
	return fallback
}

func (h *Handler) adminTenantID() string {
	return h.authoritativeTenantID("tenant-1")
}

func (h *Handler) authoritativeUserID(fallback string) string {
	if h.defaultUserID != "" {
		return h.defaultUserID
	}
	return fallback
}

func (h *Handler) applyAuthoritativeConversationIdentity(conversation *types.Conversation) {
	if conversation == nil {
		return
	}
	conversation.TenantID = h.authoritativeTenantID(conversation.TenantID)
	conversation.UserID = h.authoritativeUserID(conversation.UserID)
}

func (h *Handler) applyAuthoritativeTurnIdentity(request *types.TurnRequest) {
	if request == nil {
		return
	}
	request.TenantID = h.authoritativeTenantID(request.TenantID)
	request.UserID = h.authoritativeUserID(request.UserID)
}

type httpStreamSink struct {
	writer http.ResponseWriter
}

func (s httpStreamSink) Emit(_ context.Context, event types.StreamEvent) error {
	writeSSEJSON(s.writer, string(event.Name), event)
	if flusher, ok := s.writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

type httpPresentationSink struct {
	writer http.ResponseWriter
	events []presentation.Event
}

func (s *httpPresentationSink) Emit(_ context.Context, event presentation.Event) error {
	s.events = append(s.events, event)
	writeSSEJSON(s.writer, string(event.Name), event)
	if flusher, ok := s.writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func turnRequestFromConversation(conversation types.Conversation) (types.TurnRequest, error) {
	if strings.TrimSpace(conversation.ID) == "" || strings.TrimSpace(conversation.TenantID) == "" || strings.TrimSpace(conversation.UserID) == "" {
		return types.TurnRequest{}, fmt.Errorf("conversation identity is incomplete")
	}
	for i := len(conversation.Messages) - 1; i >= 0; i-- {
		message := conversation.Messages[i]
		if message.Role != types.RoleUser || strings.TrimSpace(message.Content) == "" {
			continue
		}
		request := types.TurnRequest{
			ConversationID: conversation.ID,
			TenantID:       conversation.TenantID,
			UserID:         conversation.UserID,
			TurnID:         message.ID,
			AttemptID:      message.ID + "-attempt-1",
			Message:        message,
			Metadata:       copyConversationMetadata(conversation.Metadata),
		}
		if err := request.Validate(); err != nil {
			return types.TurnRequest{}, err
		}
		return request, nil
	}
	return types.TurnRequest{}, fmt.Errorf("conversation requires one user message")
}

func copyConversationMetadata(metadata types.Metadata) types.Metadata {
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(types.Metadata, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
