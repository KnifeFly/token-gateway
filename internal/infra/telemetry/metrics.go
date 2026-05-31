package telemetry

// Metric names are the public Prometheus contract for token-gateway.
const (
	MetricInfo                       = "token_gateway_info"
	MetricHTTPRequestsTotal          = "token_gateway_http_requests_total"
	MetricHTTPRequestDurationSeconds = "token_gateway_http_request_duration_seconds"
	MetricProviderAttemptsTotal      = "token_gateway_provider_attempts_total"
	MetricProviderAttemptDuration    = "token_gateway_provider_attempt_duration_seconds"
	MetricProviderFirstTokenLatency  = "token_gateway_provider_first_token_latency_seconds"
	MetricRetriesTotal               = "token_gateway_retries_total"
	MetricFallbacksTotal             = "token_gateway_fallbacks_total"
	MetricDegradationsTotal          = "token_gateway_degradations_total"
	MetricRateLimitRejectionsTotal   = "token_gateway_rate_limit_rejections_total"
	MetricCircuitState               = "token_gateway_circuit_state"
	MetricTokensTotal                = "token_gateway_tokens_total"
	MetricCostMicrosTotal            = "token_gateway_cost_micros_total"
	MetricSettlementFailuresTotal    = "token_gateway_settlement_failures_total"
	MetricFailedSettlementBacklog    = "token_gateway_failed_settlement_backlog"
	MetricSnapshotActive             = "token_gateway_snapshot_active"
	MetricSnapshotStalenessSeconds   = "token_gateway_snapshot_staleness_seconds"
	MetricSnapshotPublishErrorsTotal = "token_gateway_snapshot_publish_errors_total"
	MetricIdempotencyHitsTotal       = "token_gateway_idempotency_hits_total"
	MetricTaskLifecycleTransitions   = "token_gateway_task_lifecycle_transitions_total"
	MetricCallbackRetriesTotal       = "token_gateway_callback_retries_total"
	MetricCallbackDeliveriesTotal    = "token_gateway_callback_deliveries_total"
	MetricFileCleanupRunsTotal       = "token_gateway_file_cleanup_runs_total"
	MetricFileCleanupDeletedTotal    = "token_gateway_file_cleanup_deleted_total"
	MetricFileCleanupMaxAgeSeconds   = "token_gateway_file_cleanup_max_age_seconds"
	MetricFileCleanupNextRunSeconds  = "token_gateway_file_cleanup_next_run_timestamp_seconds"
	MetricWorkerJobRunsTotal         = "token_gateway_worker_job_runs_total"
	MetricWorkerJobDurationSeconds   = "token_gateway_worker_job_duration_seconds"
	MetricWorkerJobInFlight          = "token_gateway_worker_job_in_flight"
	MetricWorkerLeaseHeartbeatsTotal = "token_gateway_worker_lease_heartbeats_total"
	MetricRealtimeSessionsTotal      = "token_gateway_realtime_sessions_total"
	MetricRealtimeConnectionsTotal   = "token_gateway_realtime_connections_total"
)

// SafeMetricLabels documents allowed low-cardinality metric labels.
var SafeMetricLabels = []string{
	"protocol",
	"canonical_api",
	"provider",
	"channel",
	"model",
	"outcome",
	"error_code",
	"status_class",
	"reason",
	"currency",
	"kind",
	"state",
	"operation",
}
