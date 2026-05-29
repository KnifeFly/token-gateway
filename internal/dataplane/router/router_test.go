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

type routeSnapshot struct{}

func (routeSnapshot) Ref() engine.SnapshotRef { return engine.SnapshotRef{Version: "test"} }

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
