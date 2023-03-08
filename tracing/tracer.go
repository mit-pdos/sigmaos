package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.14.0"
	"go.opentelemetry.io/otel/trace"

	db "sigmaos/debug"
	proto "sigmaos/tracing/proto"
)

const (
	SAMPLE_RATIO = 0.01
)

type HotelRequest interface {
	GetSpanContextConfig() *proto.SpanContextConfig
}

type Tracer struct {
	t trace.Tracer
}

func MakeTracer(t trace.Tracer) *Tracer {
	return &Tracer{
		t: t,
	}
}

func (t *Tracer) StartHTTPSpan(ctx context.Context, name string) trace.Span {
	_, span := t.t.Start(ctx, name)
	return span
}

func (t *Tracer) StartRPCSpan(req HotelRequest, name string) trace.Span {
	cfg := req.GetSpanContextConfig()
	ctx := contextFromConfig(cfg)
	_, span := t.t.Start(ctx, name)
	return span
}

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

func Init(svcname string, jaegerhost string) *Tracer {
	exporter := makeJaegerExporter(jaegerhost)
	// Create a sampler for the trace provider.
	//	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(SAMPLE_RATIO))
	sampler := sdktrace.AlwaysSample()
	res, err := resource.New(context.TODO(), resource.WithAttributes(semconv.ServiceNameKey.String(svcname)))
	if err != nil {
		db.DFatalf("Error resource.New: %v", err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sampler),
		sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	return MakeTracer(otel.Tracer(svcname))
}
