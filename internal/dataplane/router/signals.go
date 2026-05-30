package router

import (
	"context"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

// CompositeSignalProvider merges multiple hot signal providers.
type CompositeSignalProvider struct {
	providers []SignalProvider
}

// NewCompositeSignalProvider returns a signal provider that merges later providers over earlier providers.
func NewCompositeSignalProvider(providers ...SignalProvider) *CompositeSignalProvider {
	return &CompositeSignalProvider{providers: providers}
}

// Signals loads and merges route signals.
func (p *CompositeSignalProvider) Signals(ctx context.Context, state *engine.RequestState, candidates []engine.ProviderCandidate) (RouteSignals, error) {
	out := RouteSignals{Candidates: map[string]CandidateSignal{}}
	for _, candidate := range candidates {
		out.Candidates[candidate.ChannelID] = defaultCandidateSignal()
	}
	if p == nil {
		return out, nil
	}
	for _, provider := range p.providers {
		if provider == nil {
			continue
		}
		loaded, err := provider.Signals(ctx, state, candidates)
		if err != nil {
			return RouteSignals{}, err
		}
		for channelID, signal := range loaded.Candidates {
			out.Candidates[channelID] = mergeSignal(out.Candidates[channelID], signal)
		}
	}
	return out, nil
}

func mergeSignal(base CandidateSignal, update CandidateSignal) CandidateSignal {
	base.Healthy = base.Healthy && update.Healthy
	if update.HealthWeight != 0 {
		base.HealthWeight = update.HealthWeight
	}
	if update.Latency != 0 {
		base.Latency = update.Latency
	}
	if update.CostMicros != 0 {
		base.CostMicros = update.CostMicros
	}
	if update.RemainingQuota != 0 {
		base.RemainingQuota = update.RemainingQuota
	}
	if update.Disabled {
		base.Disabled = true
	}
	base.ModelCompatible = base.ModelCompatible && update.ModelCompatible
	if update.SuccessRate != 0 {
		base.SuccessRate = update.SuccessRate
	}
	if update.ErrorRate != 0 {
		base.ErrorRate = update.ErrorRate
	}
	if update.RateLimited != 0 {
		base.RateLimited = update.RateLimited
	}
	if update.ServerErrors != 0 {
		base.ServerErrors = update.ServerErrors
	}
	if update.Timeouts != 0 {
		base.Timeouts = update.Timeouts
	}
	if update.StreamInterrupts != 0 {
		base.StreamInterrupts = update.StreamInterrupts
	}
	if update.CircuitState != "" {
		base.CircuitState = update.CircuitState
	}
	return base
}
