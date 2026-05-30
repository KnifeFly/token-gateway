package router

import (
	"context"
	"sort"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// CandidateSignal contains hot-path routing signals for one candidate.
type CandidateSignal struct {
	Healthy         bool
	HealthWeight    float64
	Latency         time.Duration
	CostMicros      int64
	RemainingQuota  int64
	Disabled        bool
	ModelCompatible bool
}

// RouteSignals is the unified signal input consumed by routing strategies.
type RouteSignals struct {
	Candidates map[string]CandidateSignal
}

// SignalProvider loads hot data and observations for route candidates.
type SignalProvider interface {
	Signals(ctx context.Context, state *engine.RequestState, candidates []engine.ProviderCandidate) (RouteSignals, error)
}

// Strategy orders route candidates using one strategy-specific signal policy.
type Strategy interface {
	Name() string
	Order(candidates []engine.ProviderCandidate, signals RouteSignals) []engine.ProviderCandidate
}

// StrategyRegistry stores named route strategies.
type StrategyRegistry struct {
	strategies map[string]Strategy
	fallback   Strategy
}

// NewStrategyRegistry returns the default P1 routing strategy registry.
func NewStrategyRegistry(selector *PrioritySelector) *StrategyRegistry {
	if selector == nil {
		selector = NewPrioritySelector(nil)
	}
	registry := &StrategyRegistry{strategies: map[string]Strategy{}}
	registry.Register(priorityStrategy{selector: selector})
	registry.Register(weightedStrategy{selector: NewWeightedRandomSelector(nil)})
	registry.Register(healthWeightedStrategy{})
	registry.Register(leastCostStrategy{})
	registry.Register(leastLatencyStrategy{})
	registry.Register(quotaAwareStrategy{fallback: selector})
	registry.fallback = registry.strategies["priority"]
	return registry
}

// Register adds or replaces a strategy by name.
func (r *StrategyRegistry) Register(strategy Strategy) {
	if r == nil || strategy == nil || strategy.Name() == "" {
		return
	}
	r.strategies[strategy.Name()] = strategy
	if r.fallback == nil {
		r.fallback = strategy
	}
}

// Order applies strategyName or the fallback priority strategy.
func (r *StrategyRegistry) Order(strategyName string, candidates []engine.ProviderCandidate, signals RouteSignals) []engine.ProviderCandidate {
	if r == nil {
		return append([]engine.ProviderCandidate(nil), candidates...)
	}
	strategy := r.strategies[strategyName]
	if strategy == nil {
		strategy = r.fallback
	}
	if strategy == nil {
		return append([]engine.ProviderCandidate(nil), candidates...)
	}
	return strategy.Order(candidates, signals)
}

func (s RouteSignals) signal(candidate engine.ProviderCandidate) CandidateSignal {
	if s.Candidates == nil {
		return CandidateSignal{Healthy: true, HealthWeight: 1, ModelCompatible: true}
	}
	signal, ok := s.Candidates[candidate.ChannelID]
	if !ok {
		return CandidateSignal{Healthy: true, HealthWeight: 1, ModelCompatible: true}
	}
	if signal.HealthWeight == 0 {
		signal.HealthWeight = 1
	}
	return signal
}

func filterUsable(candidates []engine.ProviderCandidate, signals RouteSignals) []engine.ProviderCandidate {
	out := make([]engine.ProviderCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		signal := signals.signal(candidate)
		if signal.Disabled || !signal.ModelCompatible || !signal.Healthy {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

type priorityStrategy struct {
	selector *PrioritySelector
}

func (priorityStrategy) Name() string { return "priority" }

func (s priorityStrategy) Order(candidates []engine.ProviderCandidate, signals RouteSignals) []engine.ProviderCandidate {
	return s.selector.Order(filterUsable(candidates, signals))
}

type weightedStrategy struct {
	selector *WeightedRandomSelector
}

func (weightedStrategy) Name() string { return "weighted" }

func (s weightedStrategy) Order(candidates []engine.ProviderCandidate, signals RouteSignals) []engine.ProviderCandidate {
	group := filterUsable(candidates, signals)
	var out []engine.ProviderCandidate
	for len(group) > 0 {
		selected := s.selector.Pick(group)
		out = append(out, group[selected])
		group = append(group[:selected], group[selected+1:]...)
	}
	return out
}

type healthWeightedStrategy struct{}

func (healthWeightedStrategy) Name() string { return "health_weighted" }

func (healthWeightedStrategy) Order(candidates []engine.ProviderCandidate, signals RouteSignals) []engine.ProviderCandidate {
	ordered := filterUsable(candidates, signals)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := float64(ordered[i].Weight) * signals.signal(ordered[i]).HealthWeight
		right := float64(ordered[j].Weight) * signals.signal(ordered[j]).HealthWeight
		if left == right {
			return ordered[i].Priority < ordered[j].Priority
		}
		return left > right
	})
	return ordered
}

type leastCostStrategy struct{}

func (leastCostStrategy) Name() string { return "least_cost" }

func (leastCostStrategy) Order(candidates []engine.ProviderCandidate, signals RouteSignals) []engine.ProviderCandidate {
	ordered := filterUsable(candidates, signals)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := signals.signal(ordered[i]).CostMicros
		right := signals.signal(ordered[j]).CostMicros
		if left == right {
			return ordered[i].Priority < ordered[j].Priority
		}
		if left == 0 {
			return false
		}
		if right == 0 {
			return true
		}
		return left < right
	})
	return ordered
}

type leastLatencyStrategy struct{}

func (leastLatencyStrategy) Name() string { return "least_latency" }

func (leastLatencyStrategy) Order(candidates []engine.ProviderCandidate, signals RouteSignals) []engine.ProviderCandidate {
	ordered := filterUsable(candidates, signals)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := signals.signal(ordered[i]).Latency
		right := signals.signal(ordered[j]).Latency
		if left == right {
			return ordered[i].Priority < ordered[j].Priority
		}
		if left == 0 {
			return false
		}
		if right == 0 {
			return true
		}
		return left < right
	})
	return ordered
}

type quotaAwareStrategy struct {
	fallback *PrioritySelector
}

func (quotaAwareStrategy) Name() string { return "quota_aware" }

func (s quotaAwareStrategy) Order(candidates []engine.ProviderCandidate, signals RouteSignals) []engine.ProviderCandidate {
	ordered := filterUsable(candidates, signals)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := signals.signal(ordered[i]).RemainingQuota
		right := signals.signal(ordered[j]).RemainingQuota
		if left == right {
			return ordered[i].Priority < ordered[j].Priority
		}
		return left > right
	})
	if len(ordered) == 0 && s.fallback != nil {
		return s.fallback.Order(candidates)
	}
	return ordered
}

func noRouteAvailable() error {
	return apperr.ServiceUnavailable("no provider channel is available", apperr.WithTemporary())
}
