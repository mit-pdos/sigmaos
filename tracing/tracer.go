package tracing

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	db "sigmaos/debug"
)

const (
	SAMPLE_RATIO = 0.01
)

func makeJaegerExporter(host string) *jaeger.Exporter {
	exp, err := jaeger.New(
		jaeger.WithAgentEndpoint(
			jaeger.WithAgentHost(host),
		),
	)
	if err != nil {
		db.DFatalf("Error make Jaeger exporter: %v", err)
	}
	return exp
}

func Init(svcname string, jaegerhost string) {
	exporter := makeJaegerExporter(jaegerhost)
	// Create a sampler for the trace provider.
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(SAMPLE_RATIO))
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sampler),
		sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	// XXX set service name?
	//	sdktrace.WithResource(resource.New(standard.ServiceNameKey.String(svcname))))
}
