package builtin

import (
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

// PromptTokenLimit rejects prompts above the configured token estimate.
type PromptTokenLimit struct{}

type promptTokenLimitConfig struct {
	MaxPromptTokens int64 `json:"max_prompt_tokens"`
}

func (PromptTokenLimit) Name() string {
	return "prompt_token_limit"
}

func (PromptTokenLimit) Phase() plugin.Phase {
	return plugin.PhasePrePrompt
}

func (PromptTokenLimit) Validate(config json.RawMessage) error {
	var cfg promptTokenLimitConfig
	return decodeConfig(config, &cfg)
}

func (PromptTokenLimit) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg promptTokenLimitConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	if cfg.MaxPromptTokens <= 0 || input.State == nil {
		return plugin.Result{Action: plugin.ActionAllow}, nil
	}
	usage := input.State.EstimatedUsage
	if usage.InputTokens == 0 && len(input.State.Parsed.RawBody) > 0 {
		usage = tokenusage.EstimateFromBytes(input.State.Parsed.RawBody)
	}
	if usage.InputTokens > cfg.MaxPromptTokens {
		return plugin.Result{Action: plugin.ActionDeny, Message: "prompt token limit exceeded"}, nil
	}
	return plugin.Result{Action: plugin.ActionAllow}, nil
}
