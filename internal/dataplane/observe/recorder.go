package observe

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	metricnames "github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/redaction"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Recorder owns M1 data-plane observations.
type Recorder struct {
	logger           *slog.Logger
	tracer           trace.Tracer
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	providerAttempts *prometheus.CounterVec
	providerDuration *prometheus.HistogramVec
	firstToken       *prometheus.HistogramVec
	retries          *prometheus.CounterVec
	fallbacks        *prometheus.CounterVec
	degradations     *prometheus.CounterVec
	rateLimited      *prometheus.CounterVec
	circuitState     *prometheus.GaugeVec
	tokens           *prometheus.CounterVec
	costMicros       *prometheus.CounterVec
	idempotencyHits  *prometheus.CounterVec
}

func NewRecorder(registry *prometheus.Registry, logger *slog.Logger) (*Recorder, error) {
	if logger == nil {
		logger = slog.Default()
	}
	recorder := &Recorder{
		logger: logger,
		tracer: otel.Tracer("token-gateway/dataplane"),
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricHTTPRequestsTotal,
			Help: "Total gateway HTTP requests by safe request dimensions.",
		}, []string{"protocol", "canonical_api", "status_class", "outcome"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    metricnames.MetricHTTPRequestDurationSeconds,
			Help:    "Gateway HTTP request duration by safe request dimensions.",
			Buckets: prometheus.DefBuckets,
		}, []string{"protocol", "canonical_api", "outcome"}),
		providerAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricProviderAttemptsTotal,
			Help: "Total provider attempts by safe routing dimensions.",
		}, []string{"provider", "channel", "model", "outcome", "error_code", "status_class"}),
		providerDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    metricnames.MetricProviderAttemptDuration,
			Help:    "Provider attempt duration by provider and outcome.",
			Buckets: prometheus.DefBuckets,
		}, []string{"provider", "outcome", "status_class"}),
		firstToken: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    metricnames.MetricProviderFirstTokenLatency,
			Help:    "Provider first token latency observed at stream close.",
			Buckets: prometheus.DefBuckets,
		}, []string{"provider", "model"}),
		retries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricRetriesTotal,
			Help: "Total provider retry attempts after the first attempt.",
		}, []string{"provider", "model", "error_code"}),
		fallbacks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricFallbacksTotal,
			Help: "Total provider fallback attempts to a different channel or provider.",
		}, []string{"provider", "model"}),
		degradations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricDegradationsTotal,
			Help: "Total plugin-requested degradations.",
		}, []string{"canonical_api", "model"}),
		rateLimited: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricRateLimitRejectionsTotal,
			Help: "Total gateway-visible rate limit rejections.",
		}, []string{"canonical_api", "model"}),
		circuitState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricnames.MetricCircuitState,
			Help: "Provider circuit state gauge: 0 closed, 1 half_open, 2 open.",
		}, []string{"provider"}),
		tokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricTokensTotal,
			Help: "Total model tokens observed by kind.",
		}, []string{"model", "kind"}),
		costMicros: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricCostMicrosTotal,
			Help: "Total estimated and actual request cost in micros.",
		}, []string{"currency", "kind"}),
		idempotencyHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricIdempotencyHitsTotal,
			Help: "Total async task idempotency cache hits.",
		}, []string{"canonical_api"}),
	}
	if registry != nil {
		for _, collector := range recorder.collectors() {
			if err := registry.Register(collector); err != nil {
				if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
					return nil, err
				}
			}
		}
	}
	recorder.circuitState.WithLabelValues("none").Set(0)
	return recorder, nil
}

func (r *Recorder) collectors() []prometheus.Collector {
	return []prometheus.Collector{
		r.requestsTotal,
		r.requestDuration,
		r.providerAttempts,
		r.providerDuration,
		r.firstToken,
		r.retries,
		r.fallbacks,
		r.degradations,
		r.rateLimited,
		r.circuitState,
		r.tokens,
		r.costMicros,
		r.idempotencyHits,
	}
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
	if r.providerDuration != nil && attempt.Duration > 0 {
		r.providerDuration.WithLabelValues(attempt.ProviderType, outcome, statusClass).Observe(attempt.Duration.Seconds())
	}
	if attempt.AttemptIndex > 1 {
		if r.retries != nil {
			r.retries.WithLabelValues(attempt.ProviderType, attempt.PublicModel, safeErrorCode(attempt.ErrorCode)).Inc()
		}
		if r.fallbacks != nil && len(state.Attempts) > 0 {
			first := state.Attempts[0]
			if first.ChannelID != attempt.ChannelID || first.ProviderType != attempt.ProviderType {
				r.fallbacks.WithLabelValues(attempt.ProviderType, attempt.PublicModel).Inc()
			}
		}
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
	r.recordRequestMetrics(state, status, err)
	r.recordPluginOutputs(state)
	if err != nil {
		r.logger.Warn("gateway_request_failed",
			"request_id", state.RequestID,
			"trace_id", state.TraceID,
			"tenant_id", state.TenantID,
			"project_id", state.ProjectID,
			"status", status,
			"error", redaction.Redact(err.Error()),
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

func (r *Recorder) recordRequestMetrics(state *engine.RequestState, status int, err error) {
	outcome := "success"
	if err != nil || status >= 400 {
		outcome = "error"
	}
	statusClass := statusClass(status)
	protocol := safeProtocol(state)
	api := safeCanonicalAPI(state)
	if r.requestsTotal != nil {
		r.requestsTotal.WithLabelValues(protocol, api, statusClass, outcome).Inc()
	}
	if r.requestDuration != nil && !state.StartedAt.IsZero() {
		r.requestDuration.WithLabelValues(protocol, api, outcome).Observe(time.Since(state.StartedAt).Seconds())
	}
	if isRateLimited(err) && r.rateLimited != nil {
		r.rateLimited.WithLabelValues(api, safeModel(state.RequestedModel)).Inc()
	}
	if r.degradations != nil && (state.Metadata["plugin.suggested_model"] != "" || state.Metadata["plugin.cost_guard.suggested_model"] != "") {
		r.degradations.WithLabelValues(api, safeModel(state.RequestedModel)).Inc()
	}
	if hit, _ := state.Internal["idempotency_hit"].(bool); hit && r.idempotencyHits != nil {
		r.idempotencyHits.WithLabelValues(api).Inc()
	}
	r.recordUsageMetrics(state)
	r.recordStreamMetrics(state)
}

func (r *Recorder) recordUsageMetrics(state *engine.RequestState) {
	if r.tokens != nil {
		model := safeModel(state.RequestedModel)
		if state.ActualUsage.InputTokens > 0 {
			r.tokens.WithLabelValues(model, "input").Add(float64(state.ActualUsage.InputTokens))
		}
		if state.ActualUsage.OutputTokens > 0 {
			r.tokens.WithLabelValues(model, "output").Add(float64(state.ActualUsage.OutputTokens))
		}
		if state.ActualUsage.TotalTokens > 0 {
			r.tokens.WithLabelValues(model, "total").Add(float64(state.ActualUsage.TotalTokens))
		}
	}
	if r.costMicros != nil {
		currency := state.Currency
		if currency == "" {
			currency = state.PriceRule.Currency
		}
		if currency == "" {
			currency = "unknown"
		}
		if state.EstimatedChargeMicros > 0 {
			r.costMicros.WithLabelValues(currency, "estimated").Add(float64(state.EstimatedChargeMicros))
		}
		if state.ActualChargeMicros > 0 {
			r.costMicros.WithLabelValues(currency, "actual").Add(float64(state.ActualChargeMicros))
		}
	}
}

func (r *Recorder) recordStreamMetrics(state *engine.RequestState) {
	if r.firstToken == nil || state.ProviderResult == nil {
		return
	}
	latency, ok := state.Internal["stream_first_token_latency_ms"].(int64)
	if !ok || latency <= 0 {
		return
	}
	r.firstToken.WithLabelValues(
		state.ProviderResult.Candidate.ProviderType,
		safeModel(state.ProviderResult.Candidate.PublicModel),
	).Observe(float64(latency) / 1000)
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

func statusClass(status int) string {
	if status <= 0 {
		return "none"
	}
	return strconv.Itoa(status/100) + "xx"
}

func safeProtocol(state *engine.RequestState) string {
	if state == nil || state.ProtocolMode == "" {
		return "unknown"
	}
	return string(state.ProtocolMode)
}

func safeCanonicalAPI(state *engine.RequestState) string {
	if state == nil || state.CanonicalAPI == "" {
		return "unknown"
	}
	return string(state.CanonicalAPI)
}

func safeModel(model string) string {
	if model == "" {
		return "unknown"
	}
	return model
}

func safeErrorCode(code string) string {
	if code == "" {
		return "none"
	}
	return code
}

func isRateLimited(err error) bool {
	appErr, ok := apperr.As(err)
	return ok && appErr.Code == apperr.CodeRateLimited
}
