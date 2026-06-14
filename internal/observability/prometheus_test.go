package observability

import (
	"strings"
	"testing"
)

func TestPrometheusExporterRendersMetrics(t *testing.T) {
	metrics := NewMemoryMetrics()
	metrics.AddCounter("requests_total", 2, map[string]string{"method": "POST"})
	metrics.SetGauge("inflight_requests", 1, nil)
	metrics.Observe("latency_ms", 12.5, nil)

	body, contentType, err := (PrometheusExporter{}).Export(metrics.Snapshot())
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if contentType != "text/plain; version=0.0.4" {
		t.Fatalf("contentType = %q", contentType)
	}

	output := string(body)
	for _, want := range []string{
		"# TYPE requests_total counter",
		`requests_total{method="POST"} 2`,
		"# TYPE inflight_requests gauge",
		"inflight_requests 1",
		"# TYPE latency_ms summary",
		"latency_ms 12.5",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestNoopTracerHandlesNilContext(t *testing.T) {
	ctx, span := (NoopTracer{}).Start(nil, "test", nil)
	if ctx == nil {
		t.Fatal("Start returned nil context")
	}
	span.End(nil)
}
