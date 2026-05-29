package engine

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// NoopSettlement reserves the M2 settlement hook.
type NoopSettlement struct{}

func (NoopSettlement) Settle(context.Context, *RequestState) error {
	return nil
}

// NoopObserveRecorder keeps tests and minimal setups dependency-free.
type NoopObserveRecorder struct{}

func (NoopObserveRecorder) StartSpan(ctx context.Context, _ string, _ ...attribute.KeyValue) (context.Context, trace.Span) {
	return noop.NewTracerProvider().Tracer("noop").Start(ctx, "noop")
}

func (NoopObserveRecorder) RecordProviderAttempt(context.Context, *RequestState, ProviderAttempt) {}

func (NoopObserveRecorder) FinishRequest(context.Context, *RequestState, *GatewayResponse, error) {}
