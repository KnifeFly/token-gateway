package engine

import (
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
