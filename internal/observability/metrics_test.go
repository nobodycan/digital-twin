package observability

import "testing"

func TestMemoryMetricsSnapshotCopiesValues(t *testing.T) {
	metrics := NewMemoryMetrics()
	labels := map[string]string{"route": "/chat", "method": "POST"}

	metrics.IncCounter("requests_total", labels)
	metrics.AddCounter("requests_total", 2, labels)
	metrics.SetGauge("inflight_requests", 3, labels)
	metrics.Observe("request_duration_ms", 42, labels)

	snapshot := metrics.Snapshot()
	key := `requests_total{method="POST",route="/chat"}`
	if snapshot.Counters[key] != 3 {
		t.Fatalf("counter = %v, want 3", snapshot.Counters[key])
	}

	gaugeKey := `inflight_requests{method="POST",route="/chat"}`
	if snapshot.Gauges[gaugeKey] != 3 {
		t.Fatalf("gauge = %v, want 3", snapshot.Gauges[gaugeKey])
	}

	observationKey := `request_duration_ms{method="POST",route="/chat"}`
	if len(snapshot.Observations[observationKey]) != 1 || snapshot.Observations[observationKey][0] != 42 {
		t.Fatalf("observations = %v, want [42]", snapshot.Observations[observationKey])
	}

	snapshot.Counters[key] = 99
	if metrics.Snapshot().Counters[key] != 3 {
		t.Fatal("snapshot mutation affected metrics state")
	}
}
