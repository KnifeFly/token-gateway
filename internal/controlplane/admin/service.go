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
	revocation interface {
		Revoke(ctx context.Context, keyHash string) error
	}
}

// NewService returns a control-plane admin service.
func NewService(repo Repository, codec *CredentialCodec, revocation *redisinfra.RevocationStore) *Service {
	return &Service{repo: repo, codec: codec, revocation: revocation}
}

// UpsertTenant creates or updates a tenant.
func (s *Service) UpsertTenant(ctx context.Context, tenant Tenant) (*Tenant, error) {
	if tenant.Name == "" {
		return nil, apperr.InvalidArgument("tenant name is required")
	}
	if !tenant.Enabled {
		tenant.Enabled = true
	}
	return s.repo.UpsertTenant(ctx, tenant)
}

// UpsertProject creates or updates a project.
func (s *Service) UpsertProject(ctx context.Context, project Project) (*Project, error) {
	if project.TenantID == "" || project.Name == "" {
		return nil, apperr.InvalidArgument("tenant_id and name are required")
	}
	if !project.Enabled {
		project.Enabled = true
	}
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
	key.KeyHash = auth.HashAPIKey(plaintext)
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
	if model.PublicModel == "" || model.Protocol == "" || model.Capability == "" {
		return nil, apperr.InvalidArgument("public_model, protocol, and capability are required")
	}
	if !model.Enabled {
		model.Enabled = true
	}
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
	if !channel.Enabled {
		channel.Enabled = true
	}
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
	if !route.Enabled {
		route.Enabled = true
	}
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
	if !price.Enabled {
		price.Enabled = true
	}
	return s.repo.UpsertPrice(ctx, price)
}

// UpsertLimit creates or updates a limit rule.
func (s *Service) UpsertLimit(ctx context.Context, limit LimitRuleConfig) (*LimitRuleConfig, error) {
	if limit.PublicModel == "" {
		return nil, apperr.InvalidArgument("public_model is required")
	}
	if !limit.Enabled {
		limit.Enabled = true
	}
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
		if !binding.Enabled {
			binding.Enabled = true
		}
	}
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
