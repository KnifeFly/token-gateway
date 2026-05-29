package observe

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	metricnames "github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
	"github.com/prometheus/client_golang/prometheus"
)

func TestRecorderLogsPluginAuditAndMetricOutputs(t *testing.T) {
	var out bytes.Buffer
	recorder, err := NewRecorder(nil, slog.New(slog.NewJSONHandler(&out, nil)))
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}
	state := &engine.RequestState{
		RequestID: "req_1",
		TraceID:   "trace_1",
		TenantID:  "tenant_1",
		ProjectID: "project_1",
		Internal: map[string]any{
			"audit_events": []map[string]string{{"plugin": "audit_log", "email": "[REDACTED_EMAIL]"}},
			"llm_metric":   map[string]string{"model": "gpt-4o-mini", "actual_input_tokens": "10"},
		},
	}

	recorder.FinishRequest(context.Background(), state, &engine.GatewayResponse{StatusCode: 200}, nil)
	logs := out.String()
	if !strings.Contains(logs, "gateway_audit_event") {
		t.Fatalf("logs missing audit event: %s", logs)
	}
	if !strings.Contains(logs, "gateway_llm_metric") {
		t.Fatalf("logs missing metric event: %s", logs)
	}
	if strings.Contains(logs, "alice@example.com") {
		t.Fatalf("logs contain raw PII: %s", logs)
	}
}

func TestRecorderRegistersM7Metrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	recorder, err := NewRecorder(registry, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}
	state := &engine.RequestState{
		StartedAt:             time.Now().Add(-10 * time.Millisecond),
		ProtocolMode:          engine.ProtocolNativeOpenAI,
		CanonicalAPI:          engine.CanonicalOpenAIChatCompletions,
		RequestedModel:        "gpt-4o-mini",
		Currency:              "USD",
		EstimatedChargeMicros: 100,
		ActualChargeMicros:    80,
		ActualUsage:           tokenusage.Actual{InputTokens: 4, OutputTokens: 6, TotalTokens: 10},
		Metadata:              map[string]string{"plugin.suggested_model": "cheap-model"},
		Internal:              map[string]any{"idempotency_hit": true},
	}
	recorder.RecordProviderAttempt(context.Background(), state, engine.ProviderAttempt{
		AttemptIndex: 1,
		ProviderType: "fake",
		ChannelID:    "channel_1",
		PublicModel:  "gpt-4o-mini",
		Success:      true,
		StatusCode:   200,
		Duration:     5 * time.Millisecond,
	})
	recorder.FinishRequest(context.Background(), state, &engine.GatewayResponse{StatusCode: 200}, nil)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	names := map[string]bool{}
	for _, family := range families {
		names[family.GetName()] = true
	}
	for _, name := range []string{
		metricnames.MetricHTTPRequestsTotal,
		metricnames.MetricHTTPRequestDurationSeconds,
		metricnames.MetricProviderAttemptsTotal,
		metricnames.MetricProviderAttemptDuration,
		metricnames.MetricTokensTotal,
		metricnames.MetricCostMicrosTotal,
		metricnames.MetricIdempotencyHitsTotal,
		metricnames.MetricDegradationsTotal,
	} {
		if !names[name] {
			t.Fatalf("metric %q was not gathered", name)
		}
	}
}
