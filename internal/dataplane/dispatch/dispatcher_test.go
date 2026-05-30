package dispatch

import (
	"context"
	"net/http"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/provider"
	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func TestDispatcherSuccess(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("fake", fakeAdapter{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	state := dispatchState()

	result, err := New(registry, nil, nil, nil).Dispatch(context.Background(), state)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result.Response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", result.Response.StatusCode)
	}
	if len(state.Attempts) != 1 || !state.Attempts[0].Success {
		t.Fatalf("attempts = %#v", state.Attempts)
	}
}

func TestDispatcherMapsProvider429(t *testing.T) {
	registry := provider.NewRegistry()
	_ = registry.Register("fake", fakeAdapter{err: &relay.ProviderError{
		StatusCode: http.StatusTooManyRequests,
		Code:       "provider_rate_limited",
		Retryable:  true,
	}})
	_, err := New(registry, nil, nil, nil).Dispatch(context.Background(), dispatchState())
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeRateLimited {
		t.Fatalf("error = %v, want rate limited", err)
	}
}

func TestDispatcherMapsProvider401(t *testing.T) {
	registry := provider.NewRegistry()
	_ = registry.Register("fake", fakeAdapter{err: &relay.ProviderError{
		StatusCode: http.StatusUnauthorized,
		Code:       "provider_auth_failed",
		Retryable:  false,
	}})
	_, err := New(registry, nil, nil, nil).Dispatch(context.Background(), dispatchState())
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeProviderError || appErr.Temporary {
		t.Fatalf("error = %#v, want non-temporary provider error", appErr)
	}
}

func TestDispatcherResolvesEncryptedCredential(t *testing.T) {
	registry := provider.NewRegistry()
	adapter := captureAdapter{}
	if err := registry.Register("fake", &adapter); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	resolver := staticCredentialResolver{apiKey: "resolved-key"}
	_, err := NewWithCredentials(registry, nil, nil, resolver, nil).Dispatch(context.Background(), dispatchState())
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if adapter.apiKey != "resolved-key" {
		t.Fatalf("api key = %q", adapter.apiKey)
	}
}

func TestDispatcherFallsBackOnRetryableProviderError(t *testing.T) {
	registry := provider.NewRegistry()
	adapter := &channelAdapter{results: map[string]relayResult{
		"channel_1": {err: &relay.ProviderError{StatusCode: http.StatusTooManyRequests, Code: "provider_rate_limited", Retryable: true}},
		"channel_2": {response: okRelayResponse()},
	}}
	if err := registry.Register("fake", adapter); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	reliability := &captureReliability{}
	state := dispatchStateWithCandidates("channel_1", "channel_2")

	result, err := New(registry, nil, nil, nil).WithReliability(reliability).Dispatch(context.Background(), state)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result.Candidate.ChannelID != "channel_2" {
		t.Fatalf("channel = %q", result.Candidate.ChannelID)
	}
	if len(adapter.calls) != 2 {
		t.Fatalf("calls = %#v", adapter.calls)
	}
	if len(state.Attempts) != 2 {
		t.Fatalf("attempts = %#v", state.Attempts)
	}
	first, second := state.Attempts[0], state.Attempts[1]
	if !first.Retryable || first.Final || first.RetryBudgetConsumed != 1 || first.RetryBudgetRemaining != 1 {
		t.Fatalf("first attempt = %#v", first)
	}
	if !second.Success || !second.Final || second.FallbackFromChannelID != "channel_1" || second.RetryBudgetConsumed != 2 || second.RetryBudgetRemaining != 0 || second.CircuitState != "half_open" {
		t.Fatalf("second attempt = %#v", second)
	}
	if len(reliability.attempts) != 2 {
		t.Fatalf("reliability attempts = %#v", reliability.attempts)
	}
}

func TestDispatcherDoesNotFallbackOnNonRetryableProviderError(t *testing.T) {
	registry := provider.NewRegistry()
	adapter := &channelAdapter{results: map[string]relayResult{
		"channel_1": {err: &relay.ProviderError{StatusCode: http.StatusUnauthorized, Code: "provider_auth_failed", Retryable: false}},
		"channel_2": {response: okRelayResponse()},
	}}
	_ = registry.Register("fake", adapter)
	state := dispatchStateWithCandidates("channel_1", "channel_2")

	_, err := New(registry, nil, nil, nil).Dispatch(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeProviderError || appErr.Temporary {
		t.Fatalf("error = %#v, want non-temporary provider error", appErr)
	}
	if len(adapter.calls) != 1 || adapter.calls[0] != "channel_1" {
		t.Fatalf("calls = %#v", adapter.calls)
	}
	if len(state.Attempts) != 1 || !state.Attempts[0].Final || state.Attempts[0].Retryable {
		t.Fatalf("attempts = %#v", state.Attempts)
	}
}

func TestDispatcherStopsAtRetryBudget(t *testing.T) {
	registry := provider.NewRegistry()
	adapter := &channelAdapter{results: map[string]relayResult{
		"channel_1": {err: &relay.ProviderError{StatusCode: http.StatusServiceUnavailable, Code: "provider_unavailable", Retryable: true}},
		"channel_2": {err: &relay.ProviderError{StatusCode: http.StatusGatewayTimeout, Code: "provider_timeout", Retryable: true}},
		"channel_3": {response: okRelayResponse()},
	}}
	_ = registry.Register("fake", adapter)
	state := dispatchStateWithCandidates("channel_1", "channel_2", "channel_3")

	_, err := New(registry, nil, nil, nil).WithRetryPolicy(RetryPolicy{MaxAttempts: 2}).Dispatch(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeProviderError || !appErr.Temporary {
		t.Fatalf("error = %#v, want temporary provider error", appErr)
	}
	if len(adapter.calls) != 2 {
		t.Fatalf("calls = %#v", adapter.calls)
	}
	last := state.Attempts[len(state.Attempts)-1]
	if !last.Final || last.RetryBudgetConsumed != 2 || last.RetryBudgetRemaining != 0 {
		t.Fatalf("last attempt = %#v", last)
	}
}

func TestDispatcherRequiresReplayableRequestForFallback(t *testing.T) {
	registry := provider.NewRegistry()
	adapter := &channelAdapter{results: map[string]relayResult{
		"channel_1": {err: &relay.ProviderError{StatusCode: http.StatusTooManyRequests, Code: "provider_rate_limited", Retryable: true}},
		"channel_2": {response: okRelayResponse()},
	}}
	_ = registry.Register("fake", adapter)
	state := dispatchStateWithCandidates("channel_1", "channel_2")
	state.Parsed.RawBody = nil

	_, err := New(registry, nil, nil, nil).Dispatch(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeRateLimited {
		t.Fatalf("error = %#v, want rate limited", appErr)
	}
	if len(adapter.calls) != 1 {
		t.Fatalf("calls = %#v", adapter.calls)
	}
	if len(state.Attempts) != 1 || !state.Attempts[0].Final || state.Attempts[0].RetryBudgetRemaining != 1 {
		t.Fatalf("attempts = %#v", state.Attempts)
	}
}

type staticCredentialResolver struct {
	apiKey string
}

func (r staticCredentialResolver) ResolveProviderAPIKey(context.Context, engine.ChannelView) (string, error) {
	return r.apiKey, nil
}

type captureAdapter struct {
	apiKey string
}

func (a *captureAdapter) Relay(_ context.Context, channel relay.ChannelConfig, _ relay.Request) (*relay.Response, error) {
	a.apiKey = channel.APIKey
	return &relay.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(`{"id":"ok"}`)}, nil
}

type fakeAdapter struct {
	err error
}

func (a fakeAdapter) Relay(context.Context, relay.ChannelConfig, relay.Request) (*relay.Response, error) {
	if a.err != nil {
		return nil, a.err
	}
	return &relay.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(`{"id":"ok"}`)}, nil
}

type relayResult struct {
	response *relay.Response
	err      error
}

type channelAdapter struct {
	results map[string]relayResult
	calls   []string
}

func (a *channelAdapter) Relay(_ context.Context, channel relay.ChannelConfig, _ relay.Request) (*relay.Response, error) {
	a.calls = append(a.calls, channel.ChannelID)
	result, ok := a.results[channel.ChannelID]
	if !ok {
		return okRelayResponse(), nil
	}
	if result.err != nil {
		return nil, result.err
	}
	if result.response != nil {
		return result.response, nil
	}
	return okRelayResponse(), nil
}

func okRelayResponse() *relay.Response {
	return &relay.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(`{"id":"ok"}`)}
}

type captureReliability struct {
	attempts []engine.ProviderAttempt
}

func (r *captureReliability) RecordProviderAttempt(_ context.Context, _ *engine.RequestState, attempt engine.ProviderAttempt) {
	r.attempts = append(r.attempts, attempt)
}

func dispatchState() *engine.RequestState {
	return &engine.RequestState{
		RequestID: "req_1",
		Snapshot:  dispatchSnapshot{},
		Parsed:    engine.ParsedRequest{RawBody: []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)},
		RoutePlan: &engine.RoutePlan{Candidates: []engine.ProviderCandidate{{
			ChannelID:     "channel_1",
			ProviderType:  "fake",
			PublicModel:   "m",
			UpstreamModel: "m",
		}}},
	}
}

func dispatchStateWithCandidates(channelIDs ...string) *engine.RequestState {
	state := dispatchState()
	state.RoutePlan.Candidates = make([]engine.ProviderCandidate, 0, len(channelIDs))
	circuitStates := make(map[string]string, len(channelIDs))
	for _, channelID := range channelIDs {
		state.RoutePlan.Candidates = append(state.RoutePlan.Candidates, engine.ProviderCandidate{
			ChannelID:     channelID,
			ProviderType:  "fake",
			PublicModel:   "m",
			UpstreamModel: "m",
		})
		circuitStates[channelID] = "closed"
	}
	if len(channelIDs) > 1 {
		circuitStates[channelIDs[1]] = "half_open"
	}
	state.Internal = map[string]any{"route.circuit_states": circuitStates}
	return state
}

type dispatchSnapshot struct{}

func (dispatchSnapshot) Ref() engine.SnapshotRef { return engine.SnapshotRef{Version: "test"} }

func (dispatchSnapshot) ListModels() []engine.ModelView { return nil }

func (dispatchSnapshot) LookupAPIKeyHash(string) (engine.APIKeyView, bool) {
	return engine.APIKeyView{}, false
}

func (dispatchSnapshot) LookupModel(string) (engine.ModelView, bool) {
	return engine.ModelView{}, false
}

func (dispatchSnapshot) LookupRoute(string) (engine.RoutePolicyView, bool) {
	return engine.RoutePolicyView{}, false
}

func (dispatchSnapshot) LookupChannel(channelID string) (engine.ChannelView, bool) {
	return engine.ChannelView{
		ID:           channelID,
		ProviderType: "fake",
		BaseURL:      "mock://fake",
		Enabled:      true,
	}, true
}

func (dispatchSnapshot) LookupPrice(string) (engine.PriceRuleView, bool) {
	return engine.PriceRuleView{}, false
}

func (dispatchSnapshot) LookupLimit(string) (engine.LimitRuleView, bool) {
	return engine.LimitRuleView{}, false
}

func (dispatchSnapshot) LookupLimits(engine.LimitScope) []engine.LimitRuleView { return nil }

func (dispatchSnapshot) LookupPluginBindings(string) []engine.PluginBindingView {
	return nil
}

func (dispatchSnapshot) IsAPIKeyRevoked(string) bool {
	return false
}
