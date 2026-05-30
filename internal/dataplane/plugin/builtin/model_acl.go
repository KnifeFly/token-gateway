package builtin

import (
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
)

// ModelACL enforces plugin-scoped model allow/deny lists after authentication.
type ModelACL struct{}

type modelACLConfig struct {
	AllowedModels []string `json:"allowed_models"`
	DeniedModels  []string `json:"denied_models"`
}

func (ModelACL) Name() string {
	return "model_acl"
}

func (ModelACL) Phase() plugin.Phase {
	return plugin.PhasePostAuth
}

func (ModelACL) Validate(config json.RawMessage) error {
	var cfg modelACLConfig
	return decodeConfig(config, &cfg)
}

func (ModelACL) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg modelACLConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	modelName, model := requestModelView(input.State)
	if modelName == "" {
		return plugin.Result{Action: plugin.ActionDeny, Message: "model is required"}, nil
	}
	if modelMatches(cfg.DeniedModels, modelName, model) {
		return plugin.Result{
			Action:  plugin.ActionDeny,
			Message: "model is denied by policy",
			AuditFields: map[string]string{
				"plugin": "model_acl",
				"action": "deny",
				"reason": "deny_list",
			},
		}, nil
	}
	if len(cfg.AllowedModels) > 0 && !modelMatches(cfg.AllowedModels, modelName, model) {
		return plugin.Result{
			Action:  plugin.ActionDeny,
			Message: "model is not allowed by policy",
			AuditFields: map[string]string{
				"plugin": "model_acl",
				"action": "deny",
				"reason": "allow_list_miss",
			},
		}, nil
	}
	return plugin.Result{Action: plugin.ActionAllow}, nil
}

func requestModelView(state *engine.RequestState) (string, engine.ModelView) {
	if state == nil {
		return "", engine.ModelView{}
	}
	modelName := firstNonEmptyString(state.RequestedModel, state.Parsed.Model)
	if state.Snapshot == nil || modelName == "" {
		return modelName, engine.ModelView{}
	}
	model, _ := state.Snapshot.LookupModel(modelName)
	return modelName, model
}

func modelMatches(values []string, requested string, model engine.ModelView) bool {
	for _, value := range values {
		if value == "*" || value == requested || value == model.PublicModel {
			return true
		}
		for _, alias := range model.Aliases {
			if value == alias {
				return true
			}
		}
	}
	return false
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
