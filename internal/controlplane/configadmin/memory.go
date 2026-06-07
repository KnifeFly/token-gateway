package configadmin

import (
	"sync"
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
