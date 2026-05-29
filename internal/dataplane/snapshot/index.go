package snapshot

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// IndexedSnapshot is the data-plane hot-path view of a runtime snapshot.
type IndexedSnapshot struct {
	ref            engine.SnapshotRef
	apiKeysByHash  map[string]engine.APIKeyView
	modelsByName   map[string]engine.ModelView
	channelsByID   map[string]engine.ChannelView
	routesByModel  map[string]engine.RoutePolicyView
	pricesByModel  map[string]engine.PriceRuleView
	limitsByModel  map[string]engine.LimitRuleView
	pluginsByPhase map[string][]engine.PluginBindingView
	revokedHashes  map[string]struct{}
}

// Build validates and indexes a control-plane RuntimeSnapshot.
func Build(runtime cpsnapshot.RuntimeSnapshot) (*IndexedSnapshot, error) {
	if runtime.Version == "" {
		return nil, errors.New("snapshot version is required")
	}
	if runtime.CreatedAt.IsZero() {
		runtime.CreatedAt = time.Now().UTC()
	}
	indexed := &IndexedSnapshot{
		ref: engine.SnapshotRef{
			Version:   runtime.Version,
			CreatedAt: runtime.CreatedAt,
		},
		apiKeysByHash:  make(map[string]engine.APIKeyView, len(runtime.APIKeys)),
		modelsByName:   make(map[string]engine.ModelView, len(runtime.Models)),
		channelsByID:   make(map[string]engine.ChannelView, len(runtime.Channels)),
		routesByModel:  make(map[string]engine.RoutePolicyView, len(runtime.RoutePolicies)),
		pricesByModel:  make(map[string]engine.PriceRuleView, len(runtime.PriceRules)),
		limitsByModel:  make(map[string]engine.LimitRuleView, len(runtime.LimitRules)),
		pluginsByPhase: make(map[string][]engine.PluginBindingView),
		revokedHashes:  make(map[string]struct{}, len(runtime.RevokedKeys)),
	}
	for _, apiKey := range runtime.APIKeys {
		if apiKey.KeyHash == "" {
			return nil, fmt.Errorf("api key %q hash is required", apiKey.ID)
		}
		if _, exists := indexed.apiKeysByHash[apiKey.KeyHash]; exists {
			return nil, fmt.Errorf("duplicate api key hash for %q", apiKey.ID)
		}
		indexed.apiKeysByHash[apiKey.KeyHash] = engine.APIKeyView{
			ID:            apiKey.ID,
			TenantID:      apiKey.TenantID,
			ProjectID:     apiKey.ProjectID,
			Name:          apiKey.Name,
			Hash:          apiKey.KeyHash,
			Enabled:       apiKey.Enabled,
			AllowedModels: append([]string(nil), apiKey.AllowedModels...),
		}
	}
	for _, model := range runtime.Models {
		if model.PublicModel == "" {
			return nil, errors.New("model public name is required")
		}
		if _, exists := indexed.modelsByName[model.PublicModel]; exists {
			return nil, fmt.Errorf("duplicate model %q", model.PublicModel)
		}
		indexed.modelsByName[model.PublicModel] = engine.ModelView{
			PublicModel: model.PublicModel,
			Protocol:    engine.ProtocolMode(model.Protocol),
			Capability:  model.Capability,
			Enabled:     model.Enabled,
		}
	}
	for _, channel := range runtime.Channels {
		if channel.ID == "" {
			return nil, errors.New("channel id is required")
		}
		if _, exists := indexed.channelsByID[channel.ID]; exists {
			return nil, fmt.Errorf("duplicate channel %q", channel.ID)
		}
		models := make(map[string]string, len(channel.Models))
		for _, model := range channel.Models {
			models[model.PublicModel] = model.UpstreamModel
		}
		indexed.channelsByID[channel.ID] = engine.ChannelView{
			ID:              channel.ID,
			ProviderType:    channel.ProviderType,
			BaseURL:         channel.BaseURL,
			APIKey:          channel.APIKey,
			CredentialRef:   channel.CredentialRef,
			EncryptedAPIKey: channel.EncryptedAPIKey,
			Enabled:         channel.Enabled,
			Timeout:         channel.Timeout,
			Models:          models,
		}
	}
	for _, route := range runtime.RoutePolicies {
		if route.PublicModel == "" {
			return nil, errors.New("route public model is required")
		}
		if _, exists := indexed.routesByModel[route.PublicModel]; exists {
			return nil, fmt.Errorf("duplicate route for model %q", route.PublicModel)
		}
		candidates := make([]engine.RouteCandidateView, 0, len(route.Candidates))
		for _, candidate := range route.Candidates {
			candidates = append(candidates, engine.RouteCandidateView{
				ChannelID: candidate.ChannelID,
				Priority:  candidate.Priority,
				Weight:    candidate.Weight,
			})
		}
		indexed.routesByModel[route.PublicModel] = engine.RoutePolicyView{
			ID:          route.ID,
			PublicModel: route.PublicModel,
			Strategy:    route.Strategy,
			Candidates:  candidates,
		}
	}
	for _, price := range runtime.PriceRules {
		if price.PublicModel == "" || !price.Enabled {
			continue
		}
		indexed.pricesByModel[price.PublicModel] = engine.PriceRuleView{
			PublicModel:           price.PublicModel,
			Currency:              price.Currency,
			InputMicrosPerToken:   price.InputMicrosPerToken,
			OutputMicrosPerToken:  price.OutputMicrosPerToken,
			EstimatedOutputTokens: price.EstimatedOutputTokens,
			Enabled:               price.Enabled,
		}
	}
	for _, limit := range runtime.LimitRules {
		if limit.PublicModel == "" || !limit.Enabled {
			continue
		}
		indexed.limitsByModel[limit.PublicModel] = engine.LimitRuleView{
			PublicModel: limit.PublicModel,
			QPS:         limit.QPS,
			TPM:         limit.TPM,
			Concurrency: limit.Concurrency,
			Enabled:     limit.Enabled,
		}
	}
	for _, binding := range runtime.PluginBindings {
		if binding.Name == "" || binding.Phase == "" || !binding.Enabled {
			continue
		}
		config := append([]byte(nil), binding.Config...)
		if len(config) == 0 {
			config = []byte(`{}`)
		}
		indexed.pluginsByPhase[binding.Phase] = append(indexed.pluginsByPhase[binding.Phase], engine.PluginBindingView{
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
	for _, revoked := range runtime.RevokedKeys {
		if revoked.KeyHash != "" {
			indexed.revokedHashes[revoked.KeyHash] = struct{}{}
		}
	}
	return indexed, nil
}

func (s *IndexedSnapshot) Ref() engine.SnapshotRef {
	return s.ref
}

func (s *IndexedSnapshot) LookupAPIKeyHash(hash string) (engine.APIKeyView, bool) {
	value, ok := s.apiKeysByHash[hash]
	return value, ok
}

func (s *IndexedSnapshot) LookupModel(publicModel string) (engine.ModelView, bool) {
	value, ok := s.modelsByName[publicModel]
	return value, ok
}

func (s *IndexedSnapshot) LookupRoute(publicModel string) (engine.RoutePolicyView, bool) {
	value, ok := s.routesByModel[publicModel]
	return value, ok
}

func (s *IndexedSnapshot) LookupChannel(channelID string) (engine.ChannelView, bool) {
	value, ok := s.channelsByID[channelID]
	return value, ok
}

func (s *IndexedSnapshot) LookupPrice(publicModel string) (engine.PriceRuleView, bool) {
	value, ok := s.pricesByModel[publicModel]
	return value, ok
}

func (s *IndexedSnapshot) LookupLimit(publicModel string) (engine.LimitRuleView, bool) {
	value, ok := s.limitsByModel[publicModel]
	return value, ok
}

func (s *IndexedSnapshot) LookupPluginBindings(phase string) []engine.PluginBindingView {
	return s.pluginsByPhase[phase]
}

func (s *IndexedSnapshot) IsAPIKeyRevoked(hash string) bool {
	_, ok := s.revokedHashes[hash]
	return ok
}

// Store keeps the current indexed snapshot as an atomic pointer.
type Store struct {
	current atomic.Pointer[IndexedSnapshot]
}

func NewStore(initial *IndexedSnapshot) *Store {
	store := &Store{}
	if initial != nil {
		store.current.Store(initial)
	}
	return store
}

func (s *Store) Current() (*IndexedSnapshot, error) {
	current := s.current.Load()
	if current == nil {
		return nil, apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	return current, nil
}

func (s *Store) Replace(next *IndexedSnapshot) error {
	if next == nil {
		return errors.New("snapshot is nil")
	}
	s.current.Store(next)
	return nil
}

// Provider attaches the current Store snapshot to a RequestState.
type Provider struct {
	store *Store
}

func NewProvider(store *Store) *Provider {
	return &Provider{store: store}
}

func (p *Provider) Attach(_ context.Context, state *engine.RequestState) error {
	if p == nil || p.store == nil {
		return apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	current, err := p.store.Current()
	if err != nil {
		return err
	}
	state.PinSnapshot(current)
	return nil
}
