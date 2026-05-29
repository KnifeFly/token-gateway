package builtin

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
	"github.com/KnifeFly/token-gateway/pkg/redaction"
)

// PIIRedaction redacts common PII from prompt and response payloads.
type PIIRedaction struct{}

type piiRedactionConfig struct {
	Enabled bool `json:"enabled"`
}

func (PIIRedaction) Name() string {
	return "pii_redaction"
}

func (PIIRedaction) Phase() plugin.Phase {
	return plugin.PhasePrePrompt
}

func (PIIRedaction) Validate(config json.RawMessage) error {
	var cfg piiRedactionConfig
	return decodeConfig(config, &cfg)
}

func (PIIRedaction) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg piiRedactionConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	if input.State == nil {
		return plugin.Result{}, nil
	}
	if input.Phase == plugin.PhasePostProvider && input.State.ProviderResult != nil && input.State.ProviderResult.Response != nil {
		redacted := redaction.RedactPIIBytes(input.State.ProviderResult.Response.Body)
		if !bytes.Equal(input.State.ProviderResult.Response.Body, redacted) {
			input.State.ProviderResult.Response.Body = redacted
			return plugin.Result{Action: plugin.ActionRedact, AuditFields: map[string]string{"plugin": "pii_redaction", "target": "response", "action": "redacted"}}, nil
		}
		return plugin.Result{Action: plugin.ActionAllow}, nil
	}
	redacted := redaction.RedactPIIBytes(input.State.Parsed.RawBody)
	if !bytes.Equal(input.State.Parsed.RawBody, redacted) {
		input.State.Parsed.RawBody = redacted
		return plugin.Result{Action: plugin.ActionRedact, AuditFields: map[string]string{"plugin": "pii_redaction", "target": "prompt", "action": "redacted"}}, nil
	}
	if !cfg.Enabled {
		return plugin.Result{Action: plugin.ActionAllow}, nil
	}
	return plugin.Result{Action: plugin.ActionAllow}, nil
}
