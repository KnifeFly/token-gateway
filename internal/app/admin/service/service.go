// Package service coordinates browser-facing Admin Web BFF use cases.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	cpadmin "github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const (
	defaultSessionTTL = 12 * time.Hour
	sessionIDBytes    = 32
	csrfTokenBytes    = 32
	auditStatusOK     = "success"
	auditStatusFailed = "failed"
)

// SnapshotManager publishes, rolls back, and diagnoses runtime snapshots through the owner service.
type SnapshotManager interface {
	Publish(ctx context.Context) (*cpsnapshot.RuntimeSnapshot, error)
	Rollback(ctx context.Context) (*cpsnapshot.RuntimeSnapshot, error)
	Diagnostics(ctx context.Context) (*cpsnapshot.Diagnostics, error)
}

// Service coordinates Admin Web BFF authorization, audit, and owner-service workflows.
type Service struct {
	repo              adminapp.Repository
	owner             *cpadmin.Service
	commercial        *reporting.Service
	tasks             tasksvc.Repository
	failedSettlements *billing.FailedSettlementService
	snapshots         SnapshotManager
	now               func() time.Time
	ttl               time.Duration
}

// Option customizes the Admin Web service.
type Option func(*Service)

// WithClock configures a deterministic clock for tests.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// WithSessionTTL configures browser session lifetime.
func WithSessionTTL(ttl time.Duration) Option {
	return func(s *Service) {
		if ttl > 0 {
			s.ttl = ttl
		}
	}
}

// WithCommercialReporting attaches commercial reporting read models.
func WithCommercialReporting(commercial *reporting.Service) Option {
	return func(s *Service) {
		s.commercial = commercial
	}
}

// WithTaskRepository attaches task and callback read models.
func WithTaskRepository(tasks tasksvc.Repository) Option {
	return func(s *Service) {
		s.tasks = tasks
	}
}

// WithFailedSettlementService attaches the owner repair workflow.
func WithFailedSettlementService(failedSettlements *billing.FailedSettlementService) Option {
	return func(s *Service) {
		s.failedSettlements = failedSettlements
	}
}

// WithSnapshotManager attaches the owner snapshot workflow.
func WithSnapshotManager(snapshots SnapshotManager) Option {
	return func(s *Service) {
		s.snapshots = snapshots
	}
}

// New returns an Admin Web BFF application service.
func New(repo adminapp.Repository, owner *cpadmin.Service, opts ...Option) *Service {
	s := &Service{
		repo:  repo,
		owner: owner,
		now:   func() time.Time { return time.Now().UTC() },
		ttl:   defaultSessionTTL,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// EnsureBootstrapOperator creates a bootstrap operator when the repository has no matching email.
func (s *Service) EnsureBootstrapOperator(ctx context.Context, email string, password string, roles []adminapp.Role) (adminapp.Operator, error) {
	if s == nil || s.repo == nil {
		return adminapp.Operator{}, apperr.ConfigUnavailable("admin web repository is unavailable")
	}
	email = normalizeEmail(email)
	if email == "" || strings.TrimSpace(password) == "" {
		return adminapp.Operator{}, apperr.InvalidArgument("bootstrap email and password are required")
	}
	if existing, ok, err := s.repo.GetOperatorByEmail(ctx, email); err != nil || ok {
		return existing, err
	}
	if len(roles) == 0 {
		roles = []adminapp.Role{adminapp.RoleSuperAdmin}
	}
	now := s.now()
	operator := adminapp.Operator{
		ID:           newID("operator"),
		Email:        email,
		DisplayName:  "Local Admin",
		PasswordHash: HashPassword(password),
		Roles:        normalizeRoles(roles),
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return s.repo.SaveOperator(ctx, operator)
}

// Login authenticates an operator and creates a browser session.
func (s *Service) Login(ctx context.Context, request adminapp.LoginRequest, userAgent string, remoteAddr string) (adminapp.LoginResponse, error) {
	if s == nil || s.repo == nil {
		return adminapp.LoginResponse{}, apperr.ConfigUnavailable("admin web service is unavailable")
	}
	operator, ok, err := s.repo.GetOperatorByEmail(ctx, normalizeEmail(request.Email))
	if err != nil {
		return adminapp.LoginResponse{}, err
	}
	if !ok || !operator.Enabled || !verifyPassword(operator.PasswordHash, request.Password) {
		return adminapp.LoginResponse{}, apperr.Unauthorized("invalid operator credentials")
	}
	now := s.now()
	csrfToken := newToken(csrfTokenBytes)
	session := adminapp.Session{
		ID:            newToken(sessionIDBytes),
		OperatorID:    operator.ID,
		CSRFHash:      hashToken(csrfToken),
		UserAgentHash: hashText(userAgent),
		RemoteAddr:    strings.TrimSpace(remoteAddr),
		CreatedAt:     now,
		ExpiresAt:     now.Add(s.ttl),
		LastSeenAt:    now,
	}
	stored, err := s.repo.CreateSession(ctx, session)
	if err != nil {
		return adminapp.LoginResponse{}, err
	}
	if err := s.repo.UpdateOperatorLastLogin(ctx, operator.ID, now); err != nil {
		return adminapp.LoginResponse{}, err
	}
	stored.LastSeenAt = now
	return adminapp.LoginResponse{
		Authenticated: true,
		Session:       sessionResponse(stored, operator, csrfToken),
		CSRFToken:     csrfToken,
	}, nil
}

// Session returns the current Admin browser session and actor.
func (s *Service) Session(ctx context.Context, sessionID string) (adminapp.Session, adminapp.Actor, error) {
	return s.session(ctx, sessionID, true)
}

// SessionResponse returns safe session metadata and rotates the CSRF token.
func (s *Service) SessionResponse(ctx context.Context, sessionID string) (adminapp.SessionResponse, error) {
	session, actor, err := s.session(ctx, sessionID, true)
	if err != nil {
		return adminapp.SessionResponse{}, err
	}
	operator, ok, err := s.repo.GetOperator(ctx, actor.OperatorID)
	if err != nil {
		return adminapp.SessionResponse{}, err
	}
	if !ok {
		return adminapp.SessionResponse{}, apperr.Unauthorized("admin operator is unavailable")
	}
	csrfToken := newToken(csrfTokenBytes)
	session.CSRFHash = hashToken(csrfToken)
	stored, err := s.repo.CreateSession(ctx, session)
	if err != nil {
		return adminapp.SessionResponse{}, err
	}
	return sessionResponse(stored, operator, csrfToken), nil
}

// Logout revokes the current Admin browser session.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return apperr.ConfigUnavailable("admin web repository is unavailable")
	}
	_, _, err := s.repo.RevokeSession(ctx, sessionID, s.now())
	return err
}

// ValidateCSRF checks a browser mutation CSRF token.
func (s *Service) ValidateCSRF(ctx context.Context, sessionID string, token string) error {
	session, _, err := s.session(ctx, sessionID, false)
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return apperr.Unauthorized("csrf token is required")
	}
	if subtle.ConstantTimeCompare([]byte(session.CSRFHash), []byte(hashToken(token))) != 1 {
		return apperr.Unauthorized("invalid csrf token")
	}
	return nil
}

// Authorize verifies actor permission for one action/resource pair.
func (s *Service) Authorize(actor adminapp.Actor, action string, resource string) error {
	if hasPermission(actor.Roles, action, resource) {
		return nil
	}
	return apperr.Forbidden("admin permission denied")
}

// Dashboard returns Admin dashboard counters.
func (s *Service) Dashboard(ctx context.Context, actor adminapp.Actor) (adminapp.Dashboard, error) {
	if err := s.Authorize(actor, "read", "dashboard"); err != nil {
		return adminapp.Dashboard{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	if err != nil {
		return adminapp.Dashboard{}, err
	}
	tenants, err := s.owner.ListTenants(ctx)
	if err != nil {
		return adminapp.Dashboard{}, err
	}
	projects, err := s.owner.ListProjects(ctx, "")
	if err != nil {
		return adminapp.Dashboard{}, err
	}
	taskCount, _ := s.taskCount(ctx)
	failedSettlementCount, _ := s.failedSettlementCount(ctx)
	dueCallbackCount, _ := s.dueCallbackCount(ctx)
	return adminapp.Dashboard{
		GeneratedAt: s.now(),
		Counts: adminapp.DashboardCounts{
			Tenants:           len(tenants),
			Projects:          len(projects),
			APIKeys:           len(cfg.APIKeys) + len(cfg.RevokedKeys),
			Models:            len(cfg.Models),
			Channels:          len(cfg.Channels),
			Routes:            len(cfg.Routes),
			PricingRules:      len(cfg.Prices),
			LimitRules:        len(cfg.Limits),
			Tasks:             taskCount,
			FailedSettlements: failedSettlementCount,
			DueCallbacks:      dueCallbackCount,
		},
	}, nil
}

// ListTenants returns tenant read models.
func (s *Service) ListTenants(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[cpadmin.Tenant], error) {
	if err := s.Authorize(actor, "read", "tenant"); err != nil {
		return adminapp.ListResponse[cpadmin.Tenant]{}, err
	}
	tenants, err := s.owner.ListTenants(ctx)
	return adminapp.ListResponse[cpadmin.Tenant]{Data: tenants}, err
}

// GetTenant returns one tenant read model.
func (s *Service) GetTenant(ctx context.Context, actor adminapp.Actor, tenantID string) (cpadmin.Tenant, error) {
	tenants, err := s.ListTenants(ctx, actor)
	if err != nil {
		return cpadmin.Tenant{}, err
	}
	for _, tenant := range tenants.Data {
		if tenant.ID == tenantID {
			return tenant, nil
		}
	}
	return cpadmin.Tenant{}, apperr.NotFound("tenant not found")
}

// UpsertTenant writes tenant configuration through the control-plane owner service.
func (s *Service) UpsertTenant(ctx context.Context, actor adminapp.Actor, request cpadmin.Tenant, opts adminapp.MutationOptions) (*cpadmin.Tenant, error) {
	return mutate(ctx, s, actor, opts, "write", "tenant", request.ID, request, func() (*cpadmin.Tenant, error) {
		return s.owner.UpsertTenant(ctx, request)
	})
}

// ListProjects returns project read models.
func (s *Service) ListProjects(ctx context.Context, actor adminapp.Actor, tenantID string) (adminapp.ListResponse[cpadmin.Project], error) {
	if err := s.Authorize(actor, "read", "project"); err != nil {
		return adminapp.ListResponse[cpadmin.Project]{}, err
	}
	projects, err := s.owner.ListProjects(ctx, tenantID)
	return adminapp.ListResponse[cpadmin.Project]{Data: projects}, err
}

// UpsertProject writes project configuration through the control-plane owner service.
func (s *Service) UpsertProject(ctx context.Context, actor adminapp.Actor, request cpadmin.Project, opts adminapp.MutationOptions) (*cpadmin.Project, error) {
	return mutate(ctx, s, actor, opts, "write", "project", request.ID, request, func() (*cpadmin.Project, error) {
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
func (s *Service) CreateAPIKey(ctx context.Context, actor adminapp.Actor, request cpadmin.APIKey, opts adminapp.MutationOptions) (*cpadmin.APIKey, error) {
	return mutate(ctx, s, actor, opts, "write", "api_key", request.ID, request, func() (*cpadmin.APIKey, error) {
		return s.owner.CreateAPIKey(ctx, request)
	})
}

// DisableAPIKey disables an API key through the control-plane owner service.
func (s *Service) DisableAPIKey(ctx context.Context, actor adminapp.Actor, keyID string, opts adminapp.MutationOptions) (*cpadmin.APIKey, error) {
	return mutate(ctx, s, actor, opts, "write", "api_key", keyID, map[string]string{"id": keyID}, func() (*cpadmin.APIKey, error) {
		return s.owner.DisableAPIKey(ctx, keyID)
	})
}

// ListModels returns public model configuration read models.
func (s *Service) ListModels(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[cpadmin.ModelConfig], error) {
	if err := s.Authorize(actor, "read", "model"); err != nil {
		return adminapp.ListResponse[cpadmin.ModelConfig]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	return adminapp.ListResponse[cpadmin.ModelConfig]{Data: cfg.Models}, err
}

// UpsertModel writes model configuration through the control-plane owner service.
func (s *Service) UpsertModel(ctx context.Context, actor adminapp.Actor, request cpadmin.ModelConfig, opts adminapp.MutationOptions) (*cpadmin.ModelConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "model", request.PublicModel, request, func() (*cpadmin.ModelConfig, error) {
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
func (s *Service) UpsertChannel(ctx context.Context, actor adminapp.Actor, request cpadmin.ChannelConfig, opts adminapp.MutationOptions) (*cpadmin.ChannelConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "channel", request.ID, request, func() (*cpadmin.ChannelConfig, error) {
		return s.owner.UpsertChannel(ctx, request)
	})
}

// SetChannelEnabled updates a channel enabled flag through the control-plane owner service.
func (s *Service) SetChannelEnabled(ctx context.Context, actor adminapp.Actor, channelID string, enabled bool, opts adminapp.MutationOptions) (*cpadmin.ChannelConfig, error) {
	request := map[string]any{"id": channelID, "enabled": enabled}
	return mutate(ctx, s, actor, opts, "write", "channel", channelID, request, func() (*cpadmin.ChannelConfig, error) {
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
func (s *Service) ListRoutes(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[cpadmin.RoutePolicyConfig], error) {
	if err := s.Authorize(actor, "read", "route"); err != nil {
		return adminapp.ListResponse[cpadmin.RoutePolicyConfig]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	return adminapp.ListResponse[cpadmin.RoutePolicyConfig]{Data: cfg.Routes}, err
}

// UpsertRoute writes route configuration through the control-plane owner service.
func (s *Service) UpsertRoute(ctx context.Context, actor adminapp.Actor, request cpadmin.RoutePolicyConfig, opts adminapp.MutationOptions) (*cpadmin.RoutePolicyConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "route", request.ID, request, func() (*cpadmin.RoutePolicyConfig, error) {
		return s.owner.UpsertRoute(ctx, request)
	})
}

// ListPricing returns price rule read models.
func (s *Service) ListPricing(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[cpadmin.PriceRuleConfig], error) {
	if err := s.Authorize(actor, "read", "pricing"); err != nil {
		return adminapp.ListResponse[cpadmin.PriceRuleConfig]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	return adminapp.ListResponse[cpadmin.PriceRuleConfig]{Data: cfg.Prices}, err
}

// UpsertPrice writes price configuration through the control-plane owner service.
func (s *Service) UpsertPrice(ctx context.Context, actor adminapp.Actor, request cpadmin.PriceRuleConfig, opts adminapp.MutationOptions) (*cpadmin.PriceRuleConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "pricing", request.PublicModel, request, func() (*cpadmin.PriceRuleConfig, error) {
		return s.owner.UpsertPrice(ctx, request)
	})
}

// ListLimits returns limit rule read models.
func (s *Service) ListLimits(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[cpadmin.LimitRuleConfig], error) {
	if err := s.Authorize(actor, "read", "limit"); err != nil {
		return adminapp.ListResponse[cpadmin.LimitRuleConfig]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	return adminapp.ListResponse[cpadmin.LimitRuleConfig]{Data: cfg.Limits}, err
}

// UpsertLimit writes limit configuration through the control-plane owner service.
func (s *Service) UpsertLimit(ctx context.Context, actor adminapp.Actor, request cpadmin.LimitRuleConfig, opts adminapp.MutationOptions) (*cpadmin.LimitRuleConfig, error) {
	return mutate(ctx, s, actor, opts, "write", "limit", request.ID, request, func() (*cpadmin.LimitRuleConfig, error) {
		return s.owner.UpsertLimit(ctx, request)
	})
}

// SnapshotDiagnostics returns safe active and rollback snapshot state.
func (s *Service) SnapshotDiagnostics(ctx context.Context, actor adminapp.Actor) (adminapp.SnapshotSummary, error) {
	if err := s.Authorize(actor, "read", "snapshot"); err != nil {
		return adminapp.SnapshotSummary{}, err
	}
	if s.snapshots == nil {
		return adminapp.SnapshotSummary{}, apperr.ConfigUnavailable("snapshot manager is unavailable")
	}
	diagnostics, err := s.snapshots.Diagnostics(ctx)
	if err != nil {
		return adminapp.SnapshotSummary{}, err
	}
	return adminapp.SnapshotSummary{Active: diagnostics.Active, Previous: diagnostics.Previous}, nil
}

// ValidateSnapshot validates the current config graph enough for browser preflight.
func (s *Service) ValidateSnapshot(ctx context.Context, actor adminapp.Actor, opts adminapp.MutationOptions) (map[string]any, error) {
	return mutate(ctx, s, actor, opts, "publish", "snapshot", "validate", map[string]string{"operation": "validate"}, func() (map[string]any, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return nil, err
		}
		for _, channel := range cfg.Channels {
			if channel.APIKey != "" {
				return nil, apperr.InvalidArgument("snapshot must not contain plaintext provider credentials")
			}
		}
		return map[string]any{
			"valid":        true,
			"api_keys":     len(cfg.APIKeys),
			"models":       len(cfg.Models),
			"channels":     len(cfg.Channels),
			"routes":       len(cfg.Routes),
			"pricing":      len(cfg.Prices),
			"limits":       len(cfg.Limits),
			"generated_at": s.now(),
		}, nil
	})
}

// PublishSnapshot publishes a runtime snapshot through the snapshot owner.
func (s *Service) PublishSnapshot(ctx context.Context, actor adminapp.Actor, opts adminapp.MutationOptions) (adminapp.SnapshotOperationResult, error) {
	return mutate(ctx, s, actor, opts, "publish", "snapshot", "active", map[string]string{"operation": "publish"}, func() (adminapp.SnapshotOperationResult, error) {
		if s.snapshots == nil {
			return adminapp.SnapshotOperationResult{}, apperr.ConfigUnavailable("snapshot manager is unavailable")
		}
		runtime, err := s.snapshots.Publish(ctx)
		if err != nil {
			return adminapp.SnapshotOperationResult{}, err
		}
		return safeSnapshotResult(runtime), nil
	})
}

// RollbackSnapshot rolls back to the previous runtime snapshot through the snapshot owner.
func (s *Service) RollbackSnapshot(ctx context.Context, actor adminapp.Actor, opts adminapp.MutationOptions) (adminapp.SnapshotOperationResult, error) {
	return mutate(ctx, s, actor, opts, "publish", "snapshot", "rollback", map[string]string{"operation": "rollback"}, func() (adminapp.SnapshotOperationResult, error) {
		if s.snapshots == nil {
			return adminapp.SnapshotOperationResult{}, apperr.ConfigUnavailable("snapshot manager is unavailable")
		}
		runtime, err := s.snapshots.Rollback(ctx)
		if err != nil {
			return adminapp.SnapshotOperationResult{}, err
		}
		return safeSnapshotResult(runtime), nil
	})
}

// ListFailedSettlements returns safe failed settlement operations.
func (s *Service) ListFailedSettlements(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[reporting.FailedSettlementSummary], error) {
	if err := s.Authorize(actor, "read", "settlement"); err != nil {
		return adminapp.ListResponse[reporting.FailedSettlementSummary]{}, err
	}
	if s.commercial == nil {
		return adminapp.ListResponse[reporting.FailedSettlementSummary]{}, apperr.ConfigUnavailable("commercial reporting is unavailable")
	}
	report, err := s.commercial.ReconciliationReport(ctx)
	if err != nil {
		return adminapp.ListResponse[reporting.FailedSettlementSummary]{}, err
	}
	return adminapp.ListResponse[reporting.FailedSettlementSummary]{Data: report.FailedSettlements}, nil
}

// ReplayFailedSettlement triggers the owner failed-settlement replay workflow.
func (s *Service) ReplayFailedSettlement(ctx context.Context, actor adminapp.Actor, settlementID string, opts adminapp.MutationOptions) (adminapp.ReplayResult, error) {
	return mutate(ctx, s, actor, opts, "replay", "settlement", settlementID, map[string]string{"id": settlementID}, func() (adminapp.ReplayResult, error) {
		if s.failedSettlements == nil {
			return adminapp.ReplayResult{}, apperr.ConfigUnavailable("failed settlement repair is unavailable")
		}
		replayed, err := s.failedSettlements.ReplayPending(ctx, 1)
		return adminapp.ReplayResult{RequestedID: settlementID, Replayed: replayed}, err
	})
}

// ListCallbacks returns safe due callback operations.
func (s *Service) ListCallbacks(ctx context.Context, actor adminapp.Actor, limit int) (adminapp.ListResponse[adminapp.CallbackEventView], error) {
	if err := s.Authorize(actor, "read", "callback"); err != nil {
		return adminapp.ListResponse[adminapp.CallbackEventView]{}, err
	}
	if s.tasks == nil {
		return adminapp.ListResponse[adminapp.CallbackEventView]{}, apperr.ConfigUnavailable("task repository is unavailable")
	}
	events, err := s.tasks.ListDueCallbacks(ctx, normalizeLimit(limit), s.now())
	if err != nil {
		return adminapp.ListResponse[adminapp.CallbackEventView]{}, err
	}
	views := make([]adminapp.CallbackEventView, 0, len(events))
	for _, event := range events {
		views = append(views, safeCallback(event))
	}
	return adminapp.ListResponse[adminapp.CallbackEventView]{Data: views}, nil
}

// RetryCallback returns a stable disabled response until callback owner exposes single-row retry.
func (s *Service) RetryCallback(ctx context.Context, actor adminapp.Actor, callbackID string, opts adminapp.MutationOptions) (adminapp.ReplayResult, error) {
	return mutate(ctx, s, actor, opts, "retry", "callback", callbackID, map[string]string{"id": callbackID}, func() (adminapp.ReplayResult, error) {
		if s.tasks == nil {
			return adminapp.ReplayResult{}, apperr.ConfigUnavailable("task repository is unavailable")
		}
		err := s.tasks.MarkCallbackFailed(ctx, callbackID, "", s.now(), "manual retry requested by admin", 0, 0)
		return adminapp.ReplayResult{RequestedID: callbackID, Replayed: 1}, err
	})
}

// ListWorkers returns worker operation read models.
func (s *Service) ListWorkers(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[adminapp.WorkerJobView], error) {
	if err := s.Authorize(actor, "read", "worker"); err != nil {
		return adminapp.ListResponse[adminapp.WorkerJobView]{}, err
	}
	return adminapp.ListResponse[adminapp.WorkerJobView]{Data: []adminapp.WorkerJobView{
		{Name: "failed_settlement_replayer", Status: "configured_by_worker_process"},
		{Name: "callback_dispatcher", Status: "configured_by_worker_process"},
		{Name: "provider_task_poller", Status: "configured_by_worker_process"},
	}}, nil
}

// ListHolds returns hold-aging read models.
func (s *Service) ListHolds(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[adminapp.HoldAgingView], error) {
	if err := s.Authorize(actor, "read", "hold"); err != nil {
		return adminapp.ListResponse[adminapp.HoldAgingView]{}, err
	}
	return adminapp.ListResponse[adminapp.HoldAgingView]{Data: []adminapp.HoldAgingView{{Status: "active", Count: 0}}}, nil
}

// ListAuditEvents returns redacted durable Admin audit events.
func (s *Service) ListAuditEvents(ctx context.Context, actor adminapp.Actor, filter adminapp.AuditFilter) (adminapp.ListResponse[adminapp.AuditEvent], error) {
	if err := s.Authorize(actor, "read", "audit"); err != nil {
		return adminapp.ListResponse[adminapp.AuditEvent]{}, err
	}
	events, err := s.repo.ListAuditEvents(ctx, filter)
	return adminapp.ListResponse[adminapp.AuditEvent]{Data: events}, err
}

// ListOperators returns safe Admin operator metadata.
func (s *Service) ListOperators(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[adminapp.Operator], error) {
	if err := s.Authorize(actor, "read", "operator"); err != nil {
		return adminapp.ListResponse[adminapp.Operator]{}, err
	}
	operators, err := s.repo.ListOperators(ctx)
	return adminapp.ListResponse[adminapp.Operator]{Data: operators}, err
}

// CreateOperator creates an Admin operator and audits the mutation.
func (s *Service) CreateOperator(ctx context.Context, actor adminapp.Actor, request adminapp.OperatorCreateRequest, opts adminapp.MutationOptions) (adminapp.Operator, error) {
	return mutate(ctx, s, actor, opts, "write", "operator", "", request, func() (adminapp.Operator, error) {
		email := normalizeEmail(request.Email)
		if email == "" || strings.TrimSpace(request.Password) == "" {
			return adminapp.Operator{}, apperr.InvalidArgument("email and password are required")
		}
		if _, ok, err := s.repo.GetOperatorByEmail(ctx, email); err != nil || ok {
			if ok {
				return adminapp.Operator{}, apperr.InvalidArgument("operator email already exists")
			}
			return adminapp.Operator{}, err
		}
		enabled := request.Enabled
		if !enabled {
			enabled = true
		}
		now := s.now()
		operator := adminapp.Operator{
			ID:           newID("operator"),
			Email:        email,
			DisplayName:  strings.TrimSpace(request.DisplayName),
			PasswordHash: HashPassword(request.Password),
			Roles:        normalizeRoles(request.Roles),
			Enabled:      enabled,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if len(operator.Roles) == 0 {
			operator.Roles = []adminapp.Role{adminapp.RoleReadOnly}
		}
		return s.repo.SaveOperator(ctx, operator)
	})
}

// DisableOperator disables an Admin operator and audits the mutation.
func (s *Service) DisableOperator(ctx context.Context, actor adminapp.Actor, operatorID string, opts adminapp.MutationOptions) (adminapp.Operator, error) {
	return mutate(ctx, s, actor, opts, "write", "operator", operatorID, map[string]string{"id": operatorID}, func() (adminapp.Operator, error) {
		if operatorID == actor.OperatorID {
			return adminapp.Operator{}, apperr.InvalidArgument("operators cannot disable themselves")
		}
		operator, ok, err := s.repo.DisableOperator(ctx, operatorID, s.now())
		if err != nil {
			return adminapp.Operator{}, err
		}
		if !ok {
			return adminapp.Operator{}, apperr.NotFound("operator not found")
		}
		return operator, nil
	})
}

func (s *Service) session(ctx context.Context, sessionID string, touch bool) (adminapp.Session, adminapp.Actor, error) {
	sessionID = strings.TrimSpace(sessionID)
	if s == nil || s.repo == nil {
		return adminapp.Session{}, adminapp.Actor{}, apperr.ConfigUnavailable("admin web repository is unavailable")
	}
	if sessionID == "" {
		return adminapp.Session{}, adminapp.Actor{}, apperr.Unauthorized("admin session is required")
	}
	session, ok, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return adminapp.Session{}, adminapp.Actor{}, err
	}
	if !ok {
		return adminapp.Session{}, adminapp.Actor{}, apperr.Unauthorized("admin session is invalid")
	}
	now := s.now()
	if session.RevokedAt != nil {
		return adminapp.Session{}, adminapp.Actor{}, apperr.Unauthorized("admin session is revoked")
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(now) {
		return adminapp.Session{}, adminapp.Actor{}, apperr.Unauthorized("admin session is expired")
	}
	operator, ok, err := s.repo.GetOperator(ctx, session.OperatorID)
	if err != nil {
		return adminapp.Session{}, adminapp.Actor{}, err
	}
	if !ok || !operator.Enabled {
		return adminapp.Session{}, adminapp.Actor{}, apperr.Unauthorized("admin operator is unavailable")
	}
	if touch {
		session.LastSeenAt = now
		if err := s.repo.TouchSession(ctx, session.ID, now); err != nil {
			return adminapp.Session{}, adminapp.Actor{}, err
		}
	}
	return session, actorFromOperator(operator), nil
}

func mutate[T any](ctx context.Context, s *Service, actor adminapp.Actor, opts adminapp.MutationOptions, action string, resource string, resourceID string, before any, fn func() (T, error)) (T, error) {
	var zero T
	if s == nil || s.repo == nil {
		return zero, apperr.ConfigUnavailable("admin web repository is unavailable")
	}
	if err := validateMutationOptions(opts); err != nil {
		return zero, err
	}
	if err := s.Authorize(actor, action, resource); err != nil {
		audit := adminapp.AuditEvent{
			ID:             newID("audit"),
			Actor:          actor,
			Action:         action,
			Resource:       resource,
			ResourceID:     resourceID,
			RequestID:      opts.RequestID,
			IdempotencyKey: opts.IdempotencyKey,
			Reason:         opts.Reason,
			Status:         auditStatusFailed,
			ErrorCode:      errorCodeOf(err),
			Before:         redactedJSON(before),
			RemoteAddr:     strings.TrimSpace(opts.RemoteAddr),
			UserAgentHash:  hashText(opts.UserAgent),
			CreatedAt:      s.now(),
		}
		_, _ = s.repo.CreateAuditEvent(ctx, audit)
		return zero, err
	}

	result, err := fn()
	status := auditStatusOK
	errorCode := ""
	if err != nil {
		status = auditStatusFailed
		errorCode = errorCodeOf(err)
	}
	audit := adminapp.AuditEvent{
		ID:             newID("audit"),
		Actor:          actor,
		Action:         action,
		Resource:       resource,
		ResourceID:     resourceID,
		RequestID:      opts.RequestID,
		IdempotencyKey: opts.IdempotencyKey,
		Reason:         opts.Reason,
		Status:         status,
		ErrorCode:      errorCode,
		Before:         redactedJSON(before),
		After:          redactedJSON(result),
		RemoteAddr:     strings.TrimSpace(opts.RemoteAddr),
		UserAgentHash:  hashText(opts.UserAgent),
		CreatedAt:      s.now(),
	}
	if _, auditErr := s.repo.CreateAuditEvent(ctx, audit); auditErr != nil && err == nil {
		return zero, auditErr
	}
	return result, err
}

func validateMutationOptions(opts adminapp.MutationOptions) error {
	if strings.TrimSpace(opts.IdempotencyKey) == "" {
		return apperr.InvalidArgument("idempotency key is required")
	}
	if strings.TrimSpace(opts.Reason) == "" {
		return apperr.InvalidArgument("reason is required")
	}
	return nil
}

func (s *Service) snapshotConfig(ctx context.Context) (*cpadmin.SnapshotConfig, error) {
	if s == nil || s.owner == nil {
		return nil, apperr.ConfigUnavailable("control-plane admin service is unavailable")
	}
	return s.owner.LoadSnapshotConfig(ctx)
}

func (s *Service) taskCount(ctx context.Context) (int, error) {
	if s.tasks == nil {
		return 0, nil
	}
	tasks, err := s.tasks.ListTasks(ctx, tasksvc.TaskListFilter{Limit: 200})
	return len(tasks), err
}

func (s *Service) failedSettlementCount(ctx context.Context) (int, error) {
	if s.commercial == nil {
		return 0, nil
	}
	report, err := s.commercial.ReconciliationReport(ctx)
	if err != nil {
		return 0, err
	}
	return len(report.FailedSettlements), nil
}

func (s *Service) dueCallbackCount(ctx context.Context) (int, error) {
	if s.tasks == nil {
		return 0, nil
	}
	callbacks, err := s.tasks.ListDueCallbacks(ctx, 200, s.now())
	return len(callbacks), err
}

func safeAPIKey(key cpadmin.APIKey) adminapp.APIKeyView {
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

func safeChannel(channel cpadmin.ChannelConfig) adminapp.ChannelView {
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

func safeCallback(event tasksvc.CallbackEvent) adminapp.CallbackEventView {
	return adminapp.CallbackEventView{
		ID:             event.ID,
		TaskID:         event.TaskID,
		TenantID:       event.TenantID,
		ProjectID:      event.ProjectID,
		Status:         string(event.Status),
		RetryCount:     event.RetryCount,
		NextRetryAt:    event.NextRetryAt,
		LastError:      safeShort(event.LastError),
		OwnerID:        event.OwnerID,
		LastStatusCode: event.LastStatusCode,
		LastLatencyMS:  event.LastLatencyMS,
		CreatedAt:      event.CreatedAt,
		UpdatedAt:      event.UpdatedAt,
	}
}

func safeSnapshotResult(runtime *cpsnapshot.RuntimeSnapshot) adminapp.SnapshotOperationResult {
	if runtime == nil {
		return adminapp.SnapshotOperationResult{}
	}
	return adminapp.SnapshotOperationResult{
		Version:       runtime.Version,
		Checksum:      runtime.Checksum,
		SchemaVersion: runtime.SchemaVersion,
		CreatedAt:     runtime.CreatedAt,
	}
}

func sessionResponse(session adminapp.Session, operator adminapp.Operator, csrfToken string) adminapp.SessionResponse {
	return adminapp.SessionResponse{
		SessionID:     session.ID,
		Authenticated: true,
		OperatorID:    operator.ID,
		Email:         operator.Email,
		DisplayName:   operator.DisplayName,
		Roles:         append([]adminapp.Role(nil), operator.Roles...),
		Permissions:   permissionsForRoles(operator.Roles),
		ExpiresAt:     session.ExpiresAt,
		LastSeenAt:    session.LastSeenAt,
		CSRFToken:     csrfToken,
	}
}

func actorFromOperator(operator adminapp.Operator) adminapp.Actor {
	return adminapp.Actor{
		OperatorID:  operator.ID,
		Email:       operator.Email,
		DisplayName: operator.DisplayName,
		Roles:       append([]adminapp.Role(nil), operator.Roles...),
	}
}

func permissionsForRoles(roles []adminapp.Role) []adminapp.Permission {
	seen := map[string]adminapp.Permission{}
	for _, role := range normalizeRoles(roles) {
		for _, permission := range rolePermissions[role] {
			key := permission.Action + ":" + permission.Resource
			seen[key] = permission
		}
	}
	permissions := make([]adminapp.Permission, 0, len(seen))
	for _, permission := range seen {
		permissions = append(permissions, permission)
	}
	sort.Slice(permissions, func(i, j int) bool {
		if permissions[i].Resource == permissions[j].Resource {
			return permissions[i].Action < permissions[j].Action
		}
		return permissions[i].Resource < permissions[j].Resource
	})
	return permissions
}

func hasPermission(roles []adminapp.Role, action string, resource string) bool {
	action = strings.TrimSpace(action)
	resource = strings.TrimSpace(resource)
	for _, role := range normalizeRoles(roles) {
		if role == adminapp.RoleSuperAdmin {
			return true
		}
		for _, permission := range rolePermissions[role] {
			actionOK := permission.Action == "*" || permission.Action == action
			resourceOK := permission.Resource == "*" || permission.Resource == resource
			if actionOK && resourceOK {
				return true
			}
		}
	}
	return false
}

var rolePermissions = map[adminapp.Role][]adminapp.Permission{
	adminapp.RoleSuperAdmin: {{Action: "*", Resource: "*"}},
	adminapp.RoleConfigAdmin: {
		{Action: "read", Resource: "dashboard"},
		{Action: "read", Resource: "tenant"}, {Action: "write", Resource: "tenant"},
		{Action: "read", Resource: "project"}, {Action: "write", Resource: "project"},
		{Action: "read", Resource: "api_key"}, {Action: "write", Resource: "api_key"},
		{Action: "read", Resource: "model"}, {Action: "write", Resource: "model"},
		{Action: "read", Resource: "channel"}, {Action: "write", Resource: "channel"},
		{Action: "read", Resource: "route"}, {Action: "write", Resource: "route"},
		{Action: "read", Resource: "pricing"}, {Action: "write", Resource: "pricing"},
		{Action: "read", Resource: "limit"}, {Action: "write", Resource: "limit"},
		{Action: "read", Resource: "snapshot"}, {Action: "publish", Resource: "snapshot"},
	},
	adminapp.RoleFinanceAdmin: {
		{Action: "read", Resource: "dashboard"},
		{Action: "read", Resource: "tenant"}, {Action: "read", Resource: "project"},
		{Action: "read", Resource: "api_key"}, {Action: "read", Resource: "pricing"},
		{Action: "read", Resource: "limit"}, {Action: "read", Resource: "settlement"},
		{Action: "replay", Resource: "settlement"}, {Action: "read", Resource: "hold"},
		{Action: "read", Resource: "audit"},
	},
	adminapp.RoleSupport: {
		{Action: "read", Resource: "dashboard"},
		{Action: "read", Resource: "tenant"}, {Action: "read", Resource: "project"},
		{Action: "read", Resource: "api_key"}, {Action: "read", Resource: "model"},
		{Action: "read", Resource: "task"}, {Action: "read", Resource: "callback"},
	},
	adminapp.RoleOps: {
		{Action: "read", Resource: "dashboard"},
		{Action: "read", Resource: "snapshot"}, {Action: "publish", Resource: "snapshot"},
		{Action: "read", Resource: "settlement"}, {Action: "replay", Resource: "settlement"},
		{Action: "read", Resource: "callback"}, {Action: "retry", Resource: "callback"},
		{Action: "read", Resource: "worker"}, {Action: "read", Resource: "hold"},
	},
	adminapp.RoleReadOnly: {
		{Action: "read", Resource: "dashboard"},
		{Action: "read", Resource: "tenant"}, {Action: "read", Resource: "project"},
		{Action: "read", Resource: "api_key"}, {Action: "read", Resource: "model"},
		{Action: "read", Resource: "channel"}, {Action: "read", Resource: "route"},
		{Action: "read", Resource: "pricing"}, {Action: "read", Resource: "limit"},
		{Action: "read", Resource: "snapshot"}, {Action: "read", Resource: "settlement"},
		{Action: "read", Resource: "callback"}, {Action: "read", Resource: "worker"},
		{Action: "read", Resource: "hold"},
	},
}

func normalizeRoles(roles []adminapp.Role) []adminapp.Role {
	seen := map[adminapp.Role]struct{}{}
	var out []adminapp.Role
	for _, role := range roles {
		role = adminapp.Role(strings.TrimSpace(string(role)))
		if role == "" {
			continue
		}
		if _, ok := rolePermissions[role]; !ok {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	return out
}

// HashPassword returns the stable password hash format used for Admin operators.
func HashPassword(password string) string {
	salt := newToken(16)
	sum := sha256.Sum256([]byte(salt + ":" + strings.TrimSpace(password)))
	return "admin$sha256$" + salt + "$" + hex.EncodeToString(sum[:])
}

func verifyPassword(hash string, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "admin" || parts[1] != "sha256" {
		return false
	}
	sum := sha256.Sum256([]byte(parts[2] + ":" + strings.TrimSpace(password)))
	return subtle.ConstantTimeCompare([]byte(parts[3]), []byte(hex.EncodeToString(sum[:]))) == 1
}

func redactedJSON(value any) json.RawMessage {
	content, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		return nil
	}
	redacted := redactValue(decoded, "")
	content, err = json.Marshal(redacted)
	if err != nil {
		return nil
	}
	return content
}

func redactValue(value any, key string) any {
	if sensitiveKey(key) {
		return "[REDACTED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			out[childKey] = redactValue(childValue, childKey)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, redactValue(item, key))
		}
		return out
	default:
		return value
	}
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "password", "api_key", "key_hash", "plaintext_key", "encrypted_api_key", "access_token", "refresh_token", "prompt", "response", "payload":
		return true
	}
	return strings.Contains(key, "secret") || strings.Contains(key, "credential") || strings.Contains(key, "ciphertext")
}

func safeShort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > 240 {
		return value[:240]
	}
	return value
}

func errorCodeOf(err error) string {
	if appErr, ok := apperr.As(err); ok {
		return string(appErr.Code)
	}
	return string(apperr.CodeInternal)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hashText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func newID(prefix string) string {
	return prefix + "_" + newToken(12)
}
