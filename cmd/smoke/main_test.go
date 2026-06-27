package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSmokeRequiresBaseURL(t *testing.T) {
	err := run(smokeConfig{})
	if err == nil || !strings.Contains(err.Error(), "base URL is required") {
		t.Fatalf("run() error = %v, want missing base URL", err)
	}
}

func TestSmokeChecksRuntimeEndpoints(t *testing.T) {
	seen := map[string]bool{}
	requestBodies := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.Path] = true
		if r.Body != nil {
			data, _ := io.ReadAll(r.Body)
			requestBodies[r.Method+" "+r.URL.Path] = string(data)
		}
		switch r.URL.Path {
		case "/health", "/ready":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/chat":
			if r.Header.Get("Authorization") != "Bearer test-key" {
				http.Error(w, "missing auth", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"agent_name":"persona-agent","message":{"role":"assistant","content":"ok"}}`))
		case "/chat/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: done\ndata: ok\n\n"))
		case "/app", "/admin":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/metrics":
			_, _ = w.Write([]byte("requests_total 1\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	err := run(smokeConfig{BaseURL: server.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	for _, want := range []string{
		"GET /health",
		"GET /ready",
		"POST /chat",
		"POST /chat/stream",
		"GET /app",
		"GET /admin",
		"GET /metrics",
	} {
		if !seen[want] {
			t.Fatalf("missing smoke request %s; seen %#v", want, seen)
		}
	}
	if !strings.Contains(requestBodies["POST /chat/stream"], `"turn_id":"smoke-turn-1"`) {
		t.Fatalf("/chat/stream body = %s, want TurnRequest", requestBodies["POST /chat/stream"])
	}
}

func TestSmokeReturnsUsefulEndpointError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	err := run(smokeConfig{BaseURL: server.URL})
	if err == nil || !strings.Contains(err.Error(), "GET /ready returned 503") {
		t.Fatalf("run() error = %v, want ready status detail", err)
	}
}
