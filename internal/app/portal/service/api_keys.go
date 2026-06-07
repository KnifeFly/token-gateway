package service

import (
	"context"
	"strings"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListAPIKeys returns safe API key metadata.
func (s *Service) ListAPIKeys(ctx context.Context, principal portalapp.Principal) (portalapp.APIKeyListResponse, error) {
	if s == nil || s.admin == nil {
		return portalapp.APIKeyListResponse{}, apperr.ConfigUnavailable("admin service is unavailable")
	}
	keys, err := s.admin.ListAPIKeys(ctx, principal.TenantID, principal.ProjectID)
	if err != nil {
		return portalapp.APIKeyListResponse{}, err
	}
	out := make([]portalapp.APIKey, 0, len(keys))
	for _, key := range keys {
		if key.TenantID != principal.TenantID || key.ProjectID != principal.ProjectID {
			continue
		}
		view, err := s.apiKeyView(ctx, key)
		if err != nil {
			return portalapp.APIKeyListResponse{}, err
		}
		out = append(out, view)
	}
	return portalapp.APIKeyListResponse{Data: out}, nil
}

// CreateAPIKey creates a derived API key.
func (s *Service) CreateAPIKey(ctx context.Context, principal portalapp.Principal, request portalapp.APIKeyCreateRequest) (portalapp.APIKeyCreateResponse, error) {
	if s == nil || s.admin == nil {
		return portalapp.APIKeyCreateResponse{}, apperr.ConfigUnavailable("admin service is unavailable")
	}
	allowedModels := cleanStrings(request.AllowedModels)
	if len(allowedModels) == 0 {
		allowedModels = cleanStrings(principal.AllowedModels)
	}
	if !allowedModelsSubset(principal.AllowedModels, allowedModels) {
		return portalapp.APIKeyCreateResponse{}, apperr.Forbidden("allowed_models cannot exceed current api key permissions")
	}
	key, err := s.admin.CreateAPIKey(ctx, configadmin.APIKey{
		TenantID:      principal.TenantID,
		ProjectID:     principal.ProjectID,
		Name:          strings.TrimSpace(request.Name),
		AllowedModels: allowedModels,
		IPAllowlist:   cleanStrings(request.IPAllowlist),
		ExpiresAt:     request.ExpiresAt,
	})
	if err != nil {
		return portalapp.APIKeyCreateResponse{}, err
	}
	if err := s.refreshSnapshot(ctx); err != nil {
		return portalapp.APIKeyCreateResponse{}, err
	}
	view, err := s.apiKeyView(ctx, *key)
	if err != nil {
		return portalapp.APIKeyCreateResponse{}, err
	}
	return portalapp.APIKeyCreateResponse{APIKey: view, PlaintextKey: key.PlaintextKey}, nil
}

// DisableAPIKey disables a derived API key.
func (s *Service) DisableAPIKey(ctx context.Context, principal portalapp.Principal, keyID string) (portalapp.APIKey, error) {
	if s == nil || s.admin == nil {
		return portalapp.APIKey{}, apperr.ConfigUnavailable("admin service is unavailable")
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return portalapp.APIKey{}, apperr.InvalidArgument("api key id is required")
	}
	if keyID == principal.APIKeyID {
		return portalapp.APIKey{}, apperr.Forbidden("current api key cannot disable itself")
	}
	if _, err := s.findPrincipalAPIKey(ctx, principal, keyID); err != nil {
		return portalapp.APIKey{}, apperr.NotFound("api key not found")
	}
	disabled, err := s.admin.DisableAPIKey(ctx, keyID)
	if err != nil {
		return portalapp.APIKey{}, err
	}
	if err := s.refreshSnapshot(ctx); err != nil {
		return portalapp.APIKey{}, err
	}
	return s.apiKeyView(ctx, *disabled)
}

// RotateAPIKey rotates a derived API key and returns plaintext once.
func (s *Service) RotateAPIKey(ctx context.Context, principal portalapp.Principal, keyID string) (portalapp.APIKeyRotateResponse, error) {
	if s == nil || s.admin == nil {
		return portalapp.APIKeyRotateResponse{}, apperr.ConfigUnavailable("admin service is unavailable")
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return portalapp.APIKeyRotateResponse{}, apperr.InvalidArgument("api key id is required")
	}
	if _, err := s.findPrincipalAPIKey(ctx, principal, keyID); err != nil {
		return portalapp.APIKeyRotateResponse{}, apperr.NotFound("api key not found")
	}
	rotated, err := s.admin.RotateAPIKey(ctx, keyID, "")
	if err != nil {
		return portalapp.APIKeyRotateResponse{}, err
	}
	if err := s.refreshSnapshot(ctx); err != nil {
		return portalapp.APIKeyRotateResponse{}, err
	}
	view, err := s.apiKeyView(ctx, *rotated)
	if err != nil {
		return portalapp.APIKeyRotateResponse{}, err
	}
	return portalapp.APIKeyRotateResponse{APIKey: view, PlaintextKey: rotated.PlaintextKey}, nil
}

func (s *Service) findPrincipalAPIKey(ctx context.Context, principal portalapp.Principal, keyID string) (*configadmin.APIKey, error) {
	keys, err := s.admin.ListAPIKeys(ctx, principal.TenantID, principal.ProjectID)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if key.ID == keyID && key.TenantID == principal.TenantID && key.ProjectID == principal.ProjectID {
			return &key, nil
		}
	}
	return nil, apperr.NotFound("api key not found")
}

func (s *Service) apiKeyView(ctx context.Context, key configadmin.APIKey) (portalapp.APIKey, error) {
	view := safeAPIKey(key)
	if s == nil || s.reporting == nil {
		return view, nil
	}
	report, err := s.reporting.TenantUsageReport(ctx, reporting.TenantUsageFilter{
		TenantID:  key.TenantID,
		ProjectID: key.ProjectID,
		APIKeyID:  key.ID,
		Limit:     1,
	})
	if err != nil {
		return portalapp.APIKey{}, err
	}
	view.UsageSummary = portalapp.UsageTotals{
		Requests:     report.Totals.Requests,
		InputTokens:  report.Totals.InputTokens,
		OutputTokens: report.Totals.OutputTokens,
		CreditsUsed:  microsToCredits(report.Totals.RevenueMicros),
	}
	return view, nil
}

func safeAPIKey(key configadmin.APIKey) portalapp.APIKey {
	return portalapp.APIKey{
		ID:            key.ID,
		Name:          key.Name,
		Enabled:       key.Enabled,
		AllowedModels: append([]string(nil), key.AllowedModels...),
		IPAllowlist:   append([]string(nil), key.IPAllowlist...),
		ExpiresAt:     cloneTimePtr(key.ExpiresAt),
		LastUsedAt:    cloneTimePtr(key.LastUsedAt),
		CreatedAt:     key.CreatedAt,
		RevokedAt:     key.RevokedAt,
	}
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func allowedModelsSubset(parent []string, child []string) bool {
	if modelAllowed(parent, "*") {
		return true
	}
	for _, model := range child {
		if !modelAllowed(parent, model) {
			return false
		}
	}
	return true
}

func modelAllowed(allowed []string, model string) bool {
	model = strings.TrimSpace(model)
	for _, value := range allowed {
		value = strings.TrimSpace(value)
		if value == "*" || value == model {
			return true
		}
	}
	return false
}

func modelAllowedForView(allowed []string, requested string, model engine.ModelView) bool {
	if modelAllowed(allowed, model.PublicModel) || modelAllowed(allowed, requested) {
		return true
	}
	for _, alias := range model.Aliases {
		if modelAllowed(allowed, alias) {
			return true
		}
	}
	return false
}

func cleanStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
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

func (s *Service) refreshSnapshot(ctx context.Context) error {
	if s == nil || s.snapshots == nil {
		return nil
	}
	return s.snapshots.RefreshSnapshot(ctx)
}
