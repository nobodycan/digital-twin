package observability

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// PrometheusExporter renders in-memory metrics in Prometheus text exposition format.
type PrometheusExporter struct{}

// Export returns text/plain Prometheus metrics.
func (PrometheusExporter) Export(snapshot MetricsSnapshot) ([]byte, string, error) {
	var buf bytes.Buffer
	writeMetricMap(&buf, snapshot.Counters, "counter")
	writeMetricMap(&buf, snapshot.Gauges, "gauge")
	writeObservationMap(&buf, snapshot.Observations)
	return buf.Bytes(), "text/plain; version=0.0.4", nil
}

func writeMetricMap(buf *bytes.Buffer, values map[string]float64, metricType string) {
	keys := sortedKeys(values)
	for _, key := range keys {
		name := metricName(key)
		fmt.Fprintf(buf, "# TYPE %s %s\n", name, metricType)
		fmt.Fprintf(buf, "%s %v\n", key, values[key])
	}
}

func writeObservationMap(buf *bytes.Buffer, values map[string][]float64) {
	keys := sortedKeys(values)
	for _, key := range keys {
		name := metricName(key)
		fmt.Fprintf(buf, "# TYPE %s summary\n", name)
		for _, value := range values[key] {
			fmt.Fprintf(buf, "%s %v\n", key, value)
		}
	}
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func metricName(key string) string {
	if idx := strings.IndexByte(key, '{'); idx >= 0 {
		return key[:idx]
	}
	return key
}
