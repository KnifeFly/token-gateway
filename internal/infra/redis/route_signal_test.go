package redis

import (
	"testing"
	"time"
)

func TestParseRouteSignalReliabilityFields(t *testing.T) {
	signal := parseRouteSignal(map[string]string{
		"healthy":           "0",
		"health_weight":     "0.25",
		"latency_ms":        "45",
		"cost_micros":       "12",
		"remaining_quota":   "7",
		"disabled":          "1",
		"model_compatible":  "0",
		"success_rate":      "0.75",
		"error_rate":        "0.25",
		"rate_limited":      "2",
		"server_errors":     "3",
		"timeouts":          "4",
		"stream_interrupts": "5",
		"circuit_state":     "half_open",
	})

	if signal.Healthy == nil || *signal.Healthy {
		t.Fatalf("healthy = %#v", signal.Healthy)
	}
	if signal.Disabled == nil || !*signal.Disabled {
		t.Fatalf("disabled = %#v", signal.Disabled)
	}
	if signal.ModelCompatible == nil || *signal.ModelCompatible {
		t.Fatalf("model compatible = %#v", signal.ModelCompatible)
	}
	if signal.HealthWeight != 0.25 || signal.Latency != 45*time.Millisecond || signal.CostMicros != 12 || signal.RemainingQuota != 7 {
		t.Fatalf("base signal = %#v", signal)
	}
	if signal.SuccessRate != 0.75 || signal.ErrorRate != 0.25 || signal.RateLimited != 2 || signal.ServerErrors != 3 || signal.Timeouts != 4 || signal.StreamInterrupts != 5 || signal.CircuitState != "half_open" {
		t.Fatalf("reliability signal = %#v", signal)
	}
}
