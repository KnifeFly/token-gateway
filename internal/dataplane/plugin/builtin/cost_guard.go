package builtin

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// CostGuard blocks or suggests a cheaper model when estimated cost exceeds policy.
type CostGuard struct{}

type costGuardConfig struct {
	MaxEstimatedMicros int64                `json:"max_estimated_micros"`
	SuggestedModel     string               `json:"suggested_model"`
	RouteOverride      *routeOverrideConfig `json:"route_override"`
}

func (CostGuard) Name() string {
	return "cost_guard"
}

func (CostGuard) Phase() plugin.Phase {
	return plugin.PhasePostRoute
}

func (CostGuard) Validate(config json.RawMessage) error {
	var cfg costGuardConfig
	return decodeConfig(config, &cfg)
}

func (CostGuard) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg costGuardConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	estimated := estimatedChargeMicros(input.State)
	if cfg.MaxEstimatedMicros <= 0 || estimated <= cfg.MaxEstimatedMicros {
		return plugin.Result{Action: plugin.ActionAllow}, nil
	}
	if cfg.RouteOverride != nil {
		channelID := cfg.RouteOverride.ChannelID
		providerType := cfg.RouteOverride.ProviderType
		upstreamModel := cfg.RouteOverride.UpstreamModel
		if channelID == "" || providerType == "" || upstreamModel == "" {
			return plugin.Result{}, apperr.InvalidArgument("cost_guard route_override requires channel_id, provider_type, and upstream_model")
		}
		return plugin.Result{
			Action: plugin.ActionAllow,
			Metadata: map[string]string{
				"policy.action":                        "route_override",
				"policy.reason":                        "estimated cost exceeds policy",
				"policy.route_override.channel_id":     channelID,
				"policy.route_override.provider_type":  providerType,
				"policy.route_override.upstream_model": upstreamModel,
			},
			AuditFields: map[string]string{
				"plugin":             "cost_guard",
				"action":             "route_override",
				"estimated_micros":   strconv.FormatInt(estimated, 10),
				"max_allowed_micros": strconv.FormatInt(cfg.MaxEstimatedMicros, 10),
			},
		}, nil
	}
	if cfg.SuggestedModel != "" {
		return plugin.Result{
			Action:         plugin.ActionDegrade,
			SuggestedModel: cfg.SuggestedModel,
			Mutations: []plugin.StateMutation{{
				Target: plugin.MutationMetadata,
				Key:    "plugin.cost_guard.suggested_model",
				Value:  cfg.SuggestedModel,
			}, {
				Target: plugin.MutationMetadata,
				Key:    "policy.degrade_model",
				Value:  cfg.SuggestedModel,
			}},
			AuditFields: map[string]string{
				"plugin":             "cost_guard",
				"action":             "degrade",
				"estimated_micros":   strconv.FormatInt(estimated, 10),
				"suggested_model":    cfg.SuggestedModel,
				"max_allowed_micros": strconv.FormatInt(cfg.MaxEstimatedMicros, 10),
			},
		}, nil
	}
	return plugin.Result{
		Action:  plugin.ActionDeny,
		Message: "request cost exceeds policy",
		AuditFields: map[string]string{
			"plugin":             "cost_guard",
			"action":             "deny",
			"estimated_micros":   strconv.FormatInt(estimated, 10),
			"max_allowed_micros": strconv.FormatInt(cfg.MaxEstimatedMicros, 10),
		},
	}, nil
}
