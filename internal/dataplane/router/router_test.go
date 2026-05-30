package router

import (
	"context"
	"math/rand"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
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
