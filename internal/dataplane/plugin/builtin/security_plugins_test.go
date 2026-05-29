package builtin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
)

func TestRequestSizeDeniesLargeBody(t *testing.T) {
	result, err := RequestSize{}.Execute(context.Background(), plugin.Input{
		State:  &engine.RequestState{Incoming: engine.IncomingRequest{ContentLength: 1024}},
		Config: json.RawMessage(`{"max_body_bytes":100}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Action != plugin.ActionDeny {
		t.Fatalf("action = %q, want deny", result.Action)
	}
}

func TestPIIRedactionRedactsPromptAndResponse(t *testing.T) {
	state := &engine.RequestState{
		Parsed: engine.ParsedRequest{RawBody: []byte(`{"input":"alice@example.com"}`)},
		ProviderResult: &engine.ProviderResult{Response: &engine.GatewayResponse{
			Body: []byte(`{"output":"call +1 415-555-0100"}`),
		}},
	}
	if _, err := (PIIRedaction{}).Execute(context.Background(), plugin.Input{
		Phase:  plugin.PhasePrePrompt,
		State:  state,
		Config: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("Execute(prompt) error = %v", err)
	}
	if string(state.Parsed.RawBody) == `{"input":"alice@example.com"}` {
		t.Fatal("prompt was not redacted")
	}
	if _, err := (PIIRedaction{}).Execute(context.Background(), plugin.Input{
		Phase:  plugin.PhasePostProvider,
		State:  state,
		Config: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("Execute(response) error = %v", err)
	}
	if string(state.ProviderResult.Response.Body) == `{"output":"call +1 415-555-0100"}` {
		t.Fatal("response was not redacted")
	}
}

func TestPromptGuardDeniesConfiguredTerm(t *testing.T) {
	result, err := PromptGuard{}.Execute(context.Background(), plugin.Input{
		Phase:  plugin.PhasePrePrompt,
		State:  &engine.RequestState{Parsed: engine.ParsedRequest{RawBody: []byte(`{"input":"deny this"}`)}},
		Config: json.RawMessage(`{"deny_terms":["deny this"]}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Action != plugin.ActionDeny {
		t.Fatalf("action = %q, want deny", result.Action)
	}
}

func TestCostGuardDegradesWhenSuggestionConfigured(t *testing.T) {
	result, err := CostGuard{}.Execute(context.Background(), plugin.Input{
		Phase: plugin.PhasePostRoute,
		State: &engine.RequestState{
			EstimatedChargeMicros: 500,
		},
		Config: json.RawMessage(`{"max_estimated_micros":100,"suggested_model":"cheap-model"}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Action != plugin.ActionDegrade || result.SuggestedModel != "cheap-model" {
		t.Fatalf("result = %#v", result)
	}
}
