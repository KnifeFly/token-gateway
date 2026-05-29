package observe

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
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
