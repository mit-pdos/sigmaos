package tracing

import (
	"context"
	"sync"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Work around lack of thread-safety in the opentelemetry exporter for jaeger, which is currently broken (https://github.com/open-telemetry/opentelemetry-go/issues/3036)
// Based on the Docker buildkit peoples' solution (https://github.com/moby/buildkit/pull/3058).
type threadSafeExporterWrapper struct {
	mu       sync.Mutex
	exporter sdktrace.SpanExporter
}

func newThreadSafeExporterWrapper(exporter sdktrace.SpanExporter) sdktrace.SpanExporter {
	return &threadSafeExporterWrapper{
		exporter: exporter,
	}
}

func (tse *threadSafeExporterWrapper) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	tse.mu.Lock()
	defer tse.mu.Unlock()
	return tse.exporter.ExportSpans(ctx, spans)
}

func (tse *threadSafeExporterWrapper) Shutdown(ctx context.Context) error {
	tse.mu.Lock()
	defer tse.mu.Unlock()
	return tse.exporter.Shutdown(ctx)
}
