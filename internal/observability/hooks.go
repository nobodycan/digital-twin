package observability

import "context"

// MetricsExporter exposes collected metrics in a transport-friendly format.
// Prometheus exposition is the default intended implementation in later phases.
type MetricsExporter interface {
	Export(snapshot MetricsSnapshot) ([]byte, string, error)
}

// TraceSpan is the minimal trace lifecycle hook used before a concrete tracing
// provider, such as OpenTelemetry, is wired in.
type TraceSpan interface {
	End(err error)
}

// Tracer starts named spans and returns the derived context.
type Tracer interface {
	Start(ctx context.Context, name string, attrs map[string]string) (context.Context, TraceSpan)
}

// NoopTracer is a zero-cost tracing hook for tests and local defaults.
type NoopTracer struct{}

// Start returns the original context and a no-op span.
func (NoopTracer) Start(ctx context.Context, _ string, _ map[string]string) (context.Context, TraceSpan) {
	if ctx == nil {
		ctx = context.Background()
	}
	return ctx, noopSpan{}
}

type noopSpan struct{}

func (noopSpan) End(error) {}
