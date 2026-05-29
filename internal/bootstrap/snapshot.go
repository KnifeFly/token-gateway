package bootstrap

import (
	"fmt"
	"time"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
)

func buildSeedSnapshot(cfg Config) (*dpsnapshot.IndexedSnapshot, error) {
	runtime := cpsnapshot.RuntimeSnapshot{
		Version:       seedVersion(cfg),
		SchemaVersion: "m1",
		CreatedAt:     time.Now().UTC(),
	}
	if cfg.Gateway.Seed.Enabled {
		seed := cfg.Gateway.Seed
		runtime.APIKeys = []cpsnapshot.APIKeyRuntime{{
			ID:            seed.APIKeyID,
			TenantID:      seed.TenantID,
			ProjectID:     seed.ProjectID,
			Name:          "local seed key",
			KeyHash:       auth.HashAPIKey(seed.APIKey),
			Enabled:       true,
			AllowedModels: []string{seed.Model},
		}}
		runtime.Models = []cpsnapshot.ModelRuntime{{
			PublicModel: seed.Model,
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "chat",
			Enabled:     true,
		}}
		runtime.Channels = []cpsnapshot.ChannelRuntime{{
			ID:           seed.ChannelID,
			ProviderType: seed.ProviderType,
			BaseURL:      seed.ProviderBaseURL,
			APIKey:       seed.ProviderAPIKey,
			Enabled:      true,
			Timeout:      seed.ChannelTimeout.Duration,
			Models: []cpsnapshot.ChannelModelRuntime{{
				PublicModel:   seed.Model,
				UpstreamModel: seed.UpstreamModel,
			}},
		}}
		runtime.RoutePolicies = []cpsnapshot.RoutePolicyRuntime{{
			ID:          fmt.Sprintf("route_%s", seed.Model),
			PublicModel: seed.Model,
			Strategy:    seed.RouteStrategy,
			Candidates: []cpsnapshot.RouteCandidateRuntime{{
				ChannelID: seed.ChannelID,
				Priority:  seed.RoutePriority,
				Weight:    seed.RouteWeight,
			}},
		}}
	}
	return dpsnapshot.Build(runtime)
}

func seedVersion(cfg Config) string {
	if cfg.Gateway.Seed.Enabled {
		return "seed-m1"
	}
	return "empty-m1"
}
