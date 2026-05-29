package snapshot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

func TestBuildIndexesRuntimeSnapshot(t *testing.T) {
	indexed, err := Build(cpsnapshot.RuntimeSnapshot{
		Version:   "v1",
		CreatedAt: time.Unix(100, 0),
		APIKeys: []cpsnapshot.APIKeyRuntime{{
			ID:       "key_1",
			TenantID: "tenant_1",
			KeyHash:  "sha256:test",
			Enabled:  true,
		}},
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "gpt-4o-mini",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Enabled:     true,
		}},
		Channels: []cpsnapshot.ChannelRuntime{{
			ID:           "channel_1",
			ProviderType: "openai_compatible",
			Enabled:      true,
			Models: []cpsnapshot.ChannelModelRuntime{{
				PublicModel:   "gpt-4o-mini",
				UpstreamModel: "gpt-4o-mini",
			}},
		}},
		RoutePolicies: []cpsnapshot.RoutePolicyRuntime{{
			ID:          "route_1",
			PublicModel: "gpt-4o-mini",
			Strategy:    "priority",
			Candidates: []cpsnapshot.RouteCandidateRuntime{{
				ChannelID: "channel_1",
				Priority:  1,
				Weight:    100,
			}},
		}},
		PluginBindings: []cpsnapshot.PluginBindingRuntime{{
			ID:            "guard",
			Name:          "prompt_guard",
			Phase:         "pre_prompt",
			Model:         "gpt-4o-mini",
			Priority:      1,
			Enabled:       true,
			FailurePolicy: "fail_closed",
			Config:        json.RawMessage(`{"deny_terms":["blocked"]}`),
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if indexed.Ref().Version != "v1" {
		t.Fatalf("version = %q", indexed.Ref().Version)
	}
	if _, ok := indexed.LookupAPIKeyHash("sha256:test"); !ok {
		t.Fatal("missing api key index")
	}
	if channel, ok := indexed.LookupChannel("channel_1"); !ok || channel.Models["gpt-4o-mini"] != "gpt-4o-mini" {
		t.Fatalf("channel = %#v, ok = %v", channel, ok)
	}
	bindings := indexed.LookupPluginBindings("pre_prompt")
	if len(bindings) != 1 || bindings[0].Name != "prompt_guard" {
		t.Fatalf("plugin bindings = %#v", bindings)
	}
}

func TestProviderRejectsHardStaleSnapshot(t *testing.T) {
	indexed, err := Build(cpsnapshot.RuntimeSnapshot{
		Version:   "old",
		CreatedAt: time.Now().UTC().Add(-time.Hour),
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "m",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Enabled:     true,
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	provider := NewProvider(NewStore(indexed), WithStalePolicy(StalePolicy{
		SoftTTL: time.Minute,
		HardTTL: 2 * time.Minute,
	}))
	err = provider.Attach(context.Background(), &engine.RequestState{Internal: map[string]any{}})
	if err == nil {
		t.Fatal("expected stale error")
	}
}

func TestProviderMarksSoftStaleSnapshot(t *testing.T) {
	indexed, err := Build(cpsnapshot.RuntimeSnapshot{
		Version:   "soft",
		CreatedAt: time.Now().UTC().Add(-time.Minute),
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "m",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Enabled:     true,
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	provider := NewProvider(NewStore(indexed), WithStalePolicy(StalePolicy{
		SoftTTL: time.Second,
		HardTTL: time.Hour,
	}))
	state := &engine.RequestState{Internal: map[string]any{}}
	if err := provider.Attach(context.Background(), state); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if state.Internal["snapshot_stale"] != "soft" {
		t.Fatalf("stale marker = %#v", state.Internal["snapshot_stale"])
	}
}
