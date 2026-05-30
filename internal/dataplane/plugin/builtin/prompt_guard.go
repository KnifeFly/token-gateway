package builtin

import (
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
)

// PromptGuard blocks prompts that match configured deny terms.
type PromptGuard struct{}

type promptGuardConfig struct {
	DenyTerms []string `json:"deny_terms"`
}

// Name returns the prompt guard plugin name.
func (PromptGuard) Name() string {
	return "prompt_guard"
}

// Phase returns the pre-prompt guard phase.
func (PromptGuard) Phase() plugin.Phase {
	return plugin.PhasePrePrompt
}

// Validate verifies prompt guard plugin configuration.
func (PromptGuard) Validate(config json.RawMessage) error {
	var cfg promptGuardConfig
	return decodeConfig(config, &cfg)
}

// Execute blocks prompts containing configured deny terms.
func (PromptGuard) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg promptGuardConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	if term, ok := containsTerm(rawPrompt(input.State), cfg.DenyTerms); ok {
		return plugin.Result{
			Action:  plugin.ActionDeny,
			Message: "prompt violates policy",
			AuditFields: map[string]string{
				"plugin": "prompt_guard",
				"phase":  string(input.Phase),
				"term":   term,
				"action": "deny",
			},
		}, nil
	}
	return plugin.Result{Action: plugin.ActionAllow}, nil
}
