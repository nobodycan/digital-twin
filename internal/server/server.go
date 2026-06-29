package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/observability"
	"github.com/nobodycan/digital-twin/internal/persona"
	"github.com/nobodycan/digital-twin/internal/presentation"
	"github.com/nobodycan/digital-twin/internal/runtime"
	"github.com/nobodycan/digital-twin/internal/voice"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type Config struct {
	Metrics             observability.Metrics
	Orchestrator        core.Orchestrator
	EventRecorder       *runtime.EventRecorder
	PresentationAdapter presentation.Adapter
	ASR                 voice.ASRClient
	Readiness           ReadinessConfig
	RuntimeStatus       RuntimeStatus
	PersonaAdmin        *admin.PersonaService
	MemoryAdmin         *admin.MemoryService
	KnowledgeAdmin      *admin.KnowledgeService
	ToolPolicyAdmin     *admin.ToolPolicyService
	AuditAdmin          *admin.AuditService
	StaticDir           string
	APIKeys             []string
	RateLimitRequests   int
	DefaultTenantID     string
	DefaultUserID       string
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
	mux                 *http.ServeMux
	metrics             observability.Metrics
	orchestrator        core.Orchestrator
	eventRecorder       *runtime.EventRecorder
	presentationAdapter presentation.Adapter
	asr                 voice.ASRClient
	readiness           ReadinessConfig
	runtimeStatus       RuntimeStatus
	personaAdmin        *admin.PersonaService
	memoryAdmin         *admin.MemoryService
	knowledgeAdmin      *admin.KnowledgeService
	toolPolicyAdmin     *admin.ToolPolicyService
	auditAdmin          *admin.AuditService
	staticDir           string
	apiKeys             map[string]struct{}
	rateLimitRequests   int
	defaultTenantID     string
	defaultUserID       string
	mu                  sync.Mutex
	requestCounts       map[string]int
}

func NewHandler(config Config) http.Handler {
	metrics := config.Metrics
	if metrics == nil {
		metrics = observability.NewMemoryMetrics()
	}
	handler := &Handler{
		mux:                 http.NewServeMux(),
		metrics:             metrics,
		orchestrator:        config.Orchestrator,
		eventRecorder:       config.EventRecorder,
		presentationAdapter: config.PresentationAdapter,
		asr:                 config.ASR,
		readiness:           config.Readiness,
		runtimeStatus:       config.RuntimeStatus,
		personaAdmin:        config.PersonaAdmin,
		memoryAdmin:         config.MemoryAdmin,
		knowledgeAdmin:      config.KnowledgeAdmin,
		toolPolicyAdmin:     config.ToolPolicyAdmin,
		auditAdmin:          config.AuditAdmin,
		staticDir:           config.StaticDir,
		apiKeys:             apiKeySet(config.APIKeys),
		rateLimitRequests:   config.RateLimitRequests,
		defaultTenantID:     strings.TrimSpace(config.DefaultTenantID),
		defaultUserID:       strings.TrimSpace(config.DefaultUserID),
		requestCounts:       make(map[string]int),
	}
	handler.mux.HandleFunc("GET /health", handler.handleHealth)
	handler.mux.HandleFunc("GET /ready", handler.handleReady)
	handler.mux.HandleFunc("GET /metrics", handler.handleMetrics)
	handler.mux.HandleFunc("GET /runtime/status", handler.handleRuntimeStatus)
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
	handler.mux.HandleFunc("POST /admin/knowledge/upload", handler.handleKnowledgeUpload)
	handler.mux.HandleFunc("POST /admin/knowledge/citation-test", handler.handleKnowledgeCitationTest)
	handler.mux.HandleFunc("POST /admin/tools/policy", handler.handleToolPolicySave)
	handler.mux.HandleFunc("POST /admin/tools/authorize", handler.handleToolAuthorize)
	handler.mux.HandleFunc("GET /admin/audit", handler.handleAuditRecent)
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := requestIDFrom(r)
	w.Header().Set("X-Request-ID", requestID)
	if protectedRoute(r.URL.Path) {
		key, ok := h.authorizedKey(r)
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
	version, err := h.personaAdmin.SaveDraft("tenant-1", draft)
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
	version, err := apply("tenant-1", request.VersionID)
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
	version, err := h.personaAdmin.Active("tenant-1")
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
	records, err := h.memoryAdmin.ActiveForRecall("tenant-1", "user-1")
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
	record, err := h.memoryAdmin.Disable("tenant-1", request.MemoryID)
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
	document, err := h.knowledgeAdmin.Upload("tenant-1", upload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "knowledge_upload_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, document)
}

type knowledgeCitationRequest struct {
	Query string `json:"query"`
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
	citation, err := h.knowledgeAdmin.CitationTest("tenant-1", request.Query)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "knowledge_citation_missing", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, citation)
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
	saved, err := h.toolPolicyAdmin.Save("tenant-1", policy)
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
	if err := h.toolPolicyAdmin.Authorize("tenant-1", request.PersonaID, request.ToolName); err != nil {
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
	records, err := h.auditAdmin.Recent("tenant-1")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "audit_recent_failed", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
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
		h.recordAudit(conversation, result, presentationSink.events, admin.AuditStatusCompleted, 0)
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
	h.recordAudit(conversation, result, events, admin.AuditStatusCompleted, 0)
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
	h.recordAudit(conversation, result, events, admin.AuditStatusCompleted, 0)

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

func (h *Handler) recordAudit(conversation types.Conversation, result types.AgentResult, events []presentation.Event, status admin.AuditStatus, latencyMS int64) {
	if h.auditAdmin == nil {
		return
	}
	summary := make([]string, 0, len(events))
	for _, event := range events {
		summary = append(summary, string(event.Name))
	}
	_, _ = h.auditAdmin.Record(conversation.TenantID, admin.AuditRecord{
		ConversationID: conversation.ID,
		UserID:         conversation.UserID,
		Status:         status,
		AgentName:      result.AgentName,
		LatencyMS:      latencyMS,
		EventSummary:   summary,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
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

func apiKeySet(keys []string) map[string]struct{} {
	if len(keys) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	return set
}

func protectedRoute(path string) bool {
	return path == "/chat" ||
		path == "/chat/stream" ||
		path == "/experience/stream" ||
		path == "/experience/mock-voice/stream" ||
		strings.HasPrefix(path, "/admin/persona/") ||
		strings.HasPrefix(path, "/admin/memory") ||
		strings.HasPrefix(path, "/admin/knowledge/") ||
		strings.HasPrefix(path, "/admin/tools/") ||
		path == "/admin/audit"
}

func (h *Handler) authorizedKey(r *http.Request) (string, bool) {
	if len(h.apiKeys) == 0 {
		return "anonymous", true
	}
	key := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if key == "" {
		key = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	_, ok := h.apiKeys[key]
	return key, ok
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
		}
		if err := request.Validate(); err != nil {
			return types.TurnRequest{}, err
		}
		return request, nil
	}
	return types.TurnRequest{}, fmt.Errorf("conversation requires one user message")
}
