package engine

import (
	"context"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// NoopSettlement reserves the M2 settlement hook.
type NoopSettlement struct{}

func (NoopSettlement) Settle(context.Context, *RequestState) error {
	return nil
}

func (NoopSettlement) RecordFailed(context.Context, *RequestState, error) error {
	return nil
}

// NoopAdmission skips balance reservation when billing is disabled.
type NoopAdmission struct{}

func (NoopAdmission) Reserve(context.Context, *RequestState) error {
	return nil
}

func (NoopAdmission) Release(context.Context, *RequestState, error) error {
	return nil
}

// NoopLimitEnforcer skips distributed limits when Redis limits are disabled.
type NoopLimitEnforcer struct{}

func (NoopLimitEnforcer) Acquire(context.Context, *RequestState) (LimitRelease, error) {
	return noopLimitRelease{}, nil
}

type noopLimitRelease struct{}

func (noopLimitRelease) Release(context.Context) error {
	return nil
}

// NoopStreamFinalizer returns provider streams without extra wrapping.
type NoopStreamFinalizer struct{}

func (NoopStreamFinalizer) Wrap(_ context.Context, _ *RequestState, result *ProviderResult) (*GatewayResponse, error) {
	if result == nil {
		return nil, nil
	}
	return result.Response, nil
}

// NoopTaskBridge rejects async task operations when no task service is configured.
type NoopTaskBridge struct{}

func (NoopTaskBridge) CheckIdempotency(context.Context, *RequestState) (*GatewayResponse, bool, error) {
	return nil, false, nil
}

func (NoopTaskBridge) CreateAndDispatch(context.Context, *RequestState) (*GatewayResponse, error) {
	return nil, apperr.ConfigUnavailable("task bridge is unavailable")
}

func (NoopTaskBridge) HandleTaskOperation(context.Context, *RequestState) (*GatewayResponse, error) {
	return nil, apperr.ConfigUnavailable("task bridge is unavailable")
}

// NoopFileService rejects file operations when no file service is configured.
type NoopFileService struct{}

func (NoopFileService) HandleFileOperation(context.Context, *RequestState) (*GatewayResponse, error) {
	return nil, apperr.ConfigUnavailable("file service is unavailable")
}

// NoopPluginManager skips all data-plane plugin phases.
type NoopPluginManager struct{}

func (NoopPluginManager) Run(context.Context, string, *RequestState) error {
	return nil
}

// NoopObserveRecorder keeps tests and minimal setups dependency-free.
type NoopObserveRecorder struct{}

func (NoopObserveRecorder) StartSpan(ctx context.Context, _ string, _ ...attribute.KeyValue) (context.Context, trace.Span) {
	return noop.NewTracerProvider().Tracer("noop").Start(ctx, "noop")
}

func (NoopObserveRecorder) RecordProviderAttempt(context.Context, *RequestState, ProviderAttempt) {}

func (NoopObserveRecorder) FinishRequest(context.Context, *RequestState, *GatewayResponse, error) {}
