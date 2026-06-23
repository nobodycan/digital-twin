package voice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/config"
	"github.com/nobodycan/digital-twin/internal/observability"
)

func TestNewTTSClientDefaultsToMock(t *testing.T) {
	client, err := NewTTSClient(config.ProviderConfig{})
	if err != nil {
		t.Fatalf("NewTTSClient() error = %v", err)
	}

	result, err := client.Synthesize(context.Background(), TTSRequest{Text: "hello"})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if result.Provider != "mock" {
		t.Fatalf("provider = %q, want mock", result.Provider)
	}
}

func TestHTTPTTSClientSendsRequestAndAuthorization(t *testing.T) {
	var gotAuth string
	var gotRequest TTSRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content-type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(TTSResult{
			Provider: "http",
			Chunks: []AudioChunk{{
				Index:    0,
				MimeType: "audio/mpeg",
				Data:     []byte("audio-bytes"),
				Duration: 250 * time.Millisecond,
			}},
		})
	}))
	defer server.Close()

	client, err := NewTTSClient(config.ProviderConfig{Provider: "http", BaseURL: server.URL, APIKey: "tts-key"})
	if err != nil {
		t.Fatalf("NewTTSClient() error = %v", err)
	}

	result, err := client.Synthesize(context.Background(), TTSRequest{
		Text:    "hello voice",
		Voice:   "pro",
		Speed:   1.2,
		Emotion: "calm",
	})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	if gotAuth != "Bearer tts-key" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}
	if gotRequest.Text != "hello voice" || gotRequest.Voice != "pro" || gotRequest.Speed != 1.2 || gotRequest.Emotion != "calm" {
		t.Fatalf("request = %#v", gotRequest)
	}
	if result.Provider != "http" || len(result.Chunks) != 1 || string(result.Chunks[0].Data) != "audio-bytes" {
		t.Fatalf("result = %#v", result)
	}
}

func TestHTTPTTSClientMapsProviderFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	client, err := NewTTSClient(config.ProviderConfig{Provider: "http", BaseURL: server.URL, APIKey: "tts-key"})
	if err != nil {
		t.Fatalf("NewTTSClient() error = %v", err)
	}

	_, err = client.Synthesize(context.Background(), TTSRequest{Text: "hello"})
	if !errors.Is(err, ErrProviderFailure) {
		t.Fatalf("Synthesize() error = %v, want ErrProviderFailure", err)
	}
	if strings.Contains(err.Error(), "tts-key") {
		t.Fatalf("Synthesize() error leaked API key: %v", err)
	}
}

func TestHTTPTTSClientRecordsProviderMetricsWithoutSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(TTSResult{
			Provider: "http",
			Chunks:   []AudioChunk{{Index: 0, MimeType: "audio/mpeg", Data: []byte("audio")}},
		})
	}))
	defer server.Close()
	metrics := observability.NewMemoryMetrics()

	client, err := NewTTSClientWithMetrics(config.ProviderConfig{Provider: "http", BaseURL: server.URL, APIKey: "tts-secret"}, metrics)
	if err != nil {
		t.Fatalf("NewTTSClientWithMetrics() error = %v", err)
	}

	if _, err := client.Synthesize(context.Background(), TTSRequest{Text: "hello"}); err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	snapshot := metrics.Snapshot()
	key := `voice_provider_latency_ms{operation="tts",provider="http"}`
	if len(snapshot.Observations[key]) != 1 {
		t.Fatalf("latency observations = %#v, want one %q", snapshot.Observations, key)
	}
	for metricKey := range snapshot.Observations {
		if strings.Contains(metricKey, "tts-secret") {
			t.Fatalf("metric key leaked API key: %q", metricKey)
		}
	}
}

func TestHTTPTTSClientRecordsProviderErrorMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer server.Close()
	metrics := observability.NewMemoryMetrics()

	client, err := NewTTSClientWithMetrics(config.ProviderConfig{Provider: "http", BaseURL: server.URL, APIKey: "tts-secret"}, metrics)
	if err != nil {
		t.Fatalf("NewTTSClientWithMetrics() error = %v", err)
	}

	_, _ = client.Synthesize(context.Background(), TTSRequest{Text: "hello"})

	key := `voice_provider_errors_total{operation="tts",provider="http"}`
	if metrics.Snapshot().Counters[key] != 1 {
		t.Fatalf("error counter = %#v, want %q = 1", metrics.Snapshot().Counters, key)
	}
}

func TestHTTPTTSClientRejectsEmptyInput(t *testing.T) {
	client, err := NewTTSClient(config.ProviderConfig{Provider: "http", BaseURL: "https://tts.example.test", APIKey: "tts-key"})
	if err != nil {
		t.Fatalf("NewTTSClient() error = %v", err)
	}

	_, err = client.Synthesize(context.Background(), TTSRequest{})
	if !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("Synthesize() error = %v, want ErrEmptyInput", err)
	}
}
