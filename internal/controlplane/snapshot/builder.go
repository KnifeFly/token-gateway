package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ConfigRepository loads normalized admin config for snapshot builds.
type ConfigRepository interface {
	LoadSnapshotConfig(ctx context.Context) (*configadmin.SnapshotConfig, error)
	SaveSnapshot(ctx context.Context, record configadmin.SnapshotRecord) (*configadmin.SnapshotRecord, error)
	ActiveSnapshot(ctx context.Context) (*configadmin.SnapshotRecord, bool, error)
	PreviousSnapshot(ctx context.Context) (*configadmin.SnapshotRecord, bool, error)
	ActivateSnapshot(ctx context.Context, version string) (*configadmin.SnapshotRecord, error)
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
		SchemaVersion: "p11",
		CreatedAt:     time.Now().UTC(),
	}
	providerMappings := providerMappingsByModel(cfg.Channels)
	for _, key := range cfg.APIKeys {
		runtime.APIKeys = append(runtime.APIKeys, APIKeyRuntime{
			ID:            key.ID,
			TenantID:      key.TenantID,
			ProjectID:     key.ProjectID,
			Name:          key.Name,
			KeyHash:       key.KeyHash,
			Enabled:       key.Enabled,
			AllowedModels: append([]string(nil), key.AllowedModels...),
			IPAllowlist:   append([]string(nil), key.IPAllowlist...),
			ExpiresAt:     cloneTimePtr(key.ExpiresAt),
			LastUsedAt:    cloneTimePtr(key.LastUsedAt),
		})
	}
	for _, model := range cfg.Models {
		schema := append(json.RawMessage(nil), model.Schema...)
		if len(schema) == 0 {
			schema = json.RawMessage(`{}`)
		}
		metadata := append(json.RawMessage(nil), model.Metadata...)
		if len(metadata) == 0 {
			metadata = json.RawMessage(`{}`)
		}
		runtime.Models = append(runtime.Models, ModelRuntime{
			PublicModel:      model.PublicModel,
			Aliases:          append([]string(nil), model.Aliases...),
			DisplayName:      model.DisplayName,
			Description:      model.Description,
			Protocol:         model.Protocol,
			Capability:       model.Capability,
			Category:         model.Category,
			Tags:             append([]string(nil), model.Tags...),
			ProviderFamily:   model.ProviderFamily,
			Modalities:       append([]string(nil), model.Modalities...),
			Capabilities:     append([]string(nil), model.Capabilities...),
			ContextWindow:    model.ContextWindow,
			MaxOutputTokens:  model.MaxOutputTokens,
			Status:           model.Status,
			Deprecated:       model.Deprecated,
			SortOrder:        model.SortOrder,
			Metadata:         metadata,
			Schema:           schema,
			ProviderMappings: append([]ProviderModelMappingRuntime(nil), providerMappings[model.PublicModel]...),
			Enabled:          model.Enabled,
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
			metadata := append(json.RawMessage(nil), model.Metadata...)
			if len(metadata) == 0 {
				metadata = json.RawMessage(`{}`)
			}
			next.Models = append(next.Models, ChannelModelRuntime{
				PublicModel:         model.PublicModel,
				UpstreamModel:       model.UpstreamModel,
				Capabilities:        append([]string(nil), model.Capabilities...),
				SupportedParameters: append([]string(nil), model.SupportedParameters...),
				HealthStatus:        model.HealthStatus,
				TestStatus:          model.TestStatus,
				CostConfigStatus:    model.CostConfigStatus,
				Metadata:            metadata,
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
		metadata := append(json.RawMessage(nil), price.Metadata...)
		if len(metadata) == 0 {
			metadata = json.RawMessage(`{}`)
		}
		runtime.PriceRules = append(runtime.PriceRules, PriceRuleRuntime{
			PublicModel:           price.PublicModel,
			Category:              price.Category,
			Currency:              price.Currency,
			Components:            append([]pricing.Component(nil), price.Components...),
			InputMicrosPerToken:   price.InputMicrosPerToken,
			OutputMicrosPerToken:  price.OutputMicrosPerToken,
			EstimatedOutputTokens: price.EstimatedOutputTokens,
			Metadata:              metadata,
			Enabled:               price.Enabled,
		})
	}
	for _, limit := range cfg.Limits {
		runtime.LimitRules = append(runtime.LimitRules, LimitRuleRuntime{
			ID:                  limit.ID,
			TenantID:            limit.TenantID,
			ProjectID:           limit.ProjectID,
			APIKeyID:            limit.APIKeyID,
			PublicModel:         limit.PublicModel,
			ProviderType:        limit.ProviderType,
			ChannelID:           limit.ChannelID,
			RPM:                 limit.RPM,
			QPS:                 limit.QPS,
			TPM:                 limit.TPM,
			Concurrency:         limit.Concurrency,
			DailyBudgetMicros:   limit.DailyBudgetMicros,
			CostPerMinuteMicros: limit.CostPerMinuteMicros,
			Enabled:             limit.Enabled,
		})
	}
	for _, binding := range cfg.Plugins {
		config := append(json.RawMessage(nil), binding.Config...)
		if len(config) == 0 {
			config = json.RawMessage(`{}`)
		}
		runtime.PluginBindings = append(runtime.PluginBindings, PluginBindingRuntime{
			ID:            binding.ID,
			Name:          binding.Name,
			Phase:         binding.Phase,
			TenantID:      binding.TenantID,
			ProjectID:     binding.ProjectID,
			Model:         binding.Model,
			Priority:      binding.Priority,
			Enabled:       binding.Enabled,
			FailurePolicy: binding.FailurePolicy,
			Config:        config,
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
	aliases := map[string]string{}
	for _, model := range runtime.Models {
		if model.PublicModel == "" || model.Protocol == "" || model.Capability == "" {
			return apperr.InvalidArgument("model public_model, protocol, and capability are required")
		}
		if models[model.PublicModel].PublicModel != "" {
			return apperr.InvalidArgument("duplicate model in snapshot")
		}
		if len(model.Schema) > 0 && !json.Valid(model.Schema) {
			return apperr.InvalidArgument("model schema must be valid json")
		}
		if _, err := pricing.InferCategory(model.Category, model.Capability, model.PublicModel); err != nil {
			return apperr.InvalidArgument(err.Error())
		}
		if len(model.Metadata) > 0 && !json.Valid(model.Metadata) {
			return apperr.InvalidArgument("model metadata must be valid json")
		}
		for _, alias := range model.Aliases {
			if alias == "" || alias == model.PublicModel {
				continue
			}
			if existing := aliases[alias]; existing != "" && existing != model.PublicModel {
				return apperr.InvalidArgument("duplicate model alias in snapshot")
			}
			if models[alias].PublicModel != "" {
				return apperr.InvalidArgument("model alias conflicts with public model")
			}
			aliases[alias] = model.PublicModel
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
			if len(mapping.Metadata) > 0 && !json.Valid(mapping.Metadata) {
				return apperr.InvalidArgument("channel model metadata must be valid json")
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
		if !price.Enabled {
			continue
		}
		category, err := pricing.InferCategory(price.Category, models[price.PublicModel].Capability, price.PublicModel)
		if err != nil {
			return apperr.InvalidArgument(err.Error())
		}
		if _, err := pricing.NormalizePriceBook(pricing.PriceBook{
			Category:   category,
			Currency:   price.Currency,
			Components: price.Components,
		}, pricing.TokenPrice{
			Currency:             price.Currency,
			InputMicrosPerToken:  price.InputMicrosPerToken,
			OutputMicrosPerToken: price.OutputMicrosPerToken,
		}); err != nil {
			return apperr.InvalidArgument(err.Error())
		}
		if len(price.Metadata) > 0 && !json.Valid(price.Metadata) {
			return apperr.InvalidArgument("price metadata must be valid json")
		}
	}
	for _, limit := range runtime.LimitRules {
		if !limit.Enabled {
			continue
		}
		if limit.TenantID == "" && limit.ProjectID == "" && limit.APIKeyID == "" && limit.PublicModel == "" && limit.ProviderType == "" && limit.ChannelID == "" {
			return apperr.InvalidArgument("limit references empty scope")
		}
		if limit.PublicModel != "" && models[limit.PublicModel].PublicModel == "" {
			return apperr.InvalidArgument("limit references unknown model")
		}
	}
	for _, binding := range runtime.PluginBindings {
		if binding.Name == "" || binding.Phase == "" {
			return apperr.InvalidArgument("plugin binding name and phase are required")
		}
		if !validPhase(binding.Phase) {
			return apperr.InvalidArgument("plugin binding phase is not supported")
		}
		if binding.FailurePolicy != "" && binding.FailurePolicy != "fail_closed" && binding.FailurePolicy != "fail_open" {
			return apperr.InvalidArgument("plugin binding failure_policy is not supported")
		}
		if binding.Model != "" && models[binding.Model].PublicModel == "" {
			return apperr.InvalidArgument("plugin binding references unknown model")
		}
		if len(binding.Config) > 0 && !json.Valid(binding.Config) {
			return apperr.InvalidArgument("plugin binding config must be valid json")
		}
	}
	return nil
}

func providerMappingsByModel(channels []configadmin.ChannelConfig) map[string][]ProviderModelMappingRuntime {
	out := map[string][]ProviderModelMappingRuntime{}
	for _, channel := range channels {
		for _, model := range channel.Models {
			metadata := append(json.RawMessage(nil), model.Metadata...)
			if len(metadata) == 0 {
				metadata = json.RawMessage(`{}`)
			}
			out[model.PublicModel] = append(out[model.PublicModel], ProviderModelMappingRuntime{
				ProviderType:        channel.ProviderType,
				ChannelID:           channel.ID,
				PublicModel:         model.PublicModel,
				UpstreamModel:       model.UpstreamModel,
				Capabilities:        append([]string(nil), model.Capabilities...),
				SupportedParameters: append([]string(nil), model.SupportedParameters...),
				HealthStatus:        model.HealthStatus,
				TestStatus:          model.TestStatus,
				CostConfigStatus:    model.CostConfigStatus,
				Metadata:            metadata,
			})
		}
	}
	return out
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func validPhase(phase string) bool {
	switch phase {
	case "pre_request", "post_auth", "pre_prompt", "pre_route", "post_route", "pre_provider", "post_provider", "pre_settlement", "audit":
		return true
	default:
		return false
	}
}
