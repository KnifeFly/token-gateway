package router

import (
	"context"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
)

// RouteSignalStore loads hot route signals for channel IDs.
type RouteSignalStore interface {
	GetRouteSignals(ctx context.Context, channelIDs []string) (map[string]redisinfra.RouteSignal, error)
}

// RedisSignalProvider adapts Redis hot data into routing strategy signals.
type RedisSignalProvider struct {
	store RouteSignalStore
}

// NewRedisSignalProvider returns a routing signal provider backed by hot Redis data.
func NewRedisSignalProvider(store RouteSignalStore) *RedisSignalProvider {
	return &RedisSignalProvider{store: store}
}

// Signals loads health, latency, cost, quota, compatibility, and disable signals.
func (p *RedisSignalProvider) Signals(ctx context.Context, _ *engine.RequestState, candidates []engine.ProviderCandidate) (RouteSignals, error) {
	out := RouteSignals{Candidates: map[string]CandidateSignal{}}
	if p == nil || p.store == nil || len(candidates) == 0 {
		return out, nil
	}
	channelIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		channelIDs = append(channelIDs, candidate.ChannelID)
	}
	loaded, err := p.store.GetRouteSignals(ctx, channelIDs)
	if err != nil {
		return RouteSignals{}, err
	}
	for _, candidate := range candidates {
		signal := CandidateSignal{Healthy: true, HealthWeight: 1, ModelCompatible: true}
		if raw, ok := loaded[candidate.ChannelID]; ok {
			applyRouteSignal(&signal, raw)
		}
		out.Candidates[candidate.ChannelID] = signal
	}
	return out, nil
}

func applyRouteSignal(signal *CandidateSignal, raw redisinfra.RouteSignal) {
	if raw.Healthy != nil {
		signal.Healthy = *raw.Healthy
	}
	if raw.HealthWeight > 0 {
		signal.HealthWeight = raw.HealthWeight
	}
	if raw.Latency > 0 {
		signal.Latency = raw.Latency
	}
	if raw.CostMicros > 0 {
		signal.CostMicros = raw.CostMicros
	}
	if raw.RemainingQuota > 0 {
		signal.RemainingQuota = raw.RemainingQuota
	}
	if raw.Disabled != nil {
		signal.Disabled = *raw.Disabled
	}
	if raw.ModelCompatible != nil {
		signal.ModelCompatible = *raw.ModelCompatible
	}
}
