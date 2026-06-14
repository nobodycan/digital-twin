package observability

import "sync"

// Metrics is a minimal instrumentation surface that is easy to mock in tests.
type Metrics interface {
	IncCounter(name string, labels map[string]string)
	AddCounter(name string, delta float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
	Observe(name string, value float64, labels map[string]string)
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot is a point-in-time copy of in-memory metric values.
type MetricsSnapshot struct {
	Counters     map[string]float64
	Gauges       map[string]float64
	Observations map[string][]float64
}

// MemoryMetrics stores metrics in memory for local use and tests.
type MemoryMetrics struct {
	mu           sync.RWMutex
	counters     map[string]float64
	gauges       map[string]float64
	observations map[string][]float64
}

// NewMemoryMetrics returns an empty in-memory metrics collector.
func NewMemoryMetrics() *MemoryMetrics {
	return &MemoryMetrics{
		counters:     make(map[string]float64),
		gauges:       make(map[string]float64),
		observations: make(map[string][]float64),
	}
}

func (m *MemoryMetrics) IncCounter(name string, labels map[string]string) {
	m.AddCounter(name, 1, labels)
}

func (m *MemoryMetrics) AddCounter(name string, delta float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters[metricKey(name, labels)] += delta
}

func (m *MemoryMetrics) SetGauge(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gauges[metricKey(name, labels)] = value
}

func (m *MemoryMetrics) Observe(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := metricKey(name, labels)
	m.observations[key] = append(m.observations[key], value)
}

func (m *MemoryMetrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := MetricsSnapshot{
		Counters:     copyFloatMap(m.counters),
		Gauges:       copyFloatMap(m.gauges),
		Observations: make(map[string][]float64, len(m.observations)),
	}

	for key, values := range m.observations {
		snapshot.Observations[key] = append([]float64(nil), values...)
	}

	return snapshot
}

func copyFloatMap(values map[string]float64) map[string]float64 {
	copied := make(map[string]float64, len(values))
	for key, value := range values {
		copied[key] = value
	}

	return copied
}
