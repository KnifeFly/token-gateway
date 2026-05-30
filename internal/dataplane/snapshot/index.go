package snapshot

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// index.go builds and serves the immutable snapshot indexes used on the data-plane hot path.

// IndexedSnapshot is the data-plane hot-path view of a runtime snapshot.
type IndexedSnapshot struct {
	ref            engine.SnapshotRef
	apiKeysByHash  map[string]engine.APIKeyView
	modelsByName   map[string]engine.ModelView
	modelAliases   map[string]string
	channelsByID   map[string]engine.ChannelView
	routesByModel  map[string]engine.RoutePolicyView
	pricesByModel  map[string]engine.PriceRuleView
	limitsByModel  map[string]engine.LimitRuleView
	limitRules     []engine.LimitRuleView
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
		modelAliases:   make(map[string]string),
		channelsByID:   make(map[string]engine.ChannelView, len(runtime.Channels)),
		routesByModel:  make(map[string]engine.RoutePolicyView, len(runtime.RoutePolicies)),
		pricesByModel:  make(map[string]engine.PriceRuleView, len(runtime.PriceRules)),
		limitsByModel:  make(map[string]engine.LimitRuleView, len(runtime.LimitRules)),
		pluginsByPhase: make(map[string][]engine.PluginBindingView),
		revokedHashes:  make(map[string]struct{}, len(runtime.RevokedKeys)),
	}

	// Step 1: validate identity-bearing records and build direct lookup maps.
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
		mappings := make([]engine.ProviderModelMapping, 0, len(model.ProviderMappings))
		for _, mapping := range model.ProviderMappings {
			mappings = append(mappings, engine.ProviderModelMapping{
				ProviderType:  mapping.ProviderType,
				ChannelID:     mapping.ChannelID,
				PublicModel:   mapping.PublicModel,
				UpstreamModel: mapping.UpstreamModel,
			})
		}
		indexed.modelsByName[model.PublicModel] = engine.ModelView{
			PublicModel:      model.PublicModel,
			Aliases:          append([]string(nil), model.Aliases...),
			DisplayName:      model.DisplayName,
			Description:      model.Description,
			Protocol:         engine.ProtocolMode(model.Protocol),
			Capability:       model.Capability,
			Schema:           append([]byte(nil), model.Schema...),
			ProviderMappings: mappings,
			Enabled:          model.Enabled,
		}
		for _, alias := range model.Aliases {
			if alias == "" || alias == model.PublicModel {
				continue
			}
			if _, exists := indexed.modelsByName[alias]; exists {
				return nil, fmt.Errorf("model alias %q conflicts with public model", alias)
			}
			if existing := indexed.modelAliases[alias]; existing != "" && existing != model.PublicModel {
				return nil, fmt.Errorf("duplicate model alias %q", alias)
			}
			indexed.modelAliases[alias] = model.PublicModel
		}
	}

	// Step 2: index provider channels, routes, prices, and limits for hot-path reads.
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
		if !limit.Enabled {
			continue
		}
		view := engine.LimitRuleView{
			ID: limit.ID,
			Scope: engine.LimitScope{
				TenantID:     limit.TenantID,
				ProjectID:    limit.ProjectID,
				APIKeyID:     limit.APIKeyID,
				PublicModel:  limit.PublicModel,
				ProviderType: limit.ProviderType,
				ChannelID:    limit.ChannelID,
			},
			RPM:                 limit.RPM,
			QPS:                 limit.QPS,
			TPM:                 limit.TPM,
			Concurrency:         limit.Concurrency,
			DailyBudgetMicros:   limit.DailyBudgetMicros,
			CostPerMinuteMicros: limit.CostPerMinuteMicros,
			Enabled:             limit.Enabled,
		}
		if view.ID == "" {
			view.ID = limitKey(view.Scope)
		}
		indexed.limitRules = append(indexed.limitRules, view)
		if limit.PublicModel != "" {
			indexed.limitsByModel[limit.PublicModel] = view
		}
	}
	sort.SliceStable(indexed.limitRules, func(i, j int) bool {
		return limitSpecificity(indexed.limitRules[i].Scope) > limitSpecificity(indexed.limitRules[j].Scope)
	})

	// Step 3: index plugin bindings and revocations by hot-path lookup key.
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

// Ref returns the immutable snapshot reference.
func (s *IndexedSnapshot) Ref() engine.SnapshotRef {
	return s.ref
}

// ListModels returns enabled model views sorted by public model.
func (s *IndexedSnapshot) ListModels() []engine.ModelView {
	models := make([]engine.ModelView, 0, len(s.modelsByName))
	for _, model := range s.modelsByName {
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].PublicModel < models[j].PublicModel
	})
	return models
}

// LookupAPIKeyHash returns API key metadata by hashed key.
func (s *IndexedSnapshot) LookupAPIKeyHash(hash string) (engine.APIKeyView, bool) {
	value, ok := s.apiKeysByHash[hash]
	return value, ok
}

// LookupModel returns a model by public name or alias.
func (s *IndexedSnapshot) LookupModel(publicModel string) (engine.ModelView, bool) {
	if canonical := s.modelAliases[publicModel]; canonical != "" {
		publicModel = canonical
	}
	value, ok := s.modelsByName[publicModel]
	return value, ok
}

// LookupRoute returns the route policy for a public model.
func (s *IndexedSnapshot) LookupRoute(publicModel string) (engine.RoutePolicyView, bool) {
	value, ok := s.routesByModel[publicModel]
	return value, ok
}

// LookupChannel returns provider channel configuration by ID.
func (s *IndexedSnapshot) LookupChannel(channelID string) (engine.ChannelView, bool) {
	value, ok := s.channelsByID[channelID]
	return value, ok
}

// LookupPrice returns the price rule for a public model.
func (s *IndexedSnapshot) LookupPrice(publicModel string) (engine.PriceRuleView, bool) {
	value, ok := s.pricesByModel[publicModel]
	return value, ok
}

// LookupLimit returns the model-level limit rule for a public model.
func (s *IndexedSnapshot) LookupLimit(publicModel string) (engine.LimitRuleView, bool) {
	value, ok := s.limitsByModel[publicModel]
	return value, ok
}

// LookupLimits returns all scoped limit rules matching a request scope.
func (s *IndexedSnapshot) LookupLimits(scope engine.LimitScope) []engine.LimitRuleView {
	var out []engine.LimitRuleView
	for _, rule := range s.limitRules {
		if limitScopeMatches(rule.Scope, scope) {
			out = append(out, rule)
		}
	}
	return out
}

// LookupPluginBindings returns plugin bindings for a phase.
func (s *IndexedSnapshot) LookupPluginBindings(phase string) []engine.PluginBindingView {
	return s.pluginsByPhase[phase]
}

// IsAPIKeyRevoked reports whether hash is in the snapshot revocation set.
func (s *IndexedSnapshot) IsAPIKeyRevoked(hash string) bool {
	_, ok := s.revokedHashes[hash]
	return ok
}

// Store keeps the current indexed snapshot as an atomic pointer.
type Store struct {
	current atomic.Pointer[IndexedSnapshot]
}

// StoreDiagnostics describes the indexed snapshot currently used by the hot path.
type StoreDiagnostics struct {
	Current          engine.SnapshotRef
	StalenessSeconds float64
}

// NewStore returns an atomic snapshot store seeded with initial.
func NewStore(initial *IndexedSnapshot) *Store {
	store := &Store{}
	if initial != nil {
		store.current.Store(initial)
	}
	return store
}

// Current returns the currently indexed snapshot.
func (s *Store) Current() (*IndexedSnapshot, error) {
	current := s.current.Load()
	if current == nil {
		return nil, apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	return current, nil
}

// Replace swaps the current indexed snapshot atomically.
func (s *Store) Replace(next *IndexedSnapshot) error {
	if next == nil {
		return errors.New("snapshot is nil")
	}
	s.current.Store(next)
	return nil
}

// Diagnostics returns current hot-path snapshot age without reading control-plane tables.
func (s *Store) Diagnostics(now time.Time) (StoreDiagnostics, error) {
	current, err := s.Current()
	if err != nil {
		return StoreDiagnostics{}, err
	}
	ref := current.Ref()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	diagnostics := StoreDiagnostics{Current: ref}
	if !ref.CreatedAt.IsZero() {
		diagnostics.StalenessSeconds = now.Sub(ref.CreatedAt).Seconds()
	}
	return diagnostics, nil
}

func limitScopeMatches(rule, request engine.LimitScope) bool {
	return scopeValueMatches(rule.TenantID, request.TenantID) &&
		scopeValueMatches(rule.ProjectID, request.ProjectID) &&
		scopeValueMatches(rule.APIKeyID, request.APIKeyID) &&
		scopeValueMatches(rule.PublicModel, request.PublicModel) &&
		scopeValueMatches(rule.ProviderType, request.ProviderType) &&
		scopeValueMatches(rule.ChannelID, request.ChannelID)
}

func scopeValueMatches(rule, request string) bool {
	return rule == "" || rule == request
}

func limitSpecificity(scope engine.LimitScope) int {
	score := 0
	if scope.TenantID != "" {
		score++
	}
	if scope.ProjectID != "" {
		score++
	}
	if scope.APIKeyID != "" {
		score++
	}
	if scope.PublicModel != "" {
		score++
	}
	if scope.ProviderType != "" {
		score++
	}
	if scope.ChannelID != "" {
		score++
	}
	return score
}

func limitKey(scope engine.LimitScope) string {
	return fmt.Sprintf("limit:%s:%s:%s:%s:%s:%s", scope.TenantID, scope.ProjectID, scope.APIKeyID, scope.PublicModel, scope.ProviderType, scope.ChannelID)
}

// ProviderOption configures a Provider.
type ProviderOption func(*Provider)

// WithMetrics updates snapshot staleness from request attachment.
func WithMetrics(metrics *Metrics) ProviderOption {
	return func(p *Provider) {
		p.metrics = metrics
	}
}

// Provider attaches the current Store snapshot to a RequestState.
type Provider struct {
	store   *Store
	metrics *Metrics
}

// NewProvider returns a snapshot provider backed by store.
func NewProvider(store *Store, opts ...ProviderOption) *Provider {
	p := &Provider{store: store}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Attach pins the current indexed snapshot on the request state.
func (p *Provider) Attach(_ context.Context, state *engine.RequestState) error {
	if p == nil || p.store == nil {
		return apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	current, err := p.store.Current()
	if err != nil {
		return err
	}
	ref := current.Ref()
	if p.metrics != nil {
		p.metrics.Observe(ref)
	}
	state.PinSnapshot(current)
	return nil
}
