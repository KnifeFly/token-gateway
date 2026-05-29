package builtin

import (
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
)

// ResponseGuard blocks provider responses that match configured deny terms.
type ResponseGuard struct{}

type responseGuardConfig struct {
	DenyTerms []string `json:"deny_terms"`
}

func (ResponseGuard) Name() string {
	return "response_guard"
}

func (ResponseGuard) Phase() plugin.Phase {
	return plugin.PhasePostProvider
}

func (ResponseGuard) Validate(config json.RawMessage) error {
	var cfg responseGuardConfig
	return decodeConfig(config, &cfg)
}

func (ResponseGuard) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg responseGuardConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	if term, ok := containsTerm(rawResponse(input.State), cfg.DenyTerms); ok {
		return plugin.Result{
			Action:  plugin.ActionDeny,
			Message: "response violates policy",
			AuditFields: map[string]string{
				"plugin": "response_guard",
				"phase":  string(input.Phase),
				"term":   term,
				"action": "deny",
			},
		}, nil
	}
	return plugin.Result{Action: plugin.ActionAllow}, nil
}
