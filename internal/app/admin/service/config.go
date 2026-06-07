package service

import (
	"context"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
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
		view, err := s.apiKeyView(ctx, key)
		if err != nil {
			return adminapp.ListResponse[adminapp.APIKeyView]{}, err
		}
		views = append(views, view)
	}
	return adminapp.ListResponse[adminapp.APIKeyView]{Data: views}, nil
}

// CreateAPIKey creates an API key through the control-plane owner service and audits the mutation.
func (s *Service) CreateAPIKey(ctx context.Context, actor adminapp.Actor, request adminapp.APIKeyCreateRequest, opts adminapp.MutationOptions) (adminapp.APIKeyCreateResponse, error) {
	return mutate(ctx, s, actor, opts, "write", "api_key", request.ProjectID, request, func() (adminapp.APIKeyCreateResponse, error) {
		key, err := s.owner.CreateAPIKey(ctx, configadmin.APIKey{
			TenantID:      strings.TrimSpace(request.TenantID),
			ProjectID:     strings.TrimSpace(request.ProjectID),
			Name:          strings.TrimSpace(request.Name),
			AllowedModels: cleanCustomerStrings(request.AllowedModels),
			IPAllowlist:   cleanCustomerStrings(request.IPAllowlist),
			ExpiresAt:     request.ExpiresAt,
		})
		if err != nil {
			return adminapp.APIKeyCreateResponse{}, err
		}
		view, err := s.apiKeyView(ctx, *key)
		if err != nil {
			return adminapp.APIKeyCreateResponse{}, err
		}
		return adminapp.APIKeyCreateResponse{APIKey: view, PlaintextKey: key.PlaintextKey}, nil
	})
}

// UpdateAPIKey updates safe API key metadata through the owner service.
func (s *Service) UpdateAPIKey(ctx context.Context, actor adminapp.Actor, keyID string, request adminapp.APIKeyUpdateRequest, opts adminapp.MutationOptions) (adminapp.APIKeyView, error) {
	return mutate(ctx, s, actor, opts, "write", "api_key", keyID, request, func() (adminapp.APIKeyView, error) {
		key, err := s.owner.UpdateAPIKey(ctx, configadmin.APIKey{
			ID:            strings.TrimSpace(keyID),
			Name:          strings.TrimSpace(request.Name),
			AllowedModels: cleanCustomerStrings(request.AllowedModels),
			IPAllowlist:   cleanCustomerStrings(request.IPAllowlist),
			ExpiresAt:     request.ExpiresAt,
		})
		if err != nil {
			return adminapp.APIKeyView{}, err
		}
		return s.apiKeyView(ctx, *key)
	})
}

// EnableAPIKey enables an API key through the control-plane owner service.
func (s *Service) EnableAPIKey(ctx context.Context, actor adminapp.Actor, keyID string, opts adminapp.MutationOptions) (adminapp.APIKeyView, error) {
	return mutate(ctx, s, actor, opts, "write", "api_key", keyID, map[string]string{"id": keyID}, func() (adminapp.APIKeyView, error) {
		key, err := s.owner.EnableAPIKey(ctx, keyID)
		if err != nil {
			return adminapp.APIKeyView{}, err
		}
		return s.apiKeyView(ctx, *key)
	})
}

// DisableAPIKey disables an API key through the control-plane owner service.
func (s *Service) DisableAPIKey(ctx context.Context, actor adminapp.Actor, keyID string, opts adminapp.MutationOptions) (adminapp.APIKeyView, error) {
	return mutate(ctx, s, actor, opts, "write", "api_key", keyID, map[string]string{"id": keyID}, func() (adminapp.APIKeyView, error) {
		key, err := s.owner.DisableAPIKey(ctx, keyID)
		if err != nil {
			return adminapp.APIKeyView{}, err
		}
		return s.apiKeyView(ctx, *key)
	})
}

// RotateAPIKey rotates an API key and returns the new plaintext once.
func (s *Service) RotateAPIKey(ctx context.Context, actor adminapp.Actor, keyID string, request adminapp.APIKeyRotateRequest, opts adminapp.MutationOptions) (adminapp.APIKeyRotateResponse, error) {
	return mutate(ctx, s, actor, opts, "write", "api_key", keyID, request, func() (adminapp.APIKeyRotateResponse, error) {
		key, err := s.owner.RotateAPIKey(ctx, keyID, request.PlaintextKey)
		if err != nil {
			return adminapp.APIKeyRotateResponse{}, err
		}
		view, err := s.apiKeyView(ctx, *key)
		if err != nil {
			return adminapp.APIKeyRotateResponse{}, err
		}
		return adminapp.APIKeyRotateResponse{APIKey: view, PlaintextKey: key.PlaintextKey}, nil
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
		views = append(views, safeChannel(channel, cfg.Routes))
	}
	return adminapp.ListResponse[adminapp.ChannelView]{Data: views}, nil
}

// GetChannel returns one safe channel read model without credential material.
func (s *Service) GetChannel(ctx context.Context, actor adminapp.Actor, channelID string) (adminapp.ChannelView, error) {
	if err := s.Authorize(actor, "read", "channel"); err != nil {
		return adminapp.ChannelView{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	if err != nil {
		return adminapp.ChannelView{}, err
	}
	channel, ok := findChannel(cfg.Channels, channelID)
	if !ok {
		return adminapp.ChannelView{}, apperr.NotFound("channel not found")
	}
	return safeChannel(channel, cfg.Routes), nil
}

// UpsertChannel writes channel configuration through the control-plane owner service.
func (s *Service) UpsertChannel(ctx context.Context, actor adminapp.Actor, request configadmin.ChannelConfig, opts adminapp.MutationOptions) (adminapp.ChannelView, error) {
	return mutate(ctx, s, actor, opts, "write", "channel", request.ID, request, func() (adminapp.ChannelView, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ChannelView{}, err
		}
		updated, err := s.owner.UpsertChannel(ctx, request)
		if err != nil {
			return adminapp.ChannelView{}, err
		}
		return safeChannel(*updated, cfg.Routes), nil
	})
}

// PatchChannel updates selected channel fields while preserving credential material.
func (s *Service) PatchChannel(ctx context.Context, actor adminapp.Actor, channelID string, request configadmin.ChannelConfig, opts adminapp.MutationOptions) (adminapp.ChannelView, error) {
	request.ID = channelID
	return mutate(ctx, s, actor, opts, "write", "channel", channelID, request, func() (adminapp.ChannelView, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ChannelView{}, err
		}
		current, ok := findChannel(cfg.Channels, channelID)
		if !ok {
			return adminapp.ChannelView{}, apperr.NotFound("channel not found")
		}

		merged := mergeChannel(current, request)
		updated, err := s.owner.UpsertChannel(ctx, merged)
		if err != nil {
			return adminapp.ChannelView{}, err
		}
		return safeChannel(*updated, cfg.Routes), nil
	})
}

// SetChannelEnabled updates a channel enabled flag through the control-plane owner service.
func (s *Service) SetChannelEnabled(ctx context.Context, actor adminapp.Actor, channelID string, enabled bool, opts adminapp.MutationOptions) (adminapp.ChannelView, error) {
	request := map[string]any{"id": channelID, "enabled": enabled}
	return mutate(ctx, s, actor, opts, "write", "channel", channelID, request, func() (adminapp.ChannelView, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ChannelView{}, err
		}
		for _, channel := range cfg.Channels {
			if channel.ID == channelID {
				channel.Enabled = enabled
				channel.EnabledSet = true
				updated, err := s.owner.UpsertChannel(ctx, channel)
				if err != nil {
					return adminapp.ChannelView{}, err
				}
				return safeChannel(*updated, cfg.Routes), nil
			}
		}
		return adminapp.ChannelView{}, apperr.NotFound("channel not found")
	})
}

// RotateChannelCredential rotates provider credential material without returning it.
func (s *Service) RotateChannelCredential(ctx context.Context, actor adminapp.Actor, channelID string, apiKey string, opts adminapp.MutationOptions) (adminapp.ChannelView, error) {
	request := map[string]string{"id": channelID, "api_key": apiKey}
	return mutate(ctx, s, actor, opts, "write", "channel", channelID, request, func() (adminapp.ChannelView, error) {
		if strings.TrimSpace(apiKey) == "" {
			return adminapp.ChannelView{}, apperr.InvalidArgument("api_key is required")
		}
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ChannelView{}, err
		}
		current, ok := findChannel(cfg.Channels, channelID)
		if !ok {
			return adminapp.ChannelView{}, apperr.NotFound("channel not found")
		}
		current.APIKey = apiKey
		current.EnabledSet = true
		updated, err := s.owner.UpsertChannel(ctx, current)
		if err != nil {
			return adminapp.ChannelView{}, err
		}
		return safeChannel(*updated, cfg.Routes), nil
	})
}

// TestChannel returns a safe readiness check for channel configuration.
func (s *Service) TestChannel(ctx context.Context, actor adminapp.Actor, channelID string, opts adminapp.MutationOptions) (adminapp.ChannelTestResult, error) {
	request := map[string]string{"id": channelID}
	return mutate(ctx, s, actor, opts, "write", "channel", channelID, request, func() (adminapp.ChannelTestResult, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ChannelTestResult{}, err
		}
		channel, ok := findChannel(cfg.Channels, channelID)
		if !ok {
			return adminapp.ChannelTestResult{}, apperr.NotFound("channel not found")
		}
		return channelReadiness(channel, s.now()), nil
	})
}

// PreviewChannelModelSync returns a non-persistent upstream model diff for one channel.
func (s *Service) PreviewChannelModelSync(ctx context.Context, actor adminapp.Actor, request configadmin.ChannelModelSyncPreviewRequest, opts adminapp.MutationOptions) (*configadmin.ChannelModelSyncPreview, error) {
	return mutate(ctx, s, actor, opts, "write", "channel", request.ChannelID, request, func() (*configadmin.ChannelModelSyncPreview, error) {
		return s.owner.PreviewChannelModelSync(ctx, request)
	})
}

// ApplyChannelModelSync persists upstream model sync results for one channel.
func (s *Service) ApplyChannelModelSync(ctx context.Context, actor adminapp.Actor, request configadmin.ChannelModelSyncPreviewRequest, opts adminapp.MutationOptions) (adminapp.ChannelSyncApplyResult, error) {
	return mutate(ctx, s, actor, opts, "write", "channel", request.ChannelID, request, func() (adminapp.ChannelSyncApplyResult, error) {
		preview, err := s.owner.PreviewChannelModelSync(ctx, request)
		if err != nil {
			return adminapp.ChannelSyncApplyResult{}, err
		}
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ChannelSyncApplyResult{}, err
		}
		current, ok := findChannel(cfg.Channels, request.ChannelID)
		if !ok {
			return adminapp.ChannelSyncApplyResult{}, apperr.NotFound("channel not found")
		}
		current.Models = request.UpstreamModels
		current.EnabledSet = true
		updated, err := s.owner.UpsertChannel(ctx, current)
		if err != nil {
			return adminapp.ChannelSyncApplyResult{}, err
		}
		return adminapp.ChannelSyncApplyResult{
			ChannelID: updated.ID,
			AppliedAt: s.now(),
			Preview:   preview,
			Channel:   safeChannel(*updated, cfg.Routes),
		}, nil
	})
}

// ListChannelHealthEvents returns safe synthetic health events for one channel.
func (s *Service) ListChannelHealthEvents(ctx context.Context, actor adminapp.Actor, channelID string) (adminapp.ListResponse[adminapp.ChannelHealthEvent], error) {
	if err := s.Authorize(actor, "read", "channel"); err != nil {
		return adminapp.ListResponse[adminapp.ChannelHealthEvent]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	if err != nil {
		return adminapp.ListResponse[adminapp.ChannelHealthEvent]{}, err
	}
	channel, ok := findChannel(cfg.Channels, channelID)
	if !ok {
		return adminapp.ListResponse[adminapp.ChannelHealthEvent]{}, apperr.NotFound("channel not found")
	}
	return adminapp.ListResponse[adminapp.ChannelHealthEvent]{
		Data: channelHealthEvents(channel, s.now()),
	}, nil
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

func (s *Service) apiKeyView(ctx context.Context, key configadmin.APIKey) (adminapp.APIKeyView, error) {
	view := safeAPIKey(key)
	if s.commercial == nil {
		return view, nil
	}
	report, err := s.commercial.TenantUsageReport(ctx, reporting.TenantUsageFilter{
		TenantID:  key.TenantID,
		ProjectID: key.ProjectID,
		APIKeyID:  key.ID,
		Limit:     1,
	})
	if err != nil {
		return adminapp.APIKeyView{}, err
	}
	view.UsageSummary = customerUsageSummary(report)
	return view, nil
}

func safeAPIKey(key configadmin.APIKey) adminapp.APIKeyView {
	return adminapp.APIKeyView{
		ID:            key.ID,
		TenantID:      key.TenantID,
		ProjectID:     key.ProjectID,
		Name:          key.Name,
		Fingerprint:   apiKeyFingerprint(key),
		Enabled:       key.Enabled,
		AllowedModels: append([]string(nil), key.AllowedModels...),
		IPAllowlist:   append([]string(nil), key.IPAllowlist...),
		ExpiresAt:     cloneTimePtr(key.ExpiresAt),
		LastUsedAt:    cloneTimePtr(key.LastUsedAt),
		RevokedAt:     key.RevokedAt,
		CreatedAt:     key.CreatedAt,
		UpdatedAt:     key.UpdatedAt,
	}
}

func apiKeyFingerprint(key configadmin.APIKey) string {
	if len(key.KeyHash) >= 12 {
		return key.KeyHash[len(key.KeyHash)-12:]
	}
	if len(key.ID) >= 8 {
		return key.ID[len(key.ID)-8:]
	}
	return key.ID
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func safeChannel(channel configadmin.ChannelConfig, routes []configadmin.RoutePolicyConfig) adminapp.ChannelView {
	view := adminapp.ChannelView{
		ID:                   channel.ID,
		ProviderType:         channel.ProviderType,
		BaseURL:              channel.BaseURL,
		CredentialConfigured: credentialConfigured(channel),
		Enabled:              channel.Enabled,
		TimeoutMillis:        channel.TimeoutMillis,
		ModelCount:           len(channel.Models),
		HealthStatus:         aggregateHealthStatus(channel),
		TestStatus:           aggregateTestStatus(channel),
		CostConfigStatus:     aggregateCostConfigStatus(channel),
		RoutePolicyHints:     routePolicyHints(channel.ID, routes),
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

func findChannel(channels []configadmin.ChannelConfig, channelID string) (configadmin.ChannelConfig, bool) {
	channelID = strings.TrimSpace(channelID)
	for _, channel := range channels {
		if channel.ID == channelID {
			return channel, true
		}
	}
	return configadmin.ChannelConfig{}, false
}

func mergeChannel(current configadmin.ChannelConfig, patch configadmin.ChannelConfig) configadmin.ChannelConfig {
	if strings.TrimSpace(patch.ProviderType) != "" {
		current.ProviderType = patch.ProviderType
	}
	if strings.TrimSpace(patch.BaseURL) != "" {
		current.BaseURL = patch.BaseURL
	}
	if strings.TrimSpace(patch.CredentialRef) != "" {
		current.CredentialRef = patch.CredentialRef
	}
	if strings.TrimSpace(patch.APIKey) != "" {
		current.APIKey = patch.APIKey
	}
	if patch.TimeoutMillis > 0 {
		current.TimeoutMillis = patch.TimeoutMillis
	}
	if patch.Timeout > 0 {
		current.Timeout = patch.Timeout
	}
	if patch.Models != nil {
		current.Models = patch.Models
	}
	if patch.EnabledSet {
		current.Enabled = patch.Enabled
	}
	current.EnabledSet = true
	return current
}

func credentialConfigured(channel configadmin.ChannelConfig) bool {
	return strings.TrimSpace(channel.APIKey) != "" ||
		strings.TrimSpace(channel.CredentialRef) != "" ||
		strings.TrimSpace(channel.EncryptedAPIKey) != ""
}

func aggregateHealthStatus(channel configadmin.ChannelConfig) string {
	if !channel.Enabled {
		return "disabled"
	}
	if len(channel.Models) == 0 {
		return "unmapped"
	}
	healthy := false
	for _, model := range channel.Models {
		status := strings.ToLower(strings.TrimSpace(model.HealthStatus))
		switch status {
		case "failed", "error", "unhealthy", "degraded":
			return "degraded"
		case "healthy", "ok", "passed":
			healthy = true
		}
	}
	if healthy {
		return "healthy"
	}
	return "unknown"
}

func aggregateTestStatus(channel configadmin.ChannelConfig) string {
	if len(channel.Models) == 0 {
		return "untested"
	}
	tested := false
	for _, model := range channel.Models {
		status := strings.ToLower(strings.TrimSpace(model.TestStatus))
		switch status {
		case "failed", "error":
			return "failed"
		case "passed", "success", "ok":
			tested = true
		}
	}
	if tested {
		return "passed"
	}
	return "untested"
}

func aggregateCostConfigStatus(channel configadmin.ChannelConfig) string {
	if len(channel.Models) == 0 {
		return "unknown"
	}
	for _, model := range channel.Models {
		if strings.ToLower(strings.TrimSpace(model.CostConfigStatus)) != "configured" {
			return "incomplete"
		}
	}
	return "configured"
}

func routePolicyHints(channelID string, routes []configadmin.RoutePolicyConfig) []adminapp.RoutePolicyHint {
	var hints []adminapp.RoutePolicyHint
	for _, route := range routes {
		for _, candidate := range route.Candidates {
			if candidate.ChannelID != channelID {
				continue
			}
			hints = append(hints, adminapp.RoutePolicyHint{
				RouteID:     route.ID,
				PublicModel: route.PublicModel,
				Strategy:    route.Strategy,
				Enabled:     route.Enabled,
				Priority:    candidate.Priority,
				Weight:      candidate.Weight,
			})
		}
	}
	return hints
}

func channelHealthEvents(channel configadmin.ChannelConfig, observedAt time.Time) []adminapp.ChannelHealthEvent {
	events := []adminapp.ChannelHealthEvent{{
		ID:         "health_" + channel.ID + "_summary",
		ChannelID:  channel.ID,
		Status:     aggregateHealthStatus(channel),
		Source:     "channel_config",
		Message:    "channel health summary from safe admin read model",
		ObservedAt: observedAt,
	}}
	for _, model := range channel.Models {
		events = append(events, adminapp.ChannelHealthEvent{
			ID:         "health_" + channel.ID + "_" + model.PublicModel,
			ChannelID:  channel.ID,
			Status:     strings.TrimSpace(model.HealthStatus),
			Source:     "channel_model",
			Message:    "model " + model.PublicModel + " maps to upstream " + model.UpstreamModel,
			ObservedAt: observedAt,
		})
	}
	return events
}
