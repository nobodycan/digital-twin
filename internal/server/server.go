package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/observability"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type Config struct {
	Metrics           observability.Metrics
	Orchestrator      core.Orchestrator
	APIKeys           []string
	RateLimitRequests int
}

type Handler struct {
	mux               *http.ServeMux
	metrics           observability.Metrics
	orchestrator      core.Orchestrator
	apiKeys           map[string]struct{}
	rateLimitRequests int
	mu                sync.Mutex
	requestCounts     map[string]int
}

func NewHandler(config Config) http.Handler {
	metrics := config.Metrics
	if metrics == nil {
		metrics = observability.NewMemoryMetrics()
	}
	handler := &Handler{
		mux:               http.NewServeMux(),
		metrics:           metrics,
		orchestrator:      config.Orchestrator,
		apiKeys:           apiKeySet(config.APIKeys),
		rateLimitRequests: config.RateLimitRequests,
		requestCounts:     make(map[string]int),
	}
	handler.mux.HandleFunc("GET /health", handler.handleHealth)
	handler.mux.HandleFunc("GET /metrics", handler.handleMetrics)
	handler.mux.HandleFunc("POST /chat", handler.handleChat)
	handler.mux.HandleFunc("POST /chat/stream", handler.handleChatStream)
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.orchestrator.Handle(r.Context(), conversation)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "orchestrator_error", "cause": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if h.orchestrator == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "orchestrator_unavailable"})
		return
	}
	var conversation types.Conversation
	if err := json.NewDecoder(r.Body).Decode(&conversation); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	result, err := h.orchestrator.Handle(r.Context(), conversation)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "orchestrator_error", "cause": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	writeSSE(w, "message_completed", result.Message.Content)
	writeSSE(w, "done", "ok")
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
	return path == "/chat" || path == "/chat/stream"
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
