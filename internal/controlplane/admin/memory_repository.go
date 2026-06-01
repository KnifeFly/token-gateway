package admin

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
)

// MemoryRepository is a local control-plane repository for tests and dev.
type MemoryRepository struct {
	mu        sync.RWMutex
	tenants   map[string]Tenant
	projects  map[string]Project
	apiKeys   map[string]APIKey
	models    map[string]ModelConfig
	channels  map[string]ChannelConfig
	routes    map[string]RoutePolicyConfig
	prices    map[string]PriceRuleConfig
	limits    map[string]LimitRuleConfig
	plugins   map[string]PluginBindingConfig
	market    map[string]ModelMarketplaceConfig
	snapshots map[string]SnapshotRecord
	active    string
	previous  string
}

// NewMemoryRepository returns an empty control-plane repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		tenants:   map[string]Tenant{},
		projects:  map[string]Project{},
		apiKeys:   map[string]APIKey{},
		models:    map[string]ModelConfig{},
		channels:  map[string]ChannelConfig{},
		routes:    map[string]RoutePolicyConfig{},
		prices:    map[string]PriceRuleConfig{},
		limits:    map[string]LimitRuleConfig{},
		plugins:   map[string]PluginBindingConfig{},
		market:    map[string]ModelMarketplaceConfig{},
		snapshots: map[string]SnapshotRecord{},
	}
}

// UpsertTenant creates or updates a tenant in memory.
func (r *MemoryRepository) UpsertTenant(_ context.Context, tenant Tenant) (*Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if tenant.ID == "" {
		tenant.ID = newID("tenant")
	}
	if existing, ok := r.tenants[tenant.ID]; ok {
		tenant.CreatedAt = existing.CreatedAt
	} else {
		tenant.CreatedAt = now
	}
	tenant.UpdatedAt = now
	r.tenants[tenant.ID] = tenant
	return clone(tenant), nil
}

// ListTenants returns all tenants ordered by ID.
func (r *MemoryRepository) ListTenants(_ context.Context) ([]Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenants := make([]Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		tenants = append(tenants, tenant)
	}
	sort.Slice(tenants, func(i, j int) bool { return tenants[i].ID < tenants[j].ID })
	return tenants, nil
}

// UpsertProject creates or updates a project in memory.
func (r *MemoryRepository) UpsertProject(_ context.Context, project Project) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if project.ID == "" {
		project.ID = newID("project")
	}
	if existing, ok := r.projects[project.ID]; ok {
		project.CreatedAt = existing.CreatedAt
	} else {
		project.CreatedAt = now
	}
	project.UpdatedAt = now
	r.projects[project.ID] = project
	return clone(project), nil
}

// ListProjects returns projects ordered by ID and optionally filtered by tenant.
func (r *MemoryRepository) ListProjects(_ context.Context, tenantID string) ([]Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	projects := make([]Project, 0, len(r.projects))
	for _, project := range r.projects {
		if tenantID != "" && project.TenantID != tenantID {
			continue
		}
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	return projects, nil
}

// CreateAPIKey stores a hashed API key record.
func (r *MemoryRepository) CreateAPIKey(_ context.Context, key APIKey) (*APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if key.ID == "" {
		key.ID = newID("key")
	}
	key.CreatedAt = now
	key.UpdatedAt = now
	r.apiKeys[key.ID] = key
	return clone(key), nil
}

// ListAPIKeys returns safe API key metadata for a tenant or project scope.
func (r *MemoryRepository) ListAPIKeys(_ context.Context, tenantID, projectID string) ([]APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]APIKey, 0, len(r.apiKeys))
	for _, key := range r.apiKeys {
		if tenantID != "" && key.TenantID != tenantID {
			continue
		}
		if projectID != "" && key.ProjectID != projectID {
			continue
		}
		key.PlaintextKey = ""
		keys = append(keys, key)
	}
	return keys, nil
}

// DisableAPIKey disables a stored API key and records revocation time.
func (r *MemoryRepository) DisableAPIKey(_ context.Context, keyID string, revokedAt *time.Time) (*APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.apiKeys[keyID]
	key.Enabled = false
	key.RevokedAt = revokedAt
	key.UpdatedAt = time.Now().UTC()
	r.apiKeys[keyID] = key
	return clone(key), nil
}

// UpsertModel creates or updates public model configuration.
func (r *MemoryRepository) UpsertModel(_ context.Context, model ModelConfig) (*ModelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(model.Schema) == 0 {
		model.Schema = json.RawMessage(`{}`)
	}
	r.models[model.PublicModel] = model
	return clone(model), nil
}

// UpsertChannel creates or updates provider channel configuration.
func (r *MemoryRepository) UpsertChannel(_ context.Context, channel ChannelConfig) (*ChannelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[channel.ID] = channel
	return clone(channel), nil
}

// UpsertRoute creates or updates a route policy.
func (r *MemoryRepository) UpsertRoute(_ context.Context, route RoutePolicyConfig) (*RoutePolicyConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[route.ID] = route
	return clone(route), nil
}

// UpsertPrice creates or updates a model price rule.
func (r *MemoryRepository) UpsertPrice(_ context.Context, price PriceRuleConfig) (*PriceRuleConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prices[price.PublicModel] = price
	return clone(price), nil
}

// UpsertLimit creates or updates a scoped limit rule.
func (r *MemoryRepository) UpsertLimit(_ context.Context, limit LimitRuleConfig) (*LimitRuleConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit.ID == "" {
		limit.ID = limitRuleID(limit)
	}
	r.limits[limit.ID] = limit
	return clone(limit), nil
}

// UpsertPluginBinding creates or updates a plugin binding.
func (r *MemoryRepository) UpsertPluginBinding(_ context.Context, binding PluginBindingConfig) (*PluginBindingConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if binding.ID == "" {
		binding.ID = newID("plugin")
	}
	if existing, ok := r.plugins[binding.ID]; ok {
		binding.CreatedAt = existing.CreatedAt
	} else {
		binding.CreatedAt = now
	}
	binding.UpdatedAt = now
	r.plugins[binding.ID] = binding
	return clone(binding), nil
}

// UpsertModelMarketplace creates or updates a tenant-visible catalog row.
func (r *MemoryRepository) UpsertModelMarketplace(_ context.Context, config ModelMarketplaceConfig) (*ModelMarketplaceConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if config.ID == "" {
		config.ID = marketplaceID(config)
	}
	if existing, ok := r.market[config.ID]; ok {
		config.CreatedAt = existing.CreatedAt
	} else {
		config.CreatedAt = now
	}
	config.UpdatedAt = now
	r.market[config.ID] = config
	return clone(config), nil
}

// ListVisibleModels returns enabled catalog rows visible to tenantID and projectID.
func (r *MemoryRepository) ListVisibleModels(_ context.Context, tenantID, projectID string) ([]VisibleModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []VisibleModel
	for _, config := range r.market {
		if !config.Enabled || !marketplaceScopeMatches(config, tenantID, projectID) {
			continue
		}
		model := r.models[config.PublicModel]
		if !model.Enabled {
			continue
		}
		price := r.prices[config.PublicModel]
		out = append(out, VisibleModel{
			ID:                    config.ID,
			TenantID:              config.TenantID,
			ProjectID:             config.ProjectID,
			PublicModel:           config.PublicModel,
			DisplayName:           config.DisplayName,
			Description:           config.Description,
			Protocol:              model.Protocol,
			Capability:            model.Capability,
			Category:              model.Category,
			Tags:                  append([]string(nil), model.Tags...),
			ProviderFamily:        model.ProviderFamily,
			Modalities:            append([]string(nil), model.Modalities...),
			Capabilities:          append([]string(nil), model.Capabilities...),
			ContextWindow:         model.ContextWindow,
			MaxOutputTokens:       model.MaxOutputTokens,
			Status:                model.Status,
			Deprecated:            model.Deprecated,
			Currency:              price.Currency,
			Components:            append([]pricing.Component(nil), price.Components...),
			InputMicrosPerToken:   price.InputMicrosPerToken,
			OutputMicrosPerToken:  price.OutputMicrosPerToken,
			EstimatedOutputTokens: price.EstimatedOutputTokens,
			SortOrder:             config.SortOrder,
			Metadata:              append([]byte(nil), config.Metadata...),
		})
	}
	return out, nil
}

// LoadSnapshotConfig returns all configuration needed to build a runtime snapshot.
func (r *MemoryRepository) LoadSnapshotConfig(context.Context) (*SnapshotConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg := &SnapshotConfig{}
	for _, key := range r.apiKeys {
		if key.Enabled {
			cfg.APIKeys = append(cfg.APIKeys, key)
		}
		if key.RevokedAt != nil {
			cfg.RevokedKeys = append(cfg.RevokedKeys, key)
		}
	}
	for _, model := range r.models {
		cfg.Models = append(cfg.Models, model)
	}
	for _, channel := range r.channels {
		cfg.Channels = append(cfg.Channels, channel)
	}
	for _, route := range r.routes {
		cfg.Routes = append(cfg.Routes, route)
	}
	for _, price := range r.prices {
		cfg.Prices = append(cfg.Prices, price)
	}
	for _, limit := range r.limits {
		cfg.Limits = append(cfg.Limits, limit)
	}
	for _, binding := range r.plugins {
		cfg.Plugins = append(cfg.Plugins, binding)
	}
	return cfg, nil
}

// SaveSnapshot stores a built runtime snapshot record.
func (r *MemoryRepository) SaveSnapshot(_ context.Context, record SnapshotRecord) (*SnapshotRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	r.snapshots[record.Version] = record
	return clone(record), nil
}

// ActiveSnapshot returns the currently active runtime snapshot record.
func (r *MemoryRepository) ActiveSnapshot(context.Context) (*SnapshotRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.snapshots[r.active]
	return clone(record), ok, nil
}

// PreviousSnapshot returns the most recently deactivated snapshot record.
func (r *MemoryRepository) PreviousSnapshot(context.Context) (*SnapshotRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.snapshots[r.previous]
	return clone(record), ok, nil
}

// ActivateSnapshot marks version active and preserves the previous active version.
func (r *MemoryRepository) ActivateSnapshot(_ context.Context, version string) (*SnapshotRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record := r.snapshots[version]
	now := time.Now().UTC()
	// Step 1: demote the existing active snapshot before promotion.
	if r.active != "" && r.active != version {
		old := r.snapshots[r.active]
		old.Status = SnapshotStatusInactive
		r.snapshots[old.Version] = old
		r.previous = r.active
	}

	// Step 2: promote the requested snapshot as the active runtime version.
	record.Status = SnapshotStatusActive
	record.ActiveAt = &now
	r.snapshots[version] = record
	r.active = version
	return clone(record), nil
}

func clone[T any](value T) *T {
	content, _ := json.Marshal(value)
	var out T
	_ = json.Unmarshal(content, &out)
	return &out
}

func marketplaceID(config ModelMarketplaceConfig) string {
	base := "market_" + config.TenantID + "_" + config.ProjectID + "_" + config.PublicModel
	return strings.Trim(pluginBindingIDRe.ReplaceAllString(base, "_"), "_")
}

func marketplaceScopeMatches(config ModelMarketplaceConfig, tenantID, projectID string) bool {
	if config.TenantID != "" && config.TenantID != tenantID {
		return false
	}
	if config.ProjectID != "" && config.ProjectID != projectID {
		return false
	}
	return true
}
