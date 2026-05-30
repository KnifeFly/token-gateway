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
			Aliases:     []string{"gpt-4o-mini-alias"},
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Enabled:     true,
			ProviderMappings: []cpsnapshot.ProviderModelMappingRuntime{{
				ProviderType:  "openai_compatible",
				ChannelID:     "channel_1",
				PublicModel:   "gpt-4o-mini",
				UpstreamModel: "gpt-4o-mini",
			}},
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
		LimitRules: []cpsnapshot.LimitRuleRuntime{{
			ID:           "limit_model",
			TenantID:     "tenant_1",
			ProjectID:    "project_1",
			APIKeyID:     "key_1",
			PublicModel:  "gpt-4o-mini",
			ProviderType: "openai_compatible",
			ChannelID:    "channel_1",
			RPM:          60,
			QPS:          1,
			Enabled:      true,
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
	if model, ok := indexed.LookupModel("gpt-4o-mini-alias"); !ok || model.PublicModel != "gpt-4o-mini" || len(model.ProviderMappings) != 1 {
		t.Fatalf("alias model = %#v ok = %v", model, ok)
	}
	limits := indexed.LookupLimits(engine.LimitScope{
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		APIKeyID:     "key_1",
		PublicModel:  "gpt-4o-mini",
		ProviderType: "openai_compatible",
		ChannelID:    "channel_1",
	})
	if len(limits) != 1 || limits[0].RPM != 60 {
		t.Fatalf("limits = %#v", limits)
	}
	bindings := indexed.LookupPluginBindings("pre_prompt")
	if len(bindings) != 1 || bindings[0].Name != "prompt_guard" {
		t.Fatalf("plugin bindings = %#v", bindings)
	}
}

func TestProviderAttachesOldSnapshot(t *testing.T) {
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
	provider := NewProvider(NewStore(indexed))
	state := &engine.RequestState{Internal: map[string]any{}}
	if err := provider.Attach(context.Background(), state); err != nil {
		t.Fatalf("Attach() error = %v", err)
	}
	if state.SnapshotRef.Version != "old" {
		t.Fatalf("snapshot version = %q", state.SnapshotRef.Version)
	}
}

func TestStoreDiagnosticsReportsCurrentSnapshotStaleness(t *testing.T) {
	createdAt := time.Unix(100, 0).UTC()
	indexed, err := Build(cpsnapshot.RuntimeSnapshot{
		Version:   "diag",
		CreatedAt: createdAt,
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "m",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Enabled:     true,
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	diagnostics, err := NewStore(indexed).Diagnostics(createdAt.Add(30 * time.Second))
	if err != nil {
		t.Fatalf("Diagnostics() error = %v", err)
	}
	if diagnostics.Current.Version != "diag" || diagnostics.StalenessSeconds != 30 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}
