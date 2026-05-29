package observe

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Recorder owns M1 data-plane observations.
type Recorder struct {
	logger           *slog.Logger
	tracer           trace.Tracer
	providerAttempts *prometheus.CounterVec
}

func NewRecorder(registry *prometheus.Registry, logger *slog.Logger) (*Recorder, error) {
	if logger == nil {
		logger = slog.Default()
	}
	recorder := &Recorder{
		logger: logger,
		tracer: otel.Tracer("token-gateway/dataplane"),
		providerAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "token_gateway_provider_attempts_total",
			Help: "Total provider attempts by safe routing dimensions.",
		}, []string{"provider", "channel", "model", "outcome", "error_code", "status_class"}),
	}
	if registry != nil {
		if err := registry.Register(recorder.providerAttempts); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				return nil, err
			}
		}
	}
	return recorder, nil
}

func (r *Recorder) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if r == nil || r.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return r.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func (r *Recorder) RecordProviderAttempt(_ context.Context, state *engine.RequestState, attempt engine.ProviderAttempt) {
	if r == nil {
		return
	}
	outcome := "failure"
	if attempt.Success {
		outcome = "success"
	}
	statusClass := "none"
	if attempt.StatusCode > 0 {
		statusClass = strconv.Itoa(attempt.StatusCode/100) + "xx"
	}
	if r.providerAttempts != nil {
		r.providerAttempts.WithLabelValues(
			attempt.ProviderType,
			attempt.ChannelID,
			attempt.PublicModel,
			outcome,
			attempt.ErrorCode,
			statusClass,
		).Inc()
	}
	r.logger.Info("provider_attempt",
		"request_id", state.RequestID,
		"trace_id", state.TraceID,
		"tenant_id", state.TenantID,
		"project_id", state.ProjectID,
		"model", attempt.PublicModel,
		"provider", attempt.ProviderType,
		"channel", attempt.ChannelID,
		"status", attempt.StatusCode,
		"error_code", attempt.ErrorCode,
		"success", attempt.Success,
		"duration_ms", attempt.Duration.Milliseconds(),
		"snapshot_version", state.SnapshotRef.Version,
	)
}

func (r *Recorder) FinishRequest(_ context.Context, state *engine.RequestState, response *engine.GatewayResponse, err error) {
	if r == nil || r.logger == nil || state == nil {
		return
	}
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	r.recordPluginOutputs(state)
	if err != nil {
		r.logger.Warn("gateway_request_failed",
			"request_id", state.RequestID,
			"trace_id", state.TraceID,
			"tenant_id", state.TenantID,
			"project_id", state.ProjectID,
			"status", status,
			"error", err.Error(),
		)
		return
	}
	r.logger.Info("gateway_request_finished",
		"request_id", state.RequestID,
		"trace_id", state.TraceID,
		"tenant_id", state.TenantID,
		"project_id", state.ProjectID,
		"status", status,
		"model", state.RequestedModel,
		"snapshot_version", state.SnapshotRef.Version,
	)
}

func (r *Recorder) recordPluginOutputs(state *engine.RequestState) {
	if state == nil || len(state.Internal) == 0 {
		return
	}
	if events, ok := state.Internal["audit_events"].([]map[string]string); ok {
		for _, event := range events {
			r.logger.Info("gateway_audit_event",
				"request_id", state.RequestID,
				"trace_id", state.TraceID,
				"tenant_id", state.TenantID,
				"project_id", state.ProjectID,
				"fields", event,
			)
		}
	}
	if metric, ok := state.Internal["llm_metric"].(map[string]string); ok {
		r.logger.Info("gateway_llm_metric",
			"request_id", state.RequestID,
			"trace_id", state.TraceID,
			"tenant_id", state.TenantID,
			"project_id", state.ProjectID,
			"metric", metric,
		)
	}
}
