package builtin

import (
	"context"
	"encoding/json"
	"testing"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
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
	if result.Mutations[1].Key != "policy.degrade_model" {
		t.Fatalf("mutations = %#v", result.Mutations)
	}
}

func TestCostGuardEmitsRouteOverrideDecision(t *testing.T) {
	result, err := CostGuard{}.Execute(context.Background(), plugin.Input{
		Phase: plugin.PhasePostRoute,
		State: &engine.RequestState{EstimatedChargeMicros: 500},
		Config: json.RawMessage(`{
			"max_estimated_micros":100,
			"route_override":{"channel_id":"channel_cheap","provider_type":"openai_compatible","upstream_model":"cheap-upstream"}
		}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Metadata["policy.action"] != string(engine.PolicyRouteOverride) {
		t.Fatalf("metadata = %#v", result.Metadata)
	}
}

func TestIPAllowlistAllowsAndDeniesWithoutIPAuditField(t *testing.T) {
	allow, err := IPAllowlist{}.Execute(context.Background(), plugin.Input{
		State:  &engine.RequestState{ClientIP: "203.0.113.10:443"},
		Config: json.RawMessage(`{"allow_cidrs":["203.0.113.0/24"]}`),
	})
	if err != nil {
		t.Fatalf("allow Execute() error = %v", err)
	}
	if allow.Action != plugin.ActionAllow || allow.AuditFields["client_ip"] != "" {
		t.Fatalf("allow result = %#v", allow)
	}
	deny, err := IPAllowlist{}.Execute(context.Background(), plugin.Input{
		State:  &engine.RequestState{ClientIP: "198.51.100.10"},
		Config: json.RawMessage(`{"allow_cidrs":["203.0.113.0/24"]}`),
	})
	if err != nil {
		t.Fatalf("deny Execute() error = %v", err)
	}
	if deny.Action != plugin.ActionDeny || deny.AuditFields["client_ip"] != "" {
		t.Fatalf("deny result = %#v", deny)
	}
}

func TestModelACLDeniesConfiguredModel(t *testing.T) {
	result, err := ModelACL{}.Execute(context.Background(), plugin.Input{
		State:  &engine.RequestState{RequestedModel: "blocked-model"},
		Config: json.RawMessage(`{"denied_models":["blocked-model"]}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Action != plugin.ActionDeny {
		t.Fatalf("result = %#v", result)
	}
}

func TestRouteOverrideEmitsConstrainedPolicyMetadata(t *testing.T) {
	state := &engine.RequestState{
		RequestedModel: "gpt-4o-mini",
		Snapshot:       mustBuiltinSnapshot(t),
	}
	result, err := RouteOverride{}.Execute(context.Background(), plugin.Input{
		Phase:  plugin.PhasePreRoute,
		State:  state,
		Config: json.RawMessage(`{"channel_id":"channel_cheap","reason":"tenant override"}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Metadata["policy.action"] != string(engine.PolicyRouteOverride) ||
		result.Metadata["policy.route_override.upstream_model"] != "gpt-4o-mini-cheap" {
		t.Fatalf("metadata = %#v", result.Metadata)
	}
}

func TestCallbackPluginControlsTaskCallbackURL(t *testing.T) {
	state := &engine.RequestState{
		Parsed: engine.ParsedRequest{Media: &engine.UnifiedMediaRequest{CallbackURL: "https://client.example/cb"}},
	}
	result, err := Callback{}.Execute(context.Background(), plugin.Input{
		Phase:  plugin.PhasePrePrompt,
		State:  state,
		Config: json.RawMessage(`{"mode":"override","callback_url":"https://hooks.example/task","allowed_hosts":["hooks.example"]}`),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Action != plugin.ActionAllow || state.Parsed.Media.CallbackURL != "https://hooks.example/task" {
		t.Fatalf("result = %#v callback = %q", result, state.Parsed.Media.CallbackURL)
	}
}

func mustBuiltinSnapshot(t *testing.T) *dpsnapshot.IndexedSnapshot {
	t.Helper()
	indexed, err := dpsnapshot.Build(cpsnapshot.RuntimeSnapshot{
		Version:       "test",
		SchemaVersion: "p2",
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "gpt-4o-mini",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "chat",
			Enabled:     true,
		}},
		Channels: []cpsnapshot.ChannelRuntime{{
			ID:           "channel_cheap",
			ProviderType: "openai_compatible",
			BaseURL:      "mock://openai",
			Enabled:      true,
			Models:       []cpsnapshot.ChannelModelRuntime{{PublicModel: "gpt-4o-mini", UpstreamModel: "gpt-4o-mini-cheap"}},
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return indexed
}
