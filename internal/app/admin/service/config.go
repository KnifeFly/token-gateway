package service

import (
	"context"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListTenants returns tenant read models.
func (s *Service) ListTenants(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[configadmin.Tenant], error) {
	if err := s.Authorize(actor, "read", "tenant"); err != nil {
		return adminapp.ListResponse[configadmin.Tenant]{}, err
	}
	tenants, err := s.owner.ListTenants(ctx)
	return adminapp.ListResponse[configadmin.Tenant]{Data: tenants}, err
}

// GetTenant returns one tenant read model.
func (s *Service) GetTenant(ctx context.Context, actor adminapp.Actor, tenantID string) (configadmin.Tenant, error) {
	tenants, err := s.ListTenants(ctx, actor)
	if err != nil {
		return configadmin.Tenant{}, err
	}
	for _, tenant := range tenants.Data {
		if tenant.ID == tenantID {
			return tenant, nil
		}
	}
	return configadmin.Tenant{}, apperr.NotFound("tenant not found")
}

// UpsertTenant writes tenant configuration through the control-plane owner service.
func (s *Service) UpsertTenant(ctx context.Context, actor adminapp.Actor, request configadmin.Tenant, opts adminapp.MutationOptions) (*configadmin.Tenant, error) {
	return mutate(ctx, s, actor, opts, "write", "tenant", request.ID, request, func() (*configadmin.Tenant, error) {
		return s.owner.UpsertTenant(ctx, request)
	})
}

// ListProjects returns project read models.
func (s *Service) ListProjects(ctx context.Context, actor adminapp.Actor, tenantID string) (adminapp.ListResponse[configadmin.Project], error) {
	if err := s.Authorize(actor, "read", "project"); err != nil {
		return adminapp.ListResponse[configadmin.Project]{}, err
	}
	projects, err := s.owner.ListProjects(ctx, tenantID)
	return adminapp.ListResponse[configadmin.Project]{Data: projects}, err
}

// UpsertProject writes project configuration through the control-plane owner service.
func (s *Service) UpsertProject(ctx context.Context, actor adminapp.Actor, request configadmin.Project, opts adminapp.MutationOptions) (*configadmin.Project, error) {
	return mutate(ctx, s, actor, opts, "write", "project", request.ID, request, func() (*configadmin.Project, error) {
		return s.owner.UpsertProject(ctx, request)
	})
}

// ListAPIKeys returns safe API key read models without hashes or plaintext.
func (s *Service) ListAPIKeys(ctx context.Context, actor adminapp.Actor, tenantID string, projectID string) (adminapp.ListResponse[adminapp.APIKeyView], error) {
	if err := s.Authorize(actor, "read", "api_key"); err != nil {
		return adminapp.ListResponse[adminapp.APIKeyView]{}, err
	}
	keys, err := s.owner.ListAPIKeys(ctx, tenantID, projectID)
	if err != nil {
		return adminapp.ListResponse[adminapp.APIKeyView]{}, err
	}
	views := make([]adminapp.APIKeyView, 0, len(keys))
	for _, key := range keys {
		views = append(views, safeAPIKey(key))
	}
	return adminapp.ListResponse[adminapp.APIKeyView]{Data: views}, nil
}

// CreateAPIKey creates an API key through the control-plane owner service and audits the mutation.
func (s *Service) CreateAPIKey(ctx context.Context, actor adminapp.Actor, request configadmin.APIKey, opts adminapp.MutationOptions) (*configadmin.APIKey, error) {
	return mutate(ctx, s, actor, opts, "write", "api_key", request.ID, request, func() (*configadmin.APIKey, error) {
		return s.owner.CreateAPIKey(ctx, request)
	})
}

// DisableAPIKey disables an API key through the control-plane owner service.
func (s *Service) DisableAPIKey(ctx context.Context, actor adminapp.Actor, keyID string, opts adminapp.MutationOptions) (*configadmin.APIKey, error) {
	return mutate(ctx, s, actor, opts, "write", "api_key", keyID, map[string]string{"id": keyID}, func() (*configadmin.APIKey, error) {
		return s.owner.DisableAPIKey(ctx, keyID)
	})
}

// ListModels returns public model configuration read models.
func (s *Service) ListModels(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[configadmin.ModelConfig], error) {
	if err := s.Authorize(actor, "read", "model"); err != nil {
		return adminapp.ListResponse[configadmin.ModelConfig]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	return adminapp.ListResponse[configadmin.ModelConfig]{Data: cfg.Models}, err
}

// UpsertModel writes model configuration through the control-plane owner service.
func (s *Service) UpsertModel(ctx context.Context, actor adminapp.Actor, request configadmin.ModelConfig, opts adminapp.MutationOptions) (*configadmin.ModelConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "model", request.PublicModel, request, func() (*configadmin.ModelConfig, error) {
		return s.owner.UpsertModel(ctx, request)
	})
}

// ListChannels returns safe channel read models without credential material.
func (s *Service) ListChannels(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[adminapp.ChannelView], error) {
	if err := s.Authorize(actor, "read", "channel"); err != nil {
		return adminapp.ListResponse[adminapp.ChannelView]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	if err != nil {
		return adminapp.ListResponse[adminapp.ChannelView]{}, err
	}
	views := make([]adminapp.ChannelView, 0, len(cfg.Channels))
	for _, channel := range cfg.Channels {
		views = append(views, safeChannel(channel))
	}
	return adminapp.ListResponse[adminapp.ChannelView]{Data: views}, nil
}

// UpsertChannel writes channel configuration through the control-plane owner service.
func (s *Service) UpsertChannel(ctx context.Context, actor adminapp.Actor, request configadmin.ChannelConfig, opts adminapp.MutationOptions) (*configadmin.ChannelConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "channel", request.ID, request, func() (*configadmin.ChannelConfig, error) {
		return s.owner.UpsertChannel(ctx, request)
	})
}

// SetChannelEnabled updates a channel enabled flag through the control-plane owner service.
func (s *Service) SetChannelEnabled(ctx context.Context, actor adminapp.Actor, channelID string, enabled bool, opts adminapp.MutationOptions) (*configadmin.ChannelConfig, error) {
	request := map[string]any{"id": channelID, "enabled": enabled}
	return mutate(ctx, s, actor, opts, "write", "channel", channelID, request, func() (*configadmin.ChannelConfig, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return nil, err
		}
		for _, channel := range cfg.Channels {
			if channel.ID == channelID {
				channel.Enabled = enabled
				channel.EnabledSet = true
				return s.owner.UpsertChannel(ctx, channel)
			}
		}
		return nil, apperr.NotFound("channel not found")
	})
}

// ListRoutes returns route policy read models.
func (s *Service) ListRoutes(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[configadmin.RoutePolicyConfig], error) {
	if err := s.Authorize(actor, "read", "route"); err != nil {
		return adminapp.ListResponse[configadmin.RoutePolicyConfig]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	return adminapp.ListResponse[configadmin.RoutePolicyConfig]{Data: cfg.Routes}, err
}

// UpsertRoute writes route configuration through the control-plane owner service.
func (s *Service) UpsertRoute(ctx context.Context, actor adminapp.Actor, request configadmin.RoutePolicyConfig, opts adminapp.MutationOptions) (*configadmin.RoutePolicyConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "route", request.ID, request, func() (*configadmin.RoutePolicyConfig, error) {
		return s.owner.UpsertRoute(ctx, request)
	})
}

// ListPricing returns price rule read models.
func (s *Service) ListPricing(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[configadmin.PriceRuleConfig], error) {
	if err := s.Authorize(actor, "read", "pricing"); err != nil {
		return adminapp.ListResponse[configadmin.PriceRuleConfig]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	return adminapp.ListResponse[configadmin.PriceRuleConfig]{Data: cfg.Prices}, err
}

// UpsertPrice writes price configuration through the control-plane owner service.
func (s *Service) UpsertPrice(ctx context.Context, actor adminapp.Actor, request configadmin.PriceRuleConfig, opts adminapp.MutationOptions) (*configadmin.PriceRuleConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "pricing", request.PublicModel, request, func() (*configadmin.PriceRuleConfig, error) {
		return s.owner.UpsertPrice(ctx, request)
	})
}

// ListLimits returns limit rule read models.
func (s *Service) ListLimits(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[configadmin.LimitRuleConfig], error) {
	if err := s.Authorize(actor, "read", "limit"); err != nil {
		return adminapp.ListResponse[configadmin.LimitRuleConfig]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	return adminapp.ListResponse[configadmin.LimitRuleConfig]{Data: cfg.Limits}, err
}

// UpsertLimit writes limit configuration through the control-plane owner service.
func (s *Service) UpsertLimit(ctx context.Context, actor adminapp.Actor, request configadmin.LimitRuleConfig, opts adminapp.MutationOptions) (*configadmin.LimitRuleConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "limit", request.ID, request, func() (*configadmin.LimitRuleConfig, error) {
		return s.owner.UpsertLimit(ctx, request)
	})
}

func safeAPIKey(key configadmin.APIKey) adminapp.APIKeyView {
	return adminapp.APIKeyView{
		ID:            key.ID,
		TenantID:      key.TenantID,
		ProjectID:     key.ProjectID,
		Name:          key.Name,
		Enabled:       key.Enabled,
		AllowedModels: append([]string(nil), key.AllowedModels...),
		RevokedAt:     key.RevokedAt,
		CreatedAt:     key.CreatedAt,
		UpdatedAt:     key.UpdatedAt,
	}
}

func safeChannel(channel configadmin.ChannelConfig) adminapp.ChannelView {
	view := adminapp.ChannelView{
		ID:                   channel.ID,
		ProviderType:         channel.ProviderType,
		BaseURL:              channel.BaseURL,
		CredentialConfigured: channel.CredentialRef != "" || channel.EncryptedAPIKey != "",
		Enabled:              channel.Enabled,
		TimeoutMillis:        channel.TimeoutMillis,
	}
	if view.TimeoutMillis == 0 && channel.Timeout > 0 {
		view.TimeoutMillis = channel.Timeout.Milliseconds()
	}
	for _, model := range channel.Models {
		view.Models = append(view.Models, adminapp.ChannelModelView{
			PublicModel:         model.PublicModel,
			UpstreamModel:       model.UpstreamModel,
			Capabilities:        append([]string(nil), model.Capabilities...),
			SupportedParameters: append([]string(nil), model.SupportedParameters...),
			HealthStatus:        model.HealthStatus,
			TestStatus:          model.TestStatus,
			CostConfigStatus:    model.CostConfigStatus,
			Metadata:            append([]byte(nil), model.Metadata...),
		})
	}
	return view
}
