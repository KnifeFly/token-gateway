package configadmin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const (
	failurePolicyFailClosed = "fail_closed"
	failurePolicyFailOpen   = "fail_open"
)

var pluginBindingIDRe = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// Service validates and writes control-plane admin configuration.
type Service struct {
	repo       Repository
	codec      *CredentialCodec
	hasher     *auth.APIKeyHasher
	revocation interface {
		Revoke(ctx context.Context, keyHash string) error
	}
}

// ServiceOption customizes the control-plane admin service.
type ServiceOption func(*Service)

// NewService returns a control-plane admin service.
func NewService(repo Repository, codec *CredentialCodec, revocation *redisinfra.RevocationStore, options ...ServiceOption) *Service {
	service := &Service{repo: repo, codec: codec, hasher: auth.NewAPIKeyHasher(""), revocation: revocation}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	if service.hasher == nil {
		service.hasher = auth.NewAPIKeyHasher("")
	}
	return service
}

// WithAPIKeyHasher configures the hash format used for newly created API keys.
func WithAPIKeyHasher(hasher *auth.APIKeyHasher) ServiceOption {
	return func(service *Service) {
		service.hasher = hasher
	}
}

// UpsertTenant creates or updates a tenant.
func (s *Service) UpsertTenant(ctx context.Context, tenant Tenant) (*Tenant, error) {
	if tenant.Name == "" {
		return nil, apperr.InvalidArgument("tenant name is required")
	}
	tenant.Enabled = defaultEnabled(tenant.Enabled, tenant.EnabledSet)
	return s.repo.UpsertTenant(ctx, tenant)
}

// ListTenants returns tenant configuration rows for owner-mediated Admin reads.
func (s *Service) ListTenants(ctx context.Context) ([]Tenant, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("admin repository is unavailable")
	}
	return s.repo.ListTenants(ctx)
}

// UpsertProject creates or updates a project.
func (s *Service) UpsertProject(ctx context.Context, project Project) (*Project, error) {
	if project.TenantID == "" || project.Name == "" {
		return nil, apperr.InvalidArgument("tenant_id and name are required")
	}
	project.Enabled = defaultEnabled(project.Enabled, project.EnabledSet)
	return s.repo.UpsertProject(ctx, project)
}

// ListProjects returns project configuration rows for one tenant or all tenants.
func (s *Service) ListProjects(ctx context.Context, tenantID string) ([]Project, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("admin repository is unavailable")
	}
	return s.repo.ListProjects(ctx, strings.TrimSpace(tenantID))
}

// CreateAPIKey creates an API key and returns the plaintext only once.
func (s *Service) CreateAPIKey(ctx context.Context, key APIKey) (*APIKey, error) {
	if key.TenantID == "" || key.ProjectID == "" {
		return nil, apperr.InvalidArgument("tenant_id and project_id are required")
	}
	key.Name = strings.TrimSpace(key.Name)
	key.AllowedModels = cleanStrings(key.AllowedModels)
	key.IPAllowlist = cleanIPAllowlist(key.IPAllowlist)
	if err := validateIPAllowlist(key.IPAllowlist); err != nil {
		return nil, err
	}
	plaintext := key.PlaintextKey
	if plaintext == "" {
		plaintext = newPlaintextKey()
	}
	key.KeyHash = s.hasher.Hash(plaintext)
	key.PlaintextKey = plaintext
	if key.Name == "" {
		key.Name = "api key"
	}
	if len(key.AllowedModels) == 0 {
		key.AllowedModels = []string{"*"}
	}
	key.Enabled = true
	created, err := s.repo.CreateAPIKey(ctx, key)
	if err != nil {
		return nil, err
	}
	created.PlaintextKey = plaintext
	return created, nil
}

// ListAPIKeys lists safe API key metadata.
func (s *Service) ListAPIKeys(ctx context.Context, tenantID, projectID string) ([]APIKey, error) {
	return s.repo.ListAPIKeys(ctx, tenantID, projectID)
}

// UpdateAPIKey updates safe API key metadata without returning plaintext.
func (s *Service) UpdateAPIKey(ctx context.Context, key APIKey) (*APIKey, error) {
	current, err := s.findAPIKey(ctx, key.ID)
	if err != nil {
		return nil, err
	}
	key.TenantID = current.TenantID
	key.ProjectID = current.ProjectID
	key.KeyHash = current.KeyHash
	key.PlaintextKey = ""
	key.Enabled = current.Enabled
	key.RevokedAt = current.RevokedAt
	key.CreatedAt = current.CreatedAt
	key.LastUsedAt = current.LastUsedAt
	key.Name = strings.TrimSpace(key.Name)
	if key.Name == "" {
		key.Name = current.Name
	}
	key.AllowedModels = cleanStrings(key.AllowedModels)
	if len(key.AllowedModels) == 0 {
		key.AllowedModels = append([]string(nil), current.AllowedModels...)
	}
	key.IPAllowlist = cleanIPAllowlist(key.IPAllowlist)
	if key.IPAllowlist == nil {
		key.IPAllowlist = append([]string(nil), current.IPAllowlist...)
	}
	if key.ExpiresAt == nil {
		key.ExpiresAt = current.ExpiresAt
	}
	if err := validateIPAllowlist(key.IPAllowlist); err != nil {
		return nil, err
	}
	return s.repo.UpdateAPIKey(ctx, key)
}

// EnableAPIKey enables a stored API key and clears revocation metadata.
func (s *Service) EnableAPIKey(ctx context.Context, keyID string) (*APIKey, error) {
	key, err := s.findAPIKey(ctx, keyID)
	if err != nil {
		return nil, err
	}
	key.Enabled = true
	key.RevokedAt = nil
	key.PlaintextKey = ""
	return s.repo.UpdateAPIKey(ctx, *key)
}

// DisableAPIKey disables an API key and writes fast revocation state.
func (s *Service) DisableAPIKey(ctx context.Context, keyID string) (*APIKey, error) {
	now := time.Now().UTC()
	key, err := s.repo.DisableAPIKey(ctx, keyID, &now)
	if err != nil {
		return nil, err
	}
	if s.revocation != nil && key.KeyHash != "" {
		if err := s.revocation.Revoke(ctx, key.KeyHash); err != nil {
			return nil, err
		}
	}
	return key, nil
}

// RotateAPIKey replaces one API key hash and returns the new plaintext once.
func (s *Service) RotateAPIKey(ctx context.Context, keyID string, plaintext string) (*APIKey, error) {
	key, err := s.findAPIKey(ctx, keyID)
	if err != nil {
		return nil, err
	}
	oldHash := key.KeyHash
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		plaintext = newPlaintextKey()
	}
	key.KeyHash = s.hasher.Hash(plaintext)
	key.PlaintextKey = ""
	key.Enabled = true
	key.RevokedAt = nil
	updated, err := s.repo.UpdateAPIKey(ctx, *key)
	if err != nil {
		return nil, err
	}
	if s.revocation != nil && oldHash != "" {
		if err := s.revocation.Revoke(ctx, oldHash); err != nil {
			return nil, err
		}
	}
	updated.PlaintextKey = plaintext
	return updated, nil
}

func (s *Service) findAPIKey(ctx context.Context, keyID string) (*APIKey, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, apperr.InvalidArgument("api key id is required")
	}
	keys, err := s.repo.ListAPIKeys(ctx, "", "")
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if key.ID == keyID {
			return &key, nil
		}
	}
	return nil, apperr.NotFound("api key not found")
}

// UpsertModel creates or updates a model.
func (s *Service) UpsertModel(ctx context.Context, model ModelConfig) (*ModelConfig, error) {
	model.PublicModel = strings.TrimSpace(model.PublicModel)
	model.Protocol = strings.TrimSpace(model.Protocol)
	model.Capability = strings.TrimSpace(model.Capability)
	if model.PublicModel == "" || model.Protocol == "" || model.Capability == "" {
		return nil, apperr.InvalidArgument("public_model, protocol, and capability are required")
	}
	category, err := pricing.InferCategory(model.Category, model.Capability, model.PublicModel)
	if err != nil {
		return nil, apperr.InvalidArgument(err.Error())
	}
	model.Category = string(category)
	model.Aliases = cleanStrings(model.Aliases)
	model.Tags = cleanStrings(model.Tags)
	model.ProviderFamily = strings.TrimSpace(model.ProviderFamily)
	model.Modalities = cleanStrings(model.Modalities)
	model.Capabilities = cleanStrings(model.Capabilities)
	if model.DisplayName == "" {
		model.DisplayName = model.PublicModel
	}
	if model.Status == "" {
		model.Status = "active"
	}
	if len(model.Schema) == 0 {
		model.Schema = json.RawMessage(`{}`)
	}
	if !json.Valid(model.Schema) {
		return nil, apperr.InvalidArgument("model schema must be valid json")
	}
	if len(model.Metadata) == 0 {
		model.Metadata = json.RawMessage(`{}`)
	}
	if !json.Valid(model.Metadata) {
		return nil, apperr.InvalidArgument("model metadata must be valid json")
	}
	model.Enabled = defaultEnabled(model.Enabled, model.EnabledSet)
	return s.repo.UpsertModel(ctx, model)
}

// UpsertChannel creates or updates a provider channel and encrypts credentials.
func (s *Service) UpsertChannel(ctx context.Context, channel ChannelConfig) (*ChannelConfig, error) {
	if channel.ProviderType == "" || channel.BaseURL == "" {
		return nil, apperr.InvalidArgument("provider_type and base_url are required")
	}
	if channel.ID == "" {
		channel.ID = newID("channel")
	}
	if channel.CredentialRef == "" {
		channel.CredentialRef = "credential:" + channel.ID
	}
	if channel.APIKey != "" {
		ciphertext, err := s.codec.Encrypt(channel.APIKey)
		if err != nil {
			return nil, err
		}
		channel.EncryptedAPIKey = ciphertext
		channel.APIKey = ""
	}
	for i := range channel.Models {
		normalized, err := normalizeChannelModel(channel.Models[i])
		if err != nil {
			return nil, err
		}
		channel.Models[i] = normalized
	}
	channel.Enabled = defaultEnabled(channel.Enabled, channel.EnabledSet)
	return s.repo.UpsertChannel(ctx, channel)
}

// PreviewChannelModelSync compares current channel models with upstream discovery results without writing.
func (s *Service) PreviewChannelModelSync(ctx context.Context, request ChannelModelSyncPreviewRequest) (*ChannelModelSyncPreview, error) {
	request.ChannelID = strings.TrimSpace(request.ChannelID)
	if request.ChannelID == "" {
		return nil, apperr.InvalidArgument("channel_id is required")
	}
	cfg, err := s.repo.LoadSnapshotConfig(ctx)
	if err != nil {
		return nil, err
	}
	models := make(map[string]ModelConfig, len(cfg.Models))
	for _, model := range cfg.Models {
		models[model.PublicModel] = model
	}
	prices := make(map[string]PriceRuleConfig, len(cfg.Prices))
	for _, price := range cfg.Prices {
		if price.Enabled {
			prices[price.PublicModel] = price
		}
	}

	var current ChannelConfig
	for _, channel := range cfg.Channels {
		if channel.ID == request.ChannelID {
			current = channel
			break
		}
	}
	if current.ID == "" {
		return nil, apperr.NotFound("channel not found")
	}
	preview := &ChannelModelSyncPreview{
		ChannelID:    current.ID,
		ProviderType: firstNonEmpty(request.ProviderType, current.ProviderType),
	}
	currentByModel := make(map[string]ChannelModel, len(current.Models))
	for _, model := range current.Models {
		currentByModel[model.PublicModel] = model
	}
	upstreamByModel := make(map[string]ChannelModel, len(request.UpstreamModels))
	for _, model := range request.UpstreamModels {
		normalized, err := normalizeChannelModel(model)
		if err != nil {
			return nil, err
		}
		upstreamByModel[normalized.PublicModel] = normalized
		if existing, ok := currentByModel[normalized.PublicModel]; !ok {
			preview.Added = append(preview.Added, previewItem(normalized, ChannelModel{}, models, prices))
		} else if existing.UpstreamModel != normalized.UpstreamModel {
			preview.Changed = append(preview.Changed, previewItem(normalized, existing, models, prices))
		} else {
			preview.Unchanged++
		}
	}
	for publicModel, existing := range currentByModel {
		if _, ok := upstreamByModel[publicModel]; !ok {
			preview.Removed = append(preview.Removed, previewItem(existing, existing, models, prices))
		}
	}
	for _, item := range append(append([]ChannelModelPreviewItem{}, preview.Added...), preview.Changed...) {
		if !item.KnownCatalogModel {
			preview.Warnings = append(preview.Warnings, "upstream model "+item.PublicModel+" is not in the public model catalog")
			continue
		}
		if !item.CustomerPriceConfigured {
			preview.Warnings = append(preview.Warnings, "public model "+item.PublicModel+" has no enabled customer price rule")
		}
		if item.CostConfigStatus != "configured" {
			preview.Warnings = append(preview.Warnings, "public model "+item.PublicModel+" has no configured provider cost status")
		}
	}
	return preview, nil
}

// UpsertRoute creates or updates a route policy.
func (s *Service) UpsertRoute(ctx context.Context, route RoutePolicyConfig) (*RoutePolicyConfig, error) {
	if route.PublicModel == "" || len(route.Candidates) == 0 {
		return nil, apperr.InvalidArgument("public_model and candidates are required")
	}
	if route.ID == "" {
		route.ID = "route_" + route.PublicModel
	}
	if route.Strategy == "" {
		route.Strategy = "priority"
	}
	route.Enabled = defaultEnabled(route.Enabled, route.EnabledSet)
	for i := range route.Candidates {
		if route.Candidates[i].Weight <= 0 {
			route.Candidates[i].Weight = 100
		}
	}
	return s.repo.UpsertRoute(ctx, route)
}

// UpsertPrice creates or updates a price rule.
func (s *Service) UpsertPrice(ctx context.Context, price PriceRuleConfig) (*PriceRuleConfig, error) {
	price.PublicModel = strings.TrimSpace(price.PublicModel)
	if price.PublicModel == "" || strings.TrimSpace(price.Currency) == "" {
		return nil, apperr.InvalidArgument("public_model and currency are required")
	}
	price.Currency = strings.ToUpper(strings.TrimSpace(price.Currency))
	category, err := pricing.InferCategory(price.Category, price.PublicModel)
	if err != nil {
		return nil, apperr.InvalidArgument(err.Error())
	}
	book, err := pricing.NormalizePriceBook(pricing.PriceBook{
		Category:   category,
		Currency:   price.Currency,
		Components: price.Components,
	}, pricing.TokenPrice{
		Currency:             price.Currency,
		InputMicrosPerToken:  price.InputMicrosPerToken,
		OutputMicrosPerToken: price.OutputMicrosPerToken,
	})
	if err != nil {
		return nil, apperr.InvalidArgument(err.Error())
	}
	legacy := pricing.LegacyTokenPrice(book.Currency, book.Components)
	price.Category = string(book.Category)
	price.Currency = book.Currency
	price.Components = book.Components
	price.InputMicrosPerToken = legacy.InputMicrosPerToken
	price.OutputMicrosPerToken = legacy.OutputMicrosPerToken
	if len(price.Metadata) == 0 {
		price.Metadata = json.RawMessage(`{}`)
	}
	if !json.Valid(price.Metadata) {
		return nil, apperr.InvalidArgument("price metadata must be valid json")
	}
	price.Enabled = defaultEnabled(price.Enabled, price.EnabledSet)
	return s.repo.UpsertPrice(ctx, price)
}

// UpsertLimit creates or updates a limit rule.
func (s *Service) UpsertLimit(ctx context.Context, limit LimitRuleConfig) (*LimitRuleConfig, error) {
	limit.TenantID = strings.TrimSpace(limit.TenantID)
	limit.ProjectID = strings.TrimSpace(limit.ProjectID)
	limit.APIKeyID = strings.TrimSpace(limit.APIKeyID)
	limit.PublicModel = strings.TrimSpace(limit.PublicModel)
	limit.ProviderType = strings.TrimSpace(limit.ProviderType)
	limit.ChannelID = strings.TrimSpace(limit.ChannelID)
	if limit.TenantID == "" && limit.ProjectID == "" && limit.APIKeyID == "" && limit.PublicModel == "" && limit.ProviderType == "" && limit.ChannelID == "" {
		return nil, apperr.InvalidArgument("at least one limit scope dimension is required")
	}
	if limit.ID == "" {
		limit.ID = limitRuleID(limit)
	}
	normalizeLimitBudgetAlias(&limit)
	limit.Enabled = defaultEnabled(limit.Enabled, limit.EnabledSet)
	return s.repo.UpsertLimit(ctx, limit)
}

// UpsertPluginBinding creates or updates a built-in plugin binding.
func (s *Service) UpsertPluginBinding(ctx context.Context, binding PluginBindingConfig) (*PluginBindingConfig, error) {
	binding.Name = strings.TrimSpace(binding.Name)
	binding.Phase = strings.TrimSpace(binding.Phase)
	if binding.Name == "" || binding.Phase == "" {
		return nil, apperr.InvalidArgument("plugin name and phase are required")
	}
	if !validPluginPhase(binding.Phase) {
		return nil, apperr.InvalidArgument("plugin phase is not supported")
	}
	if binding.ID == "" {
		binding.ID = pluginBindingID(binding)
	}
	binding.Enabled = defaultEnabled(binding.Enabled, binding.EnabledSet)
	if binding.Priority == 0 {
		binding.Priority = 100
	}
	if binding.FailurePolicy == "" {
		binding.FailurePolicy = failurePolicyFailClosed
	}
	if binding.FailurePolicy != failurePolicyFailClosed && binding.FailurePolicy != failurePolicyFailOpen {
		return nil, apperr.InvalidArgument("plugin failure_policy is not supported")
	}
	if len(binding.Config) == 0 {
		binding.Config = json.RawMessage(`{}`)
	}
	if !json.Valid(binding.Config) {
		return nil, apperr.InvalidArgument("plugin config must be valid json")
	}
	return s.repo.UpsertPluginBinding(ctx, binding)
}

// UpsertModelMarketplace creates or updates tenant-visible model catalog rows.
func (s *Service) UpsertModelMarketplace(ctx context.Context, config ModelMarketplaceConfig) (*ModelMarketplaceConfig, error) {
	config.TenantID = strings.TrimSpace(config.TenantID)
	config.ProjectID = strings.TrimSpace(config.ProjectID)
	config.PublicModel = strings.TrimSpace(config.PublicModel)
	config.DisplayName = strings.TrimSpace(config.DisplayName)
	if config.PublicModel == "" {
		return nil, apperr.InvalidArgument("public_model is required")
	}
	if config.DisplayName == "" {
		config.DisplayName = config.PublicModel
	}
	config.Enabled = defaultEnabled(config.Enabled, config.EnabledSet)
	if len(config.Metadata) == 0 {
		config.Metadata = json.RawMessage(`{}`)
	}
	if !json.Valid(config.Metadata) {
		return nil, apperr.InvalidArgument("metadata must be valid json")
	}
	return s.repo.UpsertModelMarketplace(ctx, config)
}

// ListVisibleModels returns enabled model marketplace rows for a tenant/project.
func (s *Service) ListVisibleModels(ctx context.Context, tenantID, projectID string) ([]VisibleModel, error) {
	return s.repo.ListVisibleModels(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(projectID))
}

// LoadSnapshotConfig returns the normalized config graph for safe Admin read models.
func (s *Service) LoadSnapshotConfig(ctx context.Context) (*SnapshotConfig, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("admin repository is unavailable")
	}
	return s.repo.LoadSnapshotConfig(ctx)
}

func newPlaintextKey() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "tg_" + newID("key")
	}
	return "tg_" + base64.RawURLEncoding.EncodeToString(b[:])
}

func validPluginPhase(phase string) bool {
	switch phase {
	case "pre_request", "post_auth", "pre_prompt", "pre_route", "post_route", "pre_provider", "post_provider", "pre_settlement", "audit":
		return true
	default:
		return false
	}
}

func pluginBindingID(binding PluginBindingConfig) string {
	base := fmt.Sprintf("plugin_%s_%s_%s_%s_%s", binding.Phase, binding.Name, binding.TenantID, binding.ProjectID, binding.Model)
	return strings.Trim(pluginBindingIDRe.ReplaceAllString(base, "_"), "_")
}

func limitRuleID(limit LimitRuleConfig) string {
	base := fmt.Sprintf("limit_%s_%s_%s_%s_%s_%s", limit.TenantID, limit.ProjectID, limit.APIKeyID, limit.PublicModel, limit.ProviderType, limit.ChannelID)
	return strings.Trim(pluginBindingIDRe.ReplaceAllString(base, "_"), "_")
}

func normalizeLimitBudgetAlias(limit *LimitRuleConfig) {
	if limit == nil {
		return
	}
	if limit.DailyAdmissionBudgetMicros == 0 && limit.DailyBudgetMicros > 0 {
		limit.DailyAdmissionBudgetMicros = limit.DailyBudgetMicros
	}
	if limit.DailyBudgetMicros == 0 && limit.DailyAdmissionBudgetMicros > 0 {
		limit.DailyBudgetMicros = limit.DailyAdmissionBudgetMicros
	}
}

func defaultEnabled(enabled bool, explicitlySet bool) bool {
	if explicitlySet {
		return enabled
	}
	return true
}

func cleanStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cleanIPAllowlist(values []string) []string {
	return cleanStrings(values)
}

func validateIPAllowlist(values []string) error {
	for _, value := range values {
		if _, _, err := net.ParseCIDR(value); err == nil {
			continue
		}
		if net.ParseIP(value) != nil {
			continue
		}
		return apperr.InvalidArgument("ip_allowlist must contain IP addresses or CIDR ranges")
	}
	return nil
}

func defaultStatus(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizeChannelModel(model ChannelModel) (ChannelModel, error) {
	model.PublicModel = strings.TrimSpace(model.PublicModel)
	model.UpstreamModel = strings.TrimSpace(model.UpstreamModel)
	if model.PublicModel == "" {
		model.PublicModel = model.UpstreamModel
	}
	if model.PublicModel == "" || model.UpstreamModel == "" {
		return ChannelModel{}, apperr.InvalidArgument("public_model and upstream_model are required")
	}
	model.Capabilities = cleanStrings(model.Capabilities)
	model.SupportedParameters = cleanStrings(model.SupportedParameters)
	model.HealthStatus = defaultStatus(model.HealthStatus, "unknown")
	model.TestStatus = defaultStatus(model.TestStatus, "untested")
	model.CostConfigStatus = defaultStatus(model.CostConfigStatus, "unknown")
	if len(model.Metadata) == 0 {
		model.Metadata = json.RawMessage(`{}`)
	}
	if !json.Valid(model.Metadata) {
		return ChannelModel{}, apperr.InvalidArgument("channel model metadata must be valid json")
	}
	return model, nil
}

func previewItem(model ChannelModel, current ChannelModel, models map[string]ModelConfig, prices map[string]PriceRuleConfig) ChannelModelPreviewItem {
	_, known := models[model.PublicModel]
	_, priced := prices[model.PublicModel]
	return ChannelModelPreviewItem{
		PublicModel:             model.PublicModel,
		UpstreamModel:           model.UpstreamModel,
		CurrentUpstreamModel:    current.UpstreamModel,
		HealthStatus:            model.HealthStatus,
		TestStatus:              model.TestStatus,
		CostConfigStatus:        model.CostConfigStatus,
		KnownCatalogModel:       known,
		CustomerPriceConfigured: priced,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
