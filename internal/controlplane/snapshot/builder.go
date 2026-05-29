package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ConfigRepository loads normalized admin config for snapshot builds.
type ConfigRepository interface {
	LoadSnapshotConfig(ctx context.Context) (*admin.SnapshotConfig, error)
	SaveSnapshot(ctx context.Context, record admin.SnapshotRecord) (*admin.SnapshotRecord, error)
	ActiveSnapshot(ctx context.Context) (*admin.SnapshotRecord, bool, error)
	PreviousSnapshot(ctx context.Context) (*admin.SnapshotRecord, bool, error)
	ActivateSnapshot(ctx context.Context, version string) (*admin.SnapshotRecord, error)
}

// Builder builds runtime snapshots from admin config.
type Builder struct {
	repo ConfigRepository
}

// NewBuilder returns a runtime snapshot builder.
func NewBuilder(repo ConfigRepository) *Builder {
	return &Builder{repo: repo}
}

// Build loads config, validates references, and returns a runtime snapshot.
func (b *Builder) Build(ctx context.Context) (*RuntimeSnapshot, error) {
	cfg, err := b.repo.LoadSnapshotConfig(ctx)
	if err != nil {
		return nil, err
	}
	runtime := &RuntimeSnapshot{
		Version:       fmt.Sprintf("snap-%d", time.Now().UTC().UnixNano()),
		SchemaVersion: "m5",
		CreatedAt:     time.Now().UTC(),
	}
	for _, key := range cfg.APIKeys {
		runtime.APIKeys = append(runtime.APIKeys, APIKeyRuntime{
			ID:            key.ID,
			TenantID:      key.TenantID,
			ProjectID:     key.ProjectID,
			Name:          key.Name,
			KeyHash:       key.KeyHash,
			Enabled:       key.Enabled,
			AllowedModels: append([]string(nil), key.AllowedModels...),
		})
	}
	for _, model := range cfg.Models {
		runtime.Models = append(runtime.Models, ModelRuntime{
			PublicModel: model.PublicModel,
			Protocol:    model.Protocol,
			Capability:  model.Capability,
			Enabled:     model.Enabled,
		})
	}
	for _, channel := range cfg.Channels {
		next := ChannelRuntime{
			ID:              channel.ID,
			ProviderType:    channel.ProviderType,
			BaseURL:         channel.BaseURL,
			CredentialRef:   channel.CredentialRef,
			EncryptedAPIKey: channel.EncryptedAPIKey,
			Enabled:         channel.Enabled,
			Timeout:         channel.Timeout,
		}
		if next.Timeout == 0 && channel.TimeoutMillis > 0 {
			next.Timeout = time.Duration(channel.TimeoutMillis) * time.Millisecond
		}
		for _, model := range channel.Models {
			next.Models = append(next.Models, ChannelModelRuntime{
				PublicModel:   model.PublicModel,
				UpstreamModel: model.UpstreamModel,
			})
		}
		runtime.Channels = append(runtime.Channels, next)
	}
	for _, route := range cfg.Routes {
		if !route.Enabled {
			continue
		}
		next := RoutePolicyRuntime{
			ID:          route.ID,
			PublicModel: route.PublicModel,
			Strategy:    route.Strategy,
		}
		for _, candidate := range route.Candidates {
			next.Candidates = append(next.Candidates, RouteCandidateRuntime{
				ChannelID: candidate.ChannelID,
				Priority:  candidate.Priority,
				Weight:    candidate.Weight,
			})
		}
		runtime.RoutePolicies = append(runtime.RoutePolicies, next)
	}
	for _, price := range cfg.Prices {
		runtime.PriceRules = append(runtime.PriceRules, PriceRuleRuntime{
			PublicModel:           price.PublicModel,
			Currency:              price.Currency,
			InputMicrosPerToken:   price.InputMicrosPerToken,
			OutputMicrosPerToken:  price.OutputMicrosPerToken,
			EstimatedOutputTokens: price.EstimatedOutputTokens,
			Enabled:               price.Enabled,
		})
	}
	for _, limit := range cfg.Limits {
		runtime.LimitRules = append(runtime.LimitRules, LimitRuleRuntime{
			PublicModel: limit.PublicModel,
			QPS:         limit.QPS,
			TPM:         limit.TPM,
			Concurrency: limit.Concurrency,
			Enabled:     limit.Enabled,
		})
	}
	for _, key := range cfg.RevokedKeys {
		if key.KeyHash != "" && key.RevokedAt != nil {
			runtime.RevokedKeys = append(runtime.RevokedKeys, RevokedKeyRuntime{KeyHash: key.KeyHash, RevokedAt: *key.RevokedAt})
		}
	}
	if err := Validate(*runtime); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(runtime)
	sum := sha256.Sum256(payload)
	runtime.Checksum = "sha256:" + hex.EncodeToString(sum[:])
	return runtime, nil
}

// Validate rejects bad runtime snapshots before publication.
func Validate(runtime RuntimeSnapshot) error {
	models := map[string]ModelRuntime{}
	for _, model := range runtime.Models {
		if model.PublicModel == "" || model.Protocol == "" || model.Capability == "" {
			return apperr.InvalidArgument("model public_model, protocol, and capability are required")
		}
		if models[model.PublicModel].PublicModel != "" {
			return apperr.InvalidArgument("duplicate model in snapshot")
		}
		models[model.PublicModel] = model
	}
	channels := map[string]ChannelRuntime{}
	for _, channel := range runtime.Channels {
		if channel.ID == "" || channel.ProviderType == "" || channel.BaseURL == "" {
			return apperr.InvalidArgument("channel id, provider_type, and base_url are required")
		}
		if channel.APIKey != "" {
			return apperr.InvalidArgument("snapshot must not contain plaintext provider credentials")
		}
		channels[channel.ID] = channel
		for _, mapping := range channel.Models {
			if models[mapping.PublicModel].PublicModel == "" {
				return apperr.InvalidArgument("channel model references unknown model")
			}
			if mapping.UpstreamModel == "" {
				return apperr.InvalidArgument("channel upstream_model is required")
			}
		}
	}
	for _, route := range runtime.RoutePolicies {
		if models[route.PublicModel].PublicModel == "" {
			return apperr.InvalidArgument("route references unknown model")
		}
		if len(route.Candidates) == 0 {
			return apperr.InvalidArgument("route candidates are required")
		}
		for _, candidate := range route.Candidates {
			channel := channels[candidate.ChannelID]
			if channel.ID == "" {
				return apperr.InvalidArgument("route candidate references unknown channel")
			}
		}
	}
	for _, price := range runtime.PriceRules {
		if price.Enabled && models[price.PublicModel].PublicModel == "" {
			return apperr.InvalidArgument("price references unknown model")
		}
	}
	for _, limit := range runtime.LimitRules {
		if limit.Enabled && models[limit.PublicModel].PublicModel == "" {
			return apperr.InvalidArgument("limit references unknown model")
		}
	}
	return nil
}
