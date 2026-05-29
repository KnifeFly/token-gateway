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

type fakeAdapter struct {
	err error
}

func (a fakeAdapter) ChatCompletions(context.Context, relay.ChannelConfig, relay.ChatCompletionRequest) (*relay.Response, error) {
	if a.err != nil {
		return nil, a.err
	}
	return &relay.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(`{"id":"ok"}`)}, nil
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

type dispatchSnapshot struct{}

func (dispatchSnapshot) Ref() engine.SnapshotRef { return engine.SnapshotRef{Version: "test"} }

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
