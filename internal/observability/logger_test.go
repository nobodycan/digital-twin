package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestJSONLoggerWritesStructuredAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, slog.LevelDebug)

	logger.Info(context.Background(), "service started", slog.String("component", "test"), slog.Int("port", 8080))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log entry is not json: %v", err)
	}
	if entry["msg"] != "service started" {
		t.Fatalf("msg = %v, want service started", entry["msg"])
	}
	if entry["component"] != "test" {
		t.Fatalf("component = %v, want test", entry["component"])
	}
	if entry["port"] != float64(8080) {
		t.Fatalf("port = %v, want 8080", entry["port"])
	}
}
