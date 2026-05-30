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

// Settle accepts a request without writing settlement state.
func (NoopSettlement) Settle(context.Context, *RequestState) error {
	return nil
}

// RecordFailed ignores failed settlement repair when billing is disabled.
func (NoopSettlement) RecordFailed(context.Context, *RequestState, error) error {
	return nil
}

// NoopAdmission skips balance reservation when billing is disabled.
type NoopAdmission struct{}

// Reserve accepts a request without reserving balance.
func (NoopAdmission) Reserve(context.Context, *RequestState) error {
	return nil
}

// Release ignores balance release when admission is disabled.
func (NoopAdmission) Release(context.Context, *RequestState, error) error {
	return nil
}

// NoopLimitEnforcer skips distributed limits when Redis limits are disabled.
type NoopLimitEnforcer struct{}

// Acquire returns a no-op limit release.
func (NoopLimitEnforcer) Acquire(context.Context, *RequestState) (LimitRelease, error) {
	return noopLimitRelease{}, nil
}

type noopLimitRelease struct{}

func (noopLimitRelease) Release(context.Context) error {
	return nil
}

// NoopStreamFinalizer returns provider streams without extra wrapping.
type NoopStreamFinalizer struct{}

// Wrap returns the provider response unchanged.
func (NoopStreamFinalizer) Wrap(_ context.Context, _ *RequestState, result *ProviderResult) (*GatewayResponse, error) {
	if result == nil {
		return nil, nil
	}
	return result.Response, nil
}

// NoopTaskBridge rejects async task operations when no task service is configured.
type NoopTaskBridge struct{}

// CheckIdempotency reports no async idempotency hit.
func (NoopTaskBridge) CheckIdempotency(context.Context, *RequestState) (*GatewayResponse, bool, error) {
	return nil, false, nil
}

// CreateAndDispatch rejects async task creation.
func (NoopTaskBridge) CreateAndDispatch(context.Context, *RequestState) (*GatewayResponse, error) {
	return nil, apperr.ConfigUnavailable("task bridge is unavailable")
}

// HandleTaskOperation rejects task read and cancel operations.
func (NoopTaskBridge) HandleTaskOperation(context.Context, *RequestState) (*GatewayResponse, error) {
	return nil, apperr.ConfigUnavailable("task bridge is unavailable")
}

// NoopFileService rejects file operations when no file service is configured.
type NoopFileService struct{}

// HandleFileOperation rejects file operations.
func (NoopFileService) HandleFileOperation(context.Context, *RequestState) (*GatewayResponse, error) {
	return nil, apperr.ConfigUnavailable("file service is unavailable")
}

// NoopPluginManager skips all data-plane plugin phases.
type NoopPluginManager struct{}

// Run accepts a plugin phase without executing plugins.
func (NoopPluginManager) Run(context.Context, string, *RequestState) error {
	return nil
}

// NoopPolicyEvaluator allows every request without changes.
type NoopPolicyEvaluator struct{}

// Evaluate returns an allow decision.
func (NoopPolicyEvaluator) Evaluate(context.Context, *RequestState) (PolicyDecision, error) {
	return PolicyDecision{Action: PolicyAllow}, nil
}

// NoopObserveRecorder keeps tests and minimal setups dependency-free.
type NoopObserveRecorder struct{}

// StartSpan starts a noop tracing span.
func (NoopObserveRecorder) StartSpan(ctx context.Context, _ string, _ ...attribute.KeyValue) (context.Context, trace.Span) {
	return noop.NewTracerProvider().Tracer("noop").Start(ctx, "noop")
}

// RecordProviderAttempt ignores provider attempt telemetry.
func (NoopObserveRecorder) RecordProviderAttempt(context.Context, *RequestState, ProviderAttempt) {}

// FinishRequest ignores request completion telemetry.
func (NoopObserveRecorder) FinishRequest(context.Context, *RequestState, *GatewayResponse, error) {}
