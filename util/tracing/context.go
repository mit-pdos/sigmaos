package tracing

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	db "sigmaos/debug"
	proto "sigmaos/util/tracing/proto"
)

func SpanToContext(span trace.Span) *proto.SpanContextConfig {
	ctx := span.SpanContext()
	tid := ctx.TraceID()
	sid := ctx.SpanID()
	return &proto.SpanContextConfig{
		TraceID:    tid[:],
		SpanID:     sid[:],
		TraceFlags: int32(ctx.TraceFlags()),
		TraceState: ctx.TraceState().String(),
		Remote:     ctx.IsRemote(),
	}
}

func contextFromConfig(c *proto.SpanContextConfig) context.Context {
	if c == nil {
		return context.TODO()
	}
	var tid [16]byte
	copy(tid[:], c.TraceID[0:16])
	var sid [8]byte
	copy(sid[:], c.TraceID[0:8])
	ts, err := trace.ParseTraceState(c.TraceState)
	if err != nil {
		db.DFatalf("Error parse trace state %v", err)
	}
	cfg := trace.SpanContextConfig{
		TraceID:    trace.TraceID(tid),
		SpanID:     trace.SpanID(sid),
		TraceFlags: trace.TraceFlags(c.TraceFlags),
		TraceState: ts,
		Remote:     c.Remote,
	}
	rsc := trace.NewSpanContext(cfg)
	return trace.ContextWithRemoteSpanContext(context.TODO(), rsc)
}
