package provider

import (
	"fmt"
	"sync"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
)

// Registry maps provider types to relay adapters.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]relay.Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]relay.Adapter)}
}

func (r *Registry) Register(providerType string, adapter relay.Adapter) error {
	if providerType == "" {
		return fmt.Errorf("provider type is required")
	}
	if adapter == nil {
		return fmt.Errorf("adapter is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[providerType] = adapter
	return nil
}

func (r *Registry) Adapter(providerType string) (relay.Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[providerType]
	return adapter, ok
}
