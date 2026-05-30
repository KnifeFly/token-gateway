// Package policy turns configured policy signals into explicit engine decisions.
package policy

import (
	"context"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const (
	metadataAction           = "policy.action"
	metadataReason           = "policy.reason"
	metadataDegradeModel     = "policy.degrade_model"
	metadataOverrideChannel  = "policy.route_override.channel_id"
	metadataOverrideProvider = "policy.route_override.provider_type"
	metadataOverridePublic   = "policy.route_override.public_model"
	metadataOverrideModel    = "policy.route_override.upstream_model"
)

// Evaluator consumes stable metadata and returns one policy decision.
type Evaluator struct{}

// NewEvaluator returns the default data-plane policy evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate returns allow, deny, degrade, or route override for the current state.
func (e *Evaluator) Evaluate(_ context.Context, state *engine.RequestState) (engine.PolicyDecision, error) {
	if state == nil || len(state.Metadata) == 0 {
		return engine.PolicyDecision{Action: engine.PolicyAllow}, nil
	}
	if model := firstNonEmpty(state.Metadata[metadataDegradeModel], state.Metadata["plugin.suggested_model"], state.Metadata["plugin.cost_guard.suggested_model"]); model != "" {
		return engine.PolicyDecision{
			Action:       engine.PolicyDegrade,
			Reason:       state.Metadata[metadataReason],
			DegradeModel: model,
		}, nil
	}
	switch state.Metadata[metadataAction] {
	case "", string(engine.PolicyAllow):
		return engine.PolicyDecision{Action: engine.PolicyAllow}, nil
	case string(engine.PolicyDeny):
		return engine.PolicyDecision{Action: engine.PolicyDeny, Reason: state.Metadata[metadataReason]}, nil
	case string(engine.PolicyRouteOverride):
		plan, err := routeOverridePlan(state)
		if err != nil {
			return engine.PolicyDecision{}, err
		}
		return engine.PolicyDecision{Action: engine.PolicyRouteOverride, Reason: state.Metadata[metadataReason], RoutePlan: plan}, nil
	default:
		return engine.PolicyDecision{}, apperr.ConfigUnavailable("policy metadata action is not supported")
	}
}

func routeOverridePlan(state *engine.RequestState) (*engine.RoutePlan, error) {
	channelID := state.Metadata[metadataOverrideChannel]
	providerType := state.Metadata[metadataOverrideProvider]
	upstreamModel := state.Metadata[metadataOverrideModel]
	publicModel := firstNonEmpty(state.Metadata[metadataOverridePublic], state.RequestedModel)
	if state.ResolvedModel.PublicModel != "" {
		publicModel = state.ResolvedModel.PublicModel
	}
	if channelID == "" || providerType == "" || upstreamModel == "" || publicModel == "" {
		return nil, apperr.ConfigUnavailable("policy route override metadata is incomplete")
	}
	return &engine.RoutePlan{
		PolicyID: "policy_override",
		Candidates: []engine.ProviderCandidate{{
			ChannelID:     channelID,
			ProviderType:  providerType,
			PublicModel:   publicModel,
			UpstreamModel: upstreamModel,
			Weight:        100,
		}},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
