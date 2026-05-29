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
			AllowedModels: []string{"*"},
		}}
		runtime.Models = []cpsnapshot.ModelRuntime{
			{PublicModel: seed.Model, Protocol: string(engine.ProtocolNativeOpenAI), Capability: "chat", Enabled: true},
			{PublicModel: "text-embedding-3-small", Protocol: string(engine.ProtocolNativeOpenAI), Capability: "embeddings", Enabled: true},
			{PublicModel: "claude-3-5-sonnet-latest", Protocol: string(engine.ProtocolNativeClaude), Capability: "messages", Enabled: true},
			{PublicModel: "gemini-2.5-flash", Protocol: string(engine.ProtocolNativeGemini), Capability: "generate_content", Enabled: true},
			{PublicModel: "gemini-3.1-flash-image-preview", Protocol: string(engine.ProtocolUnified), Capability: "image_generation", Enabled: true},
			{PublicModel: "gpt-image-1", Protocol: string(engine.ProtocolUnified), Capability: "image_edit", Enabled: true},
			{PublicModel: "seedance-2.0-text-to-video", Protocol: string(engine.ProtocolUnified), Capability: "video_generation", Enabled: true},
			{PublicModel: "tts-1", Protocol: string(engine.ProtocolUnified), Capability: "audio_speech", Enabled: true},
			{PublicModel: "whisper-1", Protocol: string(engine.ProtocolUnified), Capability: "audio_transcription", Enabled: true},
			{PublicModel: "suno-music", Protocol: string(engine.ProtocolUnified), Capability: "music_generation", Enabled: true},
		}
		runtime.Channels = []cpsnapshot.ChannelRuntime{{
			ID:           seed.ChannelID,
			ProviderType: seed.ProviderType,
			BaseURL:      seed.ProviderBaseURL,
			APIKey:       seed.ProviderAPIKey,
			Enabled:      true,
			Timeout:      seed.ChannelTimeout.Duration,
			Models: []cpsnapshot.ChannelModelRuntime{
				{PublicModel: seed.Model, UpstreamModel: seed.UpstreamModel},
				{PublicModel: "text-embedding-3-small", UpstreamModel: "text-embedding-3-small"},
			},
		}, {
			ID:           "channel_mock_claude",
			ProviderType: "claude",
			BaseURL:      "mock://claude",
			Enabled:      true,
			Timeout:      seed.ChannelTimeout.Duration,
			Models: []cpsnapshot.ChannelModelRuntime{
				{PublicModel: "claude-3-5-sonnet-latest", UpstreamModel: "claude-3-5-sonnet-latest"},
			},
		}, {
			ID:           "channel_mock_media",
			ProviderType: "mock_media",
			BaseURL:      "mock://media",
			Enabled:      true,
			Timeout:      seed.ChannelTimeout.Duration,
			Models: []cpsnapshot.ChannelModelRuntime{
				{PublicModel: "gemini-3.1-flash-image-preview", UpstreamModel: "gemini-3.1-flash-image-preview"},
				{PublicModel: "gpt-image-1", UpstreamModel: "gpt-image-1"},
				{PublicModel: "seedance-2.0-text-to-video", UpstreamModel: "seedance-2.0-text-to-video"},
				{PublicModel: "tts-1", UpstreamModel: "tts-1"},
				{PublicModel: "whisper-1", UpstreamModel: "whisper-1"},
				{PublicModel: "suno-music", UpstreamModel: "suno-music"},
			},
		}, {
			ID:           "channel_mock_gemini",
			ProviderType: "gemini",
			BaseURL:      "mock://gemini",
			Enabled:      true,
			Timeout:      seed.ChannelTimeout.Duration,
			Models: []cpsnapshot.ChannelModelRuntime{
				{PublicModel: "gemini-2.5-flash", UpstreamModel: "gemini-2.5-flash"},
			},
		}}
		runtime.RoutePolicies = []cpsnapshot.RoutePolicyRuntime{
			seedRoute(seed.Model, seed.ChannelID, seed),
			seedRoute("text-embedding-3-small", seed.ChannelID, seed),
			seedRoute("claude-3-5-sonnet-latest", "channel_mock_claude", seed),
			seedRoute("gemini-2.5-flash", "channel_mock_gemini", seed),
			seedRoute("gemini-3.1-flash-image-preview", "channel_mock_media", seed),
			seedRoute("gpt-image-1", "channel_mock_media", seed),
			seedRoute("seedance-2.0-text-to-video", "channel_mock_media", seed),
			seedRoute("tts-1", "channel_mock_media", seed),
			seedRoute("whisper-1", "channel_mock_media", seed),
			seedRoute("suno-music", "channel_mock_media", seed),
		}
	}
	return dpsnapshot.Build(runtime)
}

func seedRoute(model string, channelID string, seed SeedSnapshotConfig) cpsnapshot.RoutePolicyRuntime {
	return cpsnapshot.RoutePolicyRuntime{
		ID:          fmt.Sprintf("route_%s", model),
		PublicModel: model,
		Strategy:    seed.RouteStrategy,
		Candidates: []cpsnapshot.RouteCandidateRuntime{{
			ChannelID: channelID,
			Priority:  seed.RoutePriority,
			Weight:    seed.RouteWeight,
		}},
	}
}

func seedVersion(cfg Config) string {
	if cfg.Gateway.Seed.Enabled {
		return "seed-m1"
	}
	return "empty-m1"
}
