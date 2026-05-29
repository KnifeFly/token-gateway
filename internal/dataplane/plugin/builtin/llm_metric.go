package builtin

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
)

// LLMMetric records safe per-request usage and cost dimensions.
type LLMMetric struct{}

type llmMetricConfig struct {
	Enabled bool `json:"enabled"`
}

func (LLMMetric) Name() string {
	return "llm_metric"
}

func (LLMMetric) Phase() plugin.Phase {
	return plugin.PhaseAudit
}

func (LLMMetric) Validate(config json.RawMessage) error {
	var cfg llmMetricConfig
	return decodeConfig(config, &cfg)
}

func (LLMMetric) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	if input.State == nil {
		return plugin.Result{}, nil
	}
	if input.State.Internal == nil {
		input.State.Internal = map[string]any{}
	}
	input.State.Internal["llm_metric"] = map[string]string{
		"request_id":              input.State.RequestID,
		"tenant_id":               input.State.TenantID,
		"project_id":              input.State.ProjectID,
		"model":                   input.State.RequestedModel,
		"estimated_input_tokens":  strconv.FormatInt(input.State.EstimatedUsage.InputTokens, 10),
		"estimated_output_tokens": strconv.FormatInt(input.State.EstimatedUsage.OutputTokens, 10),
		"actual_input_tokens":     strconv.FormatInt(input.State.ActualUsage.InputTokens, 10),
		"actual_output_tokens":    strconv.FormatInt(input.State.ActualUsage.OutputTokens, 10),
		"estimated_charge_micros": strconv.FormatInt(input.State.EstimatedChargeMicros, 10),
		"actual_charge_micros":    strconv.FormatInt(input.State.ActualChargeMicros, 10),
		"snapshot_version":        input.State.SnapshotRef.Version,
	}
	return plugin.Result{Action: plugin.ActionAudit, AuditFields: map[string]string{"plugin": "llm_metric", "action": "record"}}, nil
}
