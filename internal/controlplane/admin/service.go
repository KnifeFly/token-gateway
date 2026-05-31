package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
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

// UpsertProject creates or updates a project.
func (s *Service) UpsertProject(ctx context.Context, project Project) (*Project, error) {
	if project.TenantID == "" || project.Name == "" {
		return nil, apperr.InvalidArgument("tenant_id and name are required")
	}
	project.Enabled = defaultEnabled(project.Enabled, project.EnabledSet)
	return s.repo.UpsertProject(ctx, project)
}

// CreateAPIKey creates an API key and returns the plaintext only once.
func (s *Service) CreateAPIKey(ctx context.Context, key APIKey) (*APIKey, error) {
	if key.TenantID == "" || key.ProjectID == "" {
		return nil, apperr.InvalidArgument("tenant_id and project_id are required")
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

// UpsertModel creates or updates a model.
func (s *Service) UpsertModel(ctx context.Context, model ModelConfig) (*ModelConfig, error) {
	model.PublicModel = strings.TrimSpace(model.PublicModel)
	model.Protocol = strings.TrimSpace(model.Protocol)
	model.Capability = strings.TrimSpace(model.Capability)
	if model.PublicModel == "" || model.Protocol == "" || model.Capability == "" {
		return nil, apperr.InvalidArgument("public_model, protocol, and capability are required")
	}
	model.Aliases = cleanStrings(model.Aliases)
	if model.DisplayName == "" {
		model.DisplayName = model.PublicModel
	}
	if len(model.Schema) == 0 {
		model.Schema = json.RawMessage(`{}`)
	}
	if !json.Valid(model.Schema) {
		return nil, apperr.InvalidArgument("model schema must be valid json")
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
	channel.Enabled = defaultEnabled(channel.Enabled, channel.EnabledSet)
	return s.repo.UpsertChannel(ctx, channel)
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
	if price.PublicModel == "" || strings.TrimSpace(price.Currency) == "" {
		return nil, apperr.InvalidArgument("public_model and currency are required")
	}
	price.Currency = strings.ToUpper(strings.TrimSpace(price.Currency))
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
