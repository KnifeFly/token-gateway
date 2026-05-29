package telemetry

import (
	"strings"
	"testing"
)

func TestMetricNamesUseTokenGatewayPrefix(t *testing.T) {
	names := []string{
		MetricHTTPRequestsTotal,
		MetricHTTPRequestDurationSeconds,
		MetricProviderAttemptsTotal,
		MetricProviderAttemptDuration,
		MetricSettlementFailuresTotal,
		MetricSnapshotStalenessSeconds,
	}
	for _, name := range names {
		if !strings.HasPrefix(name, "token_gateway_") {
			t.Fatalf("metric %q does not use token_gateway_ prefix", name)
		}
	}
}

func TestSafeMetricLabelsExcludeSensitiveFields(t *testing.T) {
	blocked := map[string]bool{
		"request_id": true,
		"trace_id":   true,
		"api_key":    true,
		"prompt":     true,
		"response":   true,
	}
	for _, label := range SafeMetricLabels {
		if blocked[label] {
			t.Fatalf("unsafe metric label %q", label)
		}
	}
}
