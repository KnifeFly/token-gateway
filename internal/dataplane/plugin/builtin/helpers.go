package builtin

import (
	"encoding/json"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

func decodeConfig(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return apperr.InvalidArgument("plugin config must be valid json")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return apperr.InvalidArgument("plugin config must match schema", apperr.WithCause(err))
	}
	return nil
}

func rawPrompt(state *engine.RequestState) string {
	if state == nil {
		return ""
	}
	return string(state.Parsed.RawBody)
}

func rawResponse(state *engine.RequestState) string {
	if state == nil || state.ProviderResult == nil || state.ProviderResult.Response == nil {
		return ""
	}
	return string(state.ProviderResult.Response.Body)
}

func containsTerm(value string, terms []string) (string, bool) {
	value = strings.ToLower(value)
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if strings.Contains(value, term) {
			return term, true
		}
	}
	return "", false
}

func estimatedChargeMicros(state *engine.RequestState) int64 {
	if state == nil {
		return 0
	}
	if state.EstimatedChargeMicros > 0 {
		return state.EstimatedChargeMicros
	}
	if !state.PriceRule.Enabled {
		return 0
	}
	usage := state.EstimatedUsage
	if usage.InputTokens == 0 && len(state.Parsed.RawBody) > 0 {
		usage = tokenusage.EstimateFromBytes(state.Parsed.RawBody)
	}
	outputTokens := usage.OutputTokens
	if outputTokens == 0 {
		outputTokens = state.PriceRule.EstimatedOutputTokens
	}
	return usage.InputTokens*state.PriceRule.InputMicrosPerToken + outputTokens*state.PriceRule.OutputMicrosPerToken
}
