package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func TestErrorResponseAmbiguousProtocolCode(t *testing.T) {
	response := (&GatewayEngine{}).errorResponse(&RequestState{RequestID: "req_test"}, apperr.AmbiguousProtocol("protocol is ambiguous"))

	var payload struct {
		Error struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.Error.Code != "ambiguous_protocol" {
		t.Fatalf("code = %q", payload.Error.Code)
	}
	if payload.Error.Type != "invalid_request_error" {
		t.Fatalf("type = %q", payload.Error.Type)
	}
}

func TestNewStateSeparatesInternalAndClientRequestID(t *testing.T) {
	state := newState(IncomingRequest{
		Header: http.Header{"X-Request-ID": []string{"client_req_1"}},
	})

	if state.RequestID == "" {
		t.Fatal("missing internal request id")
	}
	if state.RequestID == "client_req_1" {
		t.Fatalf("internal request id reused client value")
	}
	if state.ClientRequestID != "client_req_1" {
		t.Fatalf("client request id = %q", state.ClientRequestID)
	}
}

func TestNewStateUsesGatewayRequestIDFromTrustedMiddleware(t *testing.T) {
	state := newState(IncomingRequest{
		Header: http.Header{
			"X-Gateway-Request-ID": []string{"internal_req_1"},
			"X-Request-ID":         []string{"internal_req_1"},
			"X-Client-Request-ID":  []string{"client_req_1"},
		},
	})

	if state.RequestID != "internal_req_1" {
		t.Fatalf("request id = %q", state.RequestID)
	}
	if state.ClientRequestID != "client_req_1" {
		t.Fatalf("client request id = %q", state.ClientRequestID)
	}
}

func TestHandleDoesNotUseClientRequestIDForBillingOrLimits(t *testing.T) {
	admission := &recordingAdmissionIDs{}
	limiter := &recordingLimiterIDs{}
	settlement := &recordingSettlementIDs{}
	gateway, err := New(
		WithSnapshot(asyncSnapshotProvider{}),
		WithClassifier(syncClassifier{}),
		WithParser(syncParser{}),
		WithAuthenticator(asyncAuthenticator{}),
		WithRoutePlanner(asyncRoutePlanner{}),
		WithAdmission(admission),
		WithLimitEnforcer(limiter),
		WithDispatcher(successDispatcher{}),
		WithSettlement(settlement),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for range 2 {
		response, err := gateway.Handle(context.Background(), IncomingRequest{
			Method: http.MethodPost,
			Path:   "/v1/chat/completions",
			Header: http.Header{"X-Request-ID": []string{"client_req_reused"}},
			Body:   io.NopCloser(strings.NewReader(`{"model":"gpt-4o-mini"}`)),
		})
		if err != nil {
			t.Fatalf("Handle() error = %v", err)
		}
		if response == nil || response.StatusCode != http.StatusOK {
			t.Fatalf("response = %#v", response)
		}
	}

	if len(admission.ids) != 2 || len(limiter.ids) != 2 || len(settlement.ids) != 2 {
		t.Fatalf("ids admission=%#v limiter=%#v settlement=%#v", admission.ids, limiter.ids, settlement.ids)
	}
	if admission.ids[0] == admission.ids[1] {
		t.Fatalf("admission reused internal request id %q", admission.ids[0])
	}
	for _, id := range append(append([]string{}, admission.ids...), limiter.ids...) {
		if id == "client_req_reused" {
			t.Fatalf("client request id used internally: %#v", id)
		}
	}
}

func TestEvaluatePolicyConsumesDecision(t *testing.T) {
	engine := &GatewayEngine{policy: fakePolicy{decision: PolicyDecision{
		Action:       PolicyDegrade,
		DegradeModel: "cheap-model",
		Metadata:     map[string]string{"policy.result": "degraded"},
	}}}
	state := &RequestState{RequestedModel: "expensive-model", Metadata: map[string]string{}}

	if err := engine.evaluatePolicy(context.Background(), state); err != nil {
		t.Fatalf("evaluatePolicy() error = %v", err)
	}
	if state.RequestedModel != "cheap-model" {
		t.Fatalf("requested model = %q", state.RequestedModel)
	}
	if state.PolicyDecision.Action != PolicyDegrade || state.Metadata["policy.result"] != "degraded" {
		t.Fatalf("decision = %#v metadata = %#v", state.PolicyDecision, state.Metadata)
	}
}

func TestEvaluatePolicyRejectsDenyDecision(t *testing.T) {
	engine := &GatewayEngine{policy: fakePolicy{decision: PolicyDecision{Action: PolicyDeny, Reason: "blocked"}}}

	err := engine.evaluatePolicy(context.Background(), &RequestState{})
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodePolicyDenied {
		t.Fatalf("error = %v, want policy denied", err)
	}
}

func TestEvaluatePolicyConstrainsRouteOverride(t *testing.T) {
	gateway := &GatewayEngine{policy: fakePolicy{decision: PolicyDecision{
		Action: PolicyRouteOverride,
		RoutePlan: &RoutePlan{PolicyID: "override", Candidates: []ProviderCandidate{{
			ChannelID:     "channel_1",
			ProviderType:  "openai_compatible",
			PublicModel:   "gpt-4o-mini",
			UpstreamModel: "gpt-4o-mini-upstream",
		}}},
	}}}
	state := &RequestState{
		RequestedModel: "gpt-4o-mini",
		ProtocolMode:   ProtocolNativeOpenAI,
		Principal:      &Principal{AllowedModels: []string{"gpt-4o-mini"}},
		Snapshot:       routeOverrideSnapshot{},
	}

	if err := gateway.evaluatePolicy(context.Background(), state); err != nil {
		t.Fatalf("evaluatePolicy() error = %v", err)
	}
	if state.RoutePlan == nil || state.ResolvedModel.PublicModel != "gpt-4o-mini" {
		t.Fatalf("route plan = %#v resolved = %#v", state.RoutePlan, state.ResolvedModel)
	}
}

type fakePolicy struct {
	decision PolicyDecision
	err      error
}

func (p fakePolicy) Evaluate(context.Context, *RequestState) (PolicyDecision, error) {
	return p.decision, p.err
}

type routeOverrideSnapshot struct{}

func (routeOverrideSnapshot) Ref() SnapshotRef { return SnapshotRef{Version: "test"} }

func (routeOverrideSnapshot) ListModels() []ModelView { return nil }

func (routeOverrideSnapshot) LookupAPIKeyHash(string) (APIKeyView, bool) {
	return APIKeyView{}, false
}

func (routeOverrideSnapshot) LookupModel(model string) (ModelView, bool) {
	if model != "gpt-4o-mini" {
		return ModelView{}, false
	}
	return ModelView{PublicModel: "gpt-4o-mini", Protocol: ProtocolNativeOpenAI, Enabled: true}, true
}

func (routeOverrideSnapshot) LookupRoute(string) (RoutePolicyView, bool) {
	return RoutePolicyView{}, false
}

func (routeOverrideSnapshot) LookupChannel(channelID string) (ChannelView, bool) {
	if channelID != "channel_1" {
		return ChannelView{}, false
	}
	return ChannelView{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		Enabled:      true,
		Models:       map[string]string{"gpt-4o-mini": "gpt-4o-mini-upstream"},
	}, true
}

func (routeOverrideSnapshot) LookupPrice(string) (PriceRuleView, bool) {
	return PriceRuleView{}, false
}

func (routeOverrideSnapshot) LookupLimit(string) (LimitRuleView, bool) {
	return LimitRuleView{}, false
}

func (routeOverrideSnapshot) LookupLimits(LimitScope) []LimitRuleView { return nil }

func (routeOverrideSnapshot) LookupPluginBindings(string) []PluginBindingView { return nil }

func (routeOverrideSnapshot) IsAPIKeyRevoked(string) bool { return false }

func TestErrorResponsePolicyDeniedCode(t *testing.T) {
	response := (&GatewayEngine{}).errorResponse(&RequestState{RequestID: "req_test"}, apperr.PolicyDenied("blocked"))

	var payload struct {
		Error struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.Error.Code != "policy_denied" {
		t.Fatalf("code = %q", payload.Error.Code)
	}
	if payload.Error.Type != "permission_error" {
		t.Fatalf("type = %q", payload.Error.Type)
	}
}

func TestHandleReleasesAdmissionHoldWhenAsyncCreateReplays(t *testing.T) {
	admission := &recordingAdmission{}
	tasks := &replayCreateTaskBridge{}
	gateway, err := New(
		WithSnapshot(asyncSnapshotProvider{}),
		WithClassifier(asyncClassifier{}),
		WithParser(asyncParser{}),
		WithAuthenticator(asyncAuthenticator{}),
		WithRoutePlanner(asyncRoutePlanner{}),
		WithDispatcher(asyncDispatcher{}),
		WithAdmission(admission),
		WithLimitEnforcer(asyncLimiter{}),
		WithTaskBridge(tasks),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response, err := gateway.Handle(context.Background(), IncomingRequest{
		Method: http.MethodPost,
		Path:   "/v1/images/generations",
		Header: http.Header{"Idempotency-Key": []string{"idem_1"}},
		Body:   io.NopCloser(strings.NewReader(`{"model":"image-public","prompt":"hi"}`)),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if response == nil || response.StatusCode != http.StatusAccepted {
		t.Fatalf("response = %#v", response)
	}
	if admission.reserveCalls != 1 {
		t.Fatalf("reserve calls = %d", admission.reserveCalls)
	}
	if admission.releaseCalls != 1 {
		t.Fatalf("release calls = %d", admission.releaseCalls)
	}
	if admission.releaseHoldID != "hold_new" {
		t.Fatalf("released hold = %q", admission.releaseHoldID)
	}
	if admission.releaseCause == nil || admission.releaseCause.Error() != "async idempotency replay" {
		t.Fatalf("release cause = %v", admission.releaseCause)
	}
}

type asyncSnapshotProvider struct{}

func (asyncSnapshotProvider) Attach(_ context.Context, state *RequestState) error {
	state.PinSnapshot(routeOverrideSnapshot{})
	return nil
}

type asyncClassifier struct{}

func (asyncClassifier) Classify(_ context.Context, state *RequestState) error {
	state.CanonicalAPI = CanonicalImageGeneration
	state.Endpoint = EndpointSpec{Path: "/v1/images/generations", Canonical: CanonicalImageGeneration}
	return nil
}

type asyncParser struct{}

func (asyncParser) Parse(_ context.Context, state *RequestState) error {
	state.Async = true
	state.IdempotencyKey = "idem_1"
	state.RequestedModel = "image-public"
	state.Parsed = ParsedRequest{
		RawBody: []byte(`{"model":"image-public","prompt":"hi"}`),
		Media: &UnifiedMediaRequest{
			Kind:      "image.generation",
			MediaType: "image",
			Model:     "image-public",
		},
	}
	return nil
}

type syncClassifier struct{}

func (syncClassifier) Classify(_ context.Context, state *RequestState) error {
	state.CanonicalAPI = CanonicalOpenAIChatCompletions
	state.Endpoint = EndpointSpec{Path: "/v1/chat/completions", Canonical: CanonicalOpenAIChatCompletions}
	return nil
}

type syncParser struct{}

func (syncParser) Parse(_ context.Context, state *RequestState) error {
	state.RequestedModel = "gpt-4o-mini"
	state.Parsed = ParsedRequest{RawBody: []byte(`{"model":"gpt-4o-mini"}`)}
	return nil
}

type asyncAuthenticator struct{}

func (asyncAuthenticator) Authenticate(_ context.Context, state *RequestState) error {
	state.TenantID = "tenant_1"
	state.ProjectID = "project_1"
	state.APIKeyID = "key_1"
	state.Principal = &Principal{TenantID: state.TenantID, ProjectID: state.ProjectID, APIKeyID: state.APIKeyID, AllowedModels: []string{"*"}}
	return nil
}

type asyncRoutePlanner struct{}

func (asyncRoutePlanner) Plan(_ context.Context, state *RequestState) error {
	state.RoutePlan = &RoutePlan{Candidates: []ProviderCandidate{{ProviderType: "mock_media", ChannelID: "channel_1", PublicModel: "image-public"}}}
	return nil
}

type recordingAdmission struct {
	reserveCalls  int
	releaseCalls  int
	releaseHoldID string
	releaseCause  error
}

func (a *recordingAdmission) Reserve(_ context.Context, state *RequestState) error {
	a.reserveCalls++
	state.BalanceHoldID = "hold_new"
	return nil
}

func (a *recordingAdmission) Release(_ context.Context, state *RequestState, cause error) error {
	a.releaseCalls++
	a.releaseHoldID = state.BalanceHoldID
	a.releaseCause = cause
	return nil
}

type recordingAdmissionIDs struct {
	ids []string
}

func (a *recordingAdmissionIDs) Reserve(_ context.Context, state *RequestState) error {
	a.ids = append(a.ids, state.RequestID)
	state.BalanceHoldID = "hold_" + state.RequestID
	return nil
}

func (a *recordingAdmissionIDs) Release(context.Context, *RequestState, error) error {
	return nil
}

type asyncLimiter struct{}

func (asyncLimiter) Acquire(context.Context, *RequestState) (LimitRelease, error) {
	return noopLimitRelease{}, nil
}

type recordingLimiterIDs struct {
	ids []string
}

func (l *recordingLimiterIDs) Acquire(_ context.Context, state *RequestState) (LimitRelease, error) {
	l.ids = append(l.ids, state.RequestID)
	return noopLimitRelease{}, nil
}

type replayCreateTaskBridge struct{}

func (replayCreateTaskBridge) CheckIdempotency(context.Context, *RequestState) (*GatewayResponse, bool, error) {
	return nil, false, nil
}

func (replayCreateTaskBridge) CreateAndDispatch(context.Context, *RequestState) (*GatewayResponse, bool, error) {
	return &GatewayResponse{StatusCode: http.StatusAccepted, Header: http.Header{}, Body: []byte(`{"id":"task_existing"}`)}, true, nil
}

func (replayCreateTaskBridge) HandleTaskOperation(context.Context, *RequestState) (*GatewayResponse, error) {
	return nil, apperr.ConfigUnavailable("unused")
}

type asyncDispatcher struct{}

func (asyncDispatcher) Dispatch(context.Context, *RequestState) (*ProviderResult, error) {
	return nil, apperr.ConfigUnavailable("unused")
}

type successDispatcher struct{}

func (successDispatcher) Dispatch(context.Context, *RequestState) (*ProviderResult, error) {
	return &ProviderResult{
		Response: &GatewayResponse{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       []byte(`{"id":"chatcmpl_test"}`),
		},
		Candidate: ProviderCandidate{ProviderType: "openai_compatible", ChannelID: "channel_1"},
	}, nil
}

type recordingSettlementIDs struct {
	ids []string
}

func (s *recordingSettlementIDs) Settle(_ context.Context, state *RequestState) error {
	s.ids = append(s.ids, state.RequestID)
	return nil
}

func (s *recordingSettlementIDs) RecordFailed(context.Context, *RequestState, error) error {
	return nil
}
