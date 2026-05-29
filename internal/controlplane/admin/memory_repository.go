package admin

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
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

func (r *MemoryRepository) UpsertModel(_ context.Context, model ModelConfig) (*ModelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[model.PublicModel] = model
	return clone(model), nil
}

func (r *MemoryRepository) UpsertChannel(_ context.Context, channel ChannelConfig) (*ChannelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[channel.ID] = channel
	return clone(channel), nil
}

func (r *MemoryRepository) UpsertRoute(_ context.Context, route RoutePolicyConfig) (*RoutePolicyConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[route.ID] = route
	return clone(route), nil
}

func (r *MemoryRepository) UpsertPrice(_ context.Context, price PriceRuleConfig) (*PriceRuleConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prices[price.PublicModel] = price
	return clone(price), nil
}

func (r *MemoryRepository) UpsertLimit(_ context.Context, limit LimitRuleConfig) (*LimitRuleConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limits[limit.PublicModel] = limit
	return clone(limit), nil
}

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
			Currency:              price.Currency,
			InputMicrosPerToken:   price.InputMicrosPerToken,
			OutputMicrosPerToken:  price.OutputMicrosPerToken,
			EstimatedOutputTokens: price.EstimatedOutputTokens,
			SortOrder:             config.SortOrder,
			Metadata:              append([]byte(nil), config.Metadata...),
		})
	}
	return out, nil
}

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

func (r *MemoryRepository) SaveSnapshot(_ context.Context, record SnapshotRecord) (*SnapshotRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	r.snapshots[record.Version] = record
	return clone(record), nil
}

func (r *MemoryRepository) ActiveSnapshot(context.Context) (*SnapshotRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.snapshots[r.active]
	return clone(record), ok, nil
}

func (r *MemoryRepository) PreviousSnapshot(context.Context) (*SnapshotRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.snapshots[r.previous]
	return clone(record), ok, nil
}

func (r *MemoryRepository) ActivateSnapshot(_ context.Context, version string) (*SnapshotRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record := r.snapshots[version]
	now := time.Now().UTC()
	if r.active != "" && r.active != version {
		old := r.snapshots[r.active]
		old.Status = SnapshotStatusInactive
		r.snapshots[old.Version] = old
		r.previous = r.active
	}
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
