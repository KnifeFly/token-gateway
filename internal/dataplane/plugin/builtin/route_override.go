package builtin

import (
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// RouteOverride emits a constrained route decision for the engine to validate.
type RouteOverride struct{}

type routeOverrideConfig struct {
	ChannelID     string `json:"channel_id"`
	ProviderType  string `json:"provider_type"`
	UpstreamModel string `json:"upstream_model"`
	Reason        string `json:"reason"`
}

func (RouteOverride) Name() string {
	return "route_override"
}

func (RouteOverride) Phase() plugin.Phase {
	return plugin.PhasePreRoute
}

func (RouteOverride) Validate(config json.RawMessage) error {
	var cfg routeOverrideConfig
	if err := decodeConfig(config, &cfg); err != nil {
		return err
	}
	if cfg.ChannelID == "" {
		return apperr.InvalidArgument("route_override channel_id is required")
	}
	return nil
}

func (RouteOverride) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg routeOverrideConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	modelName, model := requestModelView(input.State)
	if modelName == "" || input.State == nil || input.State.Snapshot == nil {
		return plugin.Result{}, apperr.ConfigUnavailable("route override requires a runtime snapshot and model")
	}
	channel, ok := input.State.Snapshot.LookupChannel(cfg.ChannelID)
	if !ok || !channel.Enabled {
		return plugin.Result{}, apperr.ConfigUnavailable("route override channel is unavailable")
	}
	if cfg.ProviderType != "" && cfg.ProviderType != channel.ProviderType {
		return plugin.Result{}, apperr.ConfigUnavailable("route override provider does not match channel")
	}
	upstreamModel := firstNonEmptyString(cfg.UpstreamModel, channel.Models[model.PublicModel])
	if upstreamModel == "" {
		return plugin.Result{}, apperr.ConfigUnavailable("route override channel does not serve model")
	}
	reason := cfg.Reason
	if reason == "" {
		reason = "plugin route override"
	}
	return plugin.Result{
		Action: plugin.ActionAllow,
		Metadata: map[string]string{
			"policy.action":                        string(engine.PolicyRouteOverride),
			"policy.reason":                        reason,
			"policy.route_override.channel_id":     cfg.ChannelID,
			"policy.route_override.provider_type":  firstNonEmptyString(cfg.ProviderType, channel.ProviderType),
			"policy.route_override.public_model":   model.PublicModel,
			"policy.route_override.upstream_model": upstreamModel,
		},
		AuditFields: map[string]string{
			"plugin":  "route_override",
			"action":  "route_override",
			"channel": cfg.ChannelID,
		},
	}, nil
}
