package router

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func TestRoutePlannerSelectsChannel(t *testing.T) {
	state := &engine.RequestState{
		RequestedModel: "gpt-4o-mini",
		Principal: &engine.Principal{
			AllowedModels: []string{"gpt-4o-mini"},
		},
		Snapshot: routeSnapshot{},
	}

	err := NewRoutePlanner(NewPrioritySelector(NewWeightedRandomSelector(rand.New(rand.NewSource(1))))).Plan(context.Background(), state)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(state.RoutePlan.Candidates) != 1 {
		t.Fatalf("candidates = %d", len(state.RoutePlan.Candidates))
	}
	if state.RoutePlan.Candidates[0].ChannelID != "channel_1" {
		t.Fatalf("channel = %q", state.RoutePlan.Candidates[0].ChannelID)
	}
}

func TestRoutePlannerRejectsForbiddenModel(t *testing.T) {
	state := &engine.RequestState{
		RequestedModel: "gpt-4o-mini",
		Principal:      &engine.Principal{AllowedModels: []string{"other"}},
		Snapshot:       routeSnapshot{},
	}

	err := NewRoutePlanner(nil).Plan(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeForbidden {
		t.Fatalf("error = %v, want forbidden", err)
	}
}

func TestRoutePlannerRejectsProtocolMismatch(t *testing.T) {
	state := &engine.RequestState{
		RequestedModel: "gpt-4o-mini",
		ProtocolMode:   engine.ProtocolNativeClaude,
		Principal:      &engine.Principal{AllowedModels: []string{"gpt-4o-mini"}},
		Snapshot:       routeSnapshot{},
	}

	err := NewRoutePlanner(nil).Plan(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeInvalidArgument {
		t.Fatalf("error = %v, want invalid argument", err)
	}
}

func TestRoutePlannerSkipsEmergencyDisabledChannel(t *testing.T) {
	state := &engine.RequestState{
		RequestedModel: "gpt-4o-mini",
		Principal:      &engine.Principal{AllowedModels: []string{"gpt-4o-mini"}},
		Snapshot:       routeSnapshot{},
	}

	err := NewRoutePlanner(nil, disabledChecker{channelID: "channel_1"}).Plan(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeServiceUnavailable {
		t.Fatalf("error = %v, want service unavailable", err)
	}
}

func TestRoutePlannerStoresCircuitStates(t *testing.T) {
	state := &engine.RequestState{
		RequestedModel: "gpt-4o-mini",
		Principal:      &engine.Principal{AllowedModels: []string{"gpt-4o-mini"}},
		Snapshot:       routeSnapshot{},
	}
	provider := staticSignalProvider{signals: map[string]CandidateSignal{
		"channel_1": {Healthy: true, HealthWeight: 1, ModelCompatible: true, CircuitState: CircuitHalfOpen},
	}}

	err := NewRoutePlanner(nil).WithSignals(provider).Plan(context.Background(), state)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	states, ok := state.Internal["route.circuit_states"].(map[string]string)
	if !ok || states["channel_1"] != CircuitHalfOpen {
		t.Fatalf("circuit states = %#v", state.Internal["route.circuit_states"])
	}
}

func TestWeightedRandomDistribution(t *testing.T) {
	selector := NewWeightedRandomSelector(rand.New(rand.NewSource(9)))
	candidates := []engine.ProviderCandidate{
		{ChannelID: "a", Weight: 1},
		{ChannelID: "b", Weight: 9},
	}
	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		counts[candidates[selector.Pick(candidates)].ChannelID]++
	}
	if counts["b"] <= counts["a"]*5 {
		t.Fatalf("weighted distribution too flat: %#v", counts)
	}
}

func TestStrategyRegistryOrdersByHealthCostLatencyAndQuota(t *testing.T) {
	registry := NewStrategyRegistry(NewPrioritySelector(NewWeightedRandomSelector(rand.New(rand.NewSource(1)))))
	candidates := []engine.ProviderCandidate{
		{ChannelID: "a", Priority: 2, Weight: 100},
		{ChannelID: "b", Priority: 1, Weight: 10},
		{ChannelID: "c", Priority: 3, Weight: 50},
	}
	signals := RouteSignals{Candidates: map[string]CandidateSignal{
		"a": {Healthy: true, HealthWeight: 0.5, CostMicros: 30, Latency: 30, RemainingQuota: 10, ModelCompatible: true},
		"b": {Healthy: true, HealthWeight: 4, CostMicros: 10, Latency: 20, RemainingQuota: 5, ModelCompatible: true},
		"c": {Healthy: true, HealthWeight: 1, CostMicros: 20, Latency: 10, RemainingQuota: 50, ModelCompatible: true},
	}}

	if got := registry.Order("health_weighted", candidates, signals); got[0].ChannelID != "a" {
		t.Fatalf("health weighted first = %q", got[0].ChannelID)
	}
	if got := registry.Order("least_cost", candidates, signals); got[0].ChannelID != "b" {
		t.Fatalf("least cost first = %q", got[0].ChannelID)
	}
	if got := registry.Order("least_latency", candidates, signals); got[0].ChannelID != "c" {
		t.Fatalf("least latency first = %q", got[0].ChannelID)
	}
	if got := registry.Order("quota_aware", candidates, signals); got[0].ChannelID != "c" {
		t.Fatalf("quota aware first = %q", got[0].ChannelID)
	}
}

func TestStrategyRegistryFiltersDisabledAndIncompatibleCandidates(t *testing.T) {
	registry := NewStrategyRegistry(nil)
	candidates := []engine.ProviderCandidate{
		{ChannelID: "disabled", Priority: 1, Weight: 100},
		{ChannelID: "usable", Priority: 2, Weight: 100},
	}
	signals := RouteSignals{Candidates: map[string]CandidateSignal{
		"disabled": {Healthy: true, Disabled: true, ModelCompatible: true},
		"usable":   {Healthy: true, ModelCompatible: true},
	}}
	got := registry.Order("priority", candidates, signals)
	if len(got) != 1 || got[0].ChannelID != "usable" {
		t.Fatalf("ordered = %#v", got)
	}
}

func TestStrategyRegistryFiltersOpenCircuitCandidates(t *testing.T) {
	registry := NewStrategyRegistry(nil)
	candidates := []engine.ProviderCandidate{
		{ChannelID: "open", Priority: 1, Weight: 100},
		{ChannelID: "half_open", Priority: 2, Weight: 100},
	}
	signals := RouteSignals{Candidates: map[string]CandidateSignal{
		"open":      {Healthy: true, ModelCompatible: true, CircuitState: CircuitOpen},
		"half_open": {Healthy: true, ModelCompatible: true, CircuitState: CircuitHalfOpen},
	}}
	got := registry.Order("priority", candidates, signals)
	if len(got) != 1 || got[0].ChannelID != "half_open" {
		t.Fatalf("ordered = %#v", got)
	}
}

func TestCompositeSignalProviderKeepsUnhealthyRedisSignal(t *testing.T) {
	healthy := false
	provider := NewCompositeSignalProvider(
		NewRedisSignalProvider(fakeRouteSignalStore{signals: map[string]redisinfra.RouteSignal{
			"channel_1": {Healthy: &healthy},
		}}),
		NewCircuitBreaker(DefaultCircuitConfig()),
	)

	signals, err := provider.Signals(context.Background(), nil, []engine.ProviderCandidate{{ChannelID: "channel_1", ProviderType: "fake", PublicModel: "m"}})
	if err != nil {
		t.Fatalf("Signals() error = %v", err)
	}
	if signals.Candidates["channel_1"].Healthy {
		t.Fatalf("signal = %#v", signals.Candidates["channel_1"])
	}
}

func TestRedisSignalProviderLoadsHotSignals(t *testing.T) {
	healthy := false
	compatible := false
	disabled := true
	provider := NewRedisSignalProvider(fakeRouteSignalStore{signals: map[string]redisinfra.RouteSignal{
		"channel_1": {
			Healthy:          &healthy,
			HealthWeight:     0.25,
			Latency:          45 * time.Millisecond,
			CostMicros:       12,
			RemainingQuota:   7,
			Disabled:         &disabled,
			ModelCompatible:  &compatible,
			SuccessRate:      0.75,
			ErrorRate:        0.25,
			RateLimited:      2,
			ServerErrors:     3,
			Timeouts:         4,
			StreamInterrupts: 5,
			CircuitState:     CircuitHalfOpen,
		},
	}})

	signals, err := provider.Signals(context.Background(), nil, []engine.ProviderCandidate{{ChannelID: "channel_1"}})
	if err != nil {
		t.Fatalf("Signals() error = %v", err)
	}
	got := signals.Candidates["channel_1"]
	if got.Healthy || got.ModelCompatible || !got.Disabled || got.HealthWeight != 0.25 || got.Latency != 45*time.Millisecond || got.CostMicros != 12 || got.RemainingQuota != 7 {
		t.Fatalf("signal = %#v", got)
	}
	if got.SuccessRate != 0.75 || got.ErrorRate != 0.25 || got.RateLimited != 2 || got.ServerErrors != 3 || got.Timeouts != 4 || got.StreamInterrupts != 5 || got.CircuitState != CircuitHalfOpen {
		t.Fatalf("signal = %#v", got)
	}
}

func TestCircuitBreakerTransitionsAndSignals(t *testing.T) {
	breaker := NewCircuitBreaker(CircuitConfig{
		FailureThreshold:         2,
		MinSamples:               2,
		OpenTimeout:              time.Millisecond,
		HalfOpenSuccessThreshold: 1,
	})
	candidates := []engine.ProviderCandidate{{ChannelID: "channel_1", ProviderType: "fake", PublicModel: "m"}}
	failure := engine.ProviderAttempt{ChannelID: "channel_1", ProviderType: "fake", PublicModel: "m", ErrorCode: "provider_unavailable"}

	breaker.RecordProviderAttempt(context.Background(), nil, failure)
	breaker.RecordProviderAttempt(context.Background(), nil, failure)
	signals, err := breaker.Signals(context.Background(), nil, candidates)
	if err != nil {
		t.Fatalf("Signals() error = %v", err)
	}
	if got := signals.Candidates["channel_1"]; got.CircuitState != CircuitOpen || got.Healthy {
		t.Fatalf("open signal = %#v", got)
	}
	if ordered := NewStrategyRegistry(nil).Order("priority", candidates, signals); len(ordered) != 0 {
		t.Fatalf("ordered = %#v", ordered)
	}

	time.Sleep(2 * time.Millisecond)
	signals, err = breaker.Signals(context.Background(), nil, candidates)
	if err != nil {
		t.Fatalf("Signals() error = %v", err)
	}
	if got := signals.Candidates["channel_1"]; got.CircuitState != CircuitHalfOpen || !got.Healthy || got.HealthWeight >= 1 {
		t.Fatalf("half-open signal = %#v", got)
	}

	breaker.RecordProviderAttempt(context.Background(), nil, engine.ProviderAttempt{ChannelID: "channel_1", ProviderType: "fake", PublicModel: "m", Success: true})
	signals, err = breaker.Signals(context.Background(), nil, candidates)
	if err != nil {
		t.Fatalf("Signals() error = %v", err)
	}
	if got := signals.Candidates["channel_1"]; got.CircuitState != CircuitClosed || !got.Healthy {
		t.Fatalf("closed signal = %#v", got)
	}
}

type routeSnapshot struct{}

func (routeSnapshot) Ref() engine.SnapshotRef { return engine.SnapshotRef{Version: "test"} }

func (routeSnapshot) ListModels() []engine.ModelView { return nil }

func (routeSnapshot) LookupAPIKeyHash(string) (engine.APIKeyView, bool) {
	return engine.APIKeyView{}, false
}

func (routeSnapshot) LookupModel(model string) (engine.ModelView, bool) {
	if model != "gpt-4o-mini" {
		return engine.ModelView{}, false
	}
	return engine.ModelView{PublicModel: model, Protocol: engine.ProtocolNativeOpenAI, Enabled: true}, true
}

func (routeSnapshot) LookupRoute(model string) (engine.RoutePolicyView, bool) {
	if model != "gpt-4o-mini" {
		return engine.RoutePolicyView{}, false
	}
	return engine.RoutePolicyView{
		ID:          "route_1",
		PublicModel: model,
		Candidates:  []engine.RouteCandidateView{{ChannelID: "channel_1", Priority: 1, Weight: 100}},
	}, true
}

func (routeSnapshot) LookupChannel(channelID string) (engine.ChannelView, bool) {
	if channelID != "channel_1" {
		return engine.ChannelView{}, false
	}
	return engine.ChannelView{
		ID:           channelID,
		ProviderType: "openai_compatible",
		Enabled:      true,
		Models:       map[string]string{"gpt-4o-mini": "gpt-4o-mini"},
	}, true
}

func (routeSnapshot) LookupPrice(string) (engine.PriceRuleView, bool) {
	return engine.PriceRuleView{}, false
}

func (routeSnapshot) LookupLimit(string) (engine.LimitRuleView, bool) {
	return engine.LimitRuleView{}, false
}

func (routeSnapshot) LookupLimits(engine.LimitScope) []engine.LimitRuleView { return nil }

func (routeSnapshot) LookupPluginBindings(string) []engine.PluginBindingView {
	return nil
}

func (routeSnapshot) IsAPIKeyRevoked(string) bool {
	return false
}

type disabledChecker struct {
	providerType string
	channelID    string
}

func (c disabledChecker) IsProviderDisabled(_ context.Context, providerType string) (bool, error) {
	return c.providerType == providerType, nil
}

func (c disabledChecker) IsChannelDisabled(_ context.Context, channelID string) (bool, error) {
	return c.channelID == channelID, nil
}

type fakeRouteSignalStore struct {
	signals map[string]redisinfra.RouteSignal
}

func (s fakeRouteSignalStore) GetRouteSignals(_ context.Context, _ []string) (map[string]redisinfra.RouteSignal, error) {
	return s.signals, nil
}

type staticSignalProvider struct {
	signals map[string]CandidateSignal
}

func (p staticSignalProvider) Signals(_ context.Context, _ *engine.RequestState, candidates []engine.ProviderCandidate) (RouteSignals, error) {
	out := RouteSignals{Candidates: map[string]CandidateSignal{}}
	for _, candidate := range candidates {
		if signal, ok := p.signals[candidate.ChannelID]; ok {
			out.Candidates[candidate.ChannelID] = signal
		}
	}
	return out, nil
}
