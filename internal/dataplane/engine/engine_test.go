package engine

import (
	"context"
	"encoding/json"
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

type fakePolicy struct {
	decision PolicyDecision
	err      error
}

func (p fakePolicy) Evaluate(context.Context, *RequestState) (PolicyDecision, error) {
	return p.decision, p.err
}

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
