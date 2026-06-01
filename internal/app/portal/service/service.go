package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	legacyportal "github.com/KnifeFly/token-gateway/internal/portal"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const (
	defaultSessionTTL = 12 * time.Hour
	sessionIDBytes    = 32
	csrfTokenBytes    = 32
)

// SnapshotProvider attaches a runtime snapshot to a request state.
type SnapshotProvider interface {
	Attach(context.Context, *engine.RequestState) error
}

// Authenticator authenticates a request state against a snapshot.
type Authenticator interface {
	Authenticate(context.Context, *engine.RequestState) error
}

// Service coordinates browser-facing Portal Web BFF use cases.
type Service struct {
	snapshot SnapshotProvider
	auth     Authenticator
	legacy   *legacyportal.Service
	sessions portalapp.SessionStore
	now      func() time.Time
	ttl      time.Duration
}

// Option customizes the Portal Web service.
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

// New returns a Portal Web BFF service.
func New(snapshot SnapshotProvider, auth Authenticator, legacy *legacyportal.Service, sessions portalapp.SessionStore, opts ...Option) *Service {
	s := &Service{
		snapshot: snapshot,
		auth:     auth,
		legacy:   legacy,
		sessions: sessions,
		now:      func() time.Time { return time.Now().UTC() },
		ttl:      defaultSessionTTL,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// LoginWithAPIKey exchanges a customer API key for a browser session.
func (s *Service) LoginWithAPIKey(ctx context.Context, request portalapp.APIKeyLoginRequest, userAgent string, remoteAddr string) (portalapp.LoginResponse, error) {
	if s == nil || s.snapshot == nil || s.auth == nil || s.sessions == nil {
		return portalapp.LoginResponse{}, apperr.ConfigUnavailable("portal web service is unavailable")
	}
	apiKey := strings.TrimSpace(request.APIKey)
	if apiKey == "" {
		return portalapp.LoginResponse{}, apperr.Unauthorized("missing api key")
	}
	state := &engine.RequestState{
		RequestID: newToken(16),
		StartedAt: s.now(),
		Incoming: engine.IncomingRequest{
			Method:     http.MethodPost,
			Path:       "/api/portal/v1/auth/api-key-login",
			Header:     http.Header{"Authorization": []string{"Bearer " + apiKey}},
			RemoteAddr: remoteAddr,
		},
		Metadata: make(map[string]string),
		Internal: make(map[string]any),
	}
	if err := s.snapshot.Attach(ctx, state); err != nil {
		return portalapp.LoginResponse{}, err
	}
	if err := s.auth.Authenticate(ctx, state); err != nil {
		return portalapp.LoginResponse{}, err
	}
	principal := principalFromState(state)
	now := s.now()
	csrfToken := newToken(csrfTokenBytes)
	session := portalapp.Session{
		ID:            newToken(sessionIDBytes),
		TenantID:      principal.TenantID,
		ProjectID:     principal.ProjectID,
		APIKeyID:      principal.APIKeyID,
		AllowedModels: append([]string(nil), principal.AllowedModels...),
		CSRFHash:      hashToken(csrfToken),
		UserAgent:     strings.TrimSpace(userAgent),
		RemoteAddr:    strings.TrimSpace(remoteAddr),
		CreatedAt:     now,
		ExpiresAt:     now.Add(s.ttl),
		LastSeenAt:    now,
	}
	stored, err := s.sessions.Create(ctx, session)
	if err != nil {
		return portalapp.LoginResponse{}, err
	}
	return portalapp.LoginResponse{
		Authenticated: true,
		Session:       sessionResponse(stored, csrfToken),
		CSRFToken:     csrfToken,
	}, nil
}

// Session returns the current browser session and safe principal.
func (s *Service) Session(ctx context.Context, sessionID string) (portalapp.Session, portalapp.Principal, error) {
	session, principal, err := s.session(ctx, sessionID, true)
	return session, principal, err
}

// SessionResponse returns browser-safe session metadata.
func (s *Service) SessionResponse(ctx context.Context, sessionID string) (portalapp.SessionResponse, error) {
	session, _, err := s.session(ctx, sessionID, true)
	if err != nil {
		return portalapp.SessionResponse{}, err
	}
	csrfToken := newToken(csrfTokenBytes)
	session.CSRFHash = hashToken(csrfToken)
	stored, err := s.sessions.Create(ctx, session)
	if err != nil {
		return portalapp.SessionResponse{}, err
	}
	return sessionResponse(stored, csrfToken), nil
}

// Logout revokes the current browser session.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if s == nil || s.sessions == nil {
		return apperr.ConfigUnavailable("portal session store is unavailable")
	}
	_, _, err := s.sessions.Revoke(ctx, sessionID, s.now())
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

// Dashboard returns Portal dashboard read models scoped to the current project.
func (s *Service) Dashboard(ctx context.Context, principal portalapp.Principal) (portalapp.Dashboard, error) {
	credits, err := s.Credits(ctx, principal, "")
	if err != nil {
		return portalapp.Dashboard{}, err
	}
	usage, err := s.Usage(ctx, principal, portalapp.UsageFilter{Limit: 5})
	if err != nil {
		return portalapp.Dashboard{}, err
	}
	apiKeys, err := s.ListAPIKeys(ctx, principal)
	if err != nil {
		return portalapp.Dashboard{}, err
	}
	tasks, err := s.ListTasks(ctx, principal, "", 5, "")
	if err != nil {
		return portalapp.Dashboard{}, err
	}
	return portalapp.Dashboard{
		GeneratedAt:    s.now(),
		Credits:        credits,
		Usage:          usage,
		APIKeyCount:    len(apiKeys.Data),
		ActiveKeyCount: activeAPIKeyCount(apiKeys.Data),
		TaskSummary:    summarizeTasks(tasks.Data),
		RecentTasks:    tasks.Data,
	}, nil
}

// Onboarding returns the current first-run checklist.
func (s *Service) Onboarding(ctx context.Context, principal portalapp.Principal) (portalapp.OnboardingState, error) {
	keys, err := s.ListAPIKeys(ctx, principal)
	if err != nil {
		return portalapp.OnboardingState{}, err
	}
	models, err := s.ListModels(ctx, principal)
	if err != nil {
		return portalapp.OnboardingState{}, err
	}
	usage, err := s.Usage(ctx, principal, portalapp.UsageFilter{Limit: 1})
	if err != nil {
		return portalapp.OnboardingState{}, err
	}
	return portalapp.OnboardingState{
		GeneratedAt: s.now(),
		Steps: []portalapp.OnboardingStep{
			{ID: "login", Title: "Sign in", Complete: true},
			{ID: "models", Title: "Review available models", Complete: len(models.Data) > 0},
			{ID: "api_keys", Title: "Create a derived API key", Complete: len(keys.Data) > 1},
			{ID: "first_request", Title: "Send first request", Complete: usage.Totals.Requests > 0},
		},
	}, nil
}

// ProjectSettings returns safe project-scoped settings.
func (s *Service) ProjectSettings(principal portalapp.Principal) portalapp.ProjectSettings {
	return portalapp.ProjectSettings{
		TenantID:      principal.TenantID,
		ProjectID:     principal.ProjectID,
		APIKeyID:      principal.APIKeyID,
		AllowedModels: append([]string(nil), principal.AllowedModels...),
		GeneratedAt:   s.now(),
	}
}

// ListModels returns customer-visible models.
func (s *Service) ListModels(ctx context.Context, principal portalapp.Principal) (legacyportal.ModelListResponse, error) {
	snapshot, err := s.currentSnapshot(ctx)
	if err != nil {
		return legacyportal.ModelListResponse{}, err
	}
	return s.legacy.ListModels(snapshot, legacyPrincipal(principal))
}

// GetModelSchema returns a visible model schema.
func (s *Service) GetModelSchema(ctx context.Context, principal portalapp.Principal, model string) (legacyportal.ModelSchemaResponse, error) {
	snapshot, err := s.currentSnapshot(ctx)
	if err != nil {
		return legacyportal.ModelSchemaResponse{}, err
	}
	return s.legacy.GetModelSchema(snapshot, legacyPrincipal(principal), model)
}

// Credits returns customer credits.
func (s *Service) Credits(ctx context.Context, principal portalapp.Principal, currency string) (legacyportal.CreditsResponse, error) {
	return s.legacy.Credits(ctx, legacyPrincipal(principal), currency)
}

// Usage returns customer usage.
func (s *Service) Usage(ctx context.Context, principal portalapp.Principal, filter portalapp.UsageFilter) (legacyportal.UsageResponse, error) {
	return s.legacy.Usage(ctx, legacyPrincipal(principal), reporting.TenantUsageFilter{
		Currency: filter.Currency,
		From:     filter.From,
		To:       filter.To,
		Limit:    filter.Limit,
	})
}

// ListAPIKeys returns safe API key metadata.
func (s *Service) ListAPIKeys(ctx context.Context, principal portalapp.Principal) (legacyportal.APIKeyListResponse, error) {
	return s.legacy.ListAPIKeys(ctx, legacyPrincipal(principal))
}

// CreateAPIKey creates a derived API key.
func (s *Service) CreateAPIKey(ctx context.Context, principal portalapp.Principal, request legacyportal.APIKeyCreateRequest) (legacyportal.APIKeyCreateResponse, error) {
	return s.legacy.CreateAPIKey(ctx, legacyPrincipal(principal), request)
}

// DisableAPIKey disables a derived API key.
func (s *Service) DisableAPIKey(ctx context.Context, principal portalapp.Principal, keyID string) (legacyportal.APIKey, error) {
	return s.legacy.DisableAPIKey(ctx, legacyPrincipal(principal), keyID)
}

// ListTasks returns project-scoped tasks.
func (s *Service) ListTasks(ctx context.Context, principal portalapp.Principal, status string, limit int, cursor string) (legacyportal.TaskListResponse, error) {
	return s.legacy.ListTasks(ctx, legacyPrincipal(principal), status, limit, cursor)
}

// GetTask returns one project-scoped task.
func (s *Service) GetTask(ctx context.Context, principal portalapp.Principal, taskID string) (map[string]any, error) {
	return s.legacy.GetTask(ctx, legacyPrincipal(principal), taskID)
}

func (s *Service) session(ctx context.Context, sessionID string, touch bool) (portalapp.Session, portalapp.Principal, error) {
	sessionID = strings.TrimSpace(sessionID)
	if s == nil || s.sessions == nil {
		return portalapp.Session{}, portalapp.Principal{}, apperr.ConfigUnavailable("portal session store is unavailable")
	}
	if sessionID == "" {
		return portalapp.Session{}, portalapp.Principal{}, apperr.Unauthorized("portal session is required")
	}
	session, ok, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return portalapp.Session{}, portalapp.Principal{}, err
	}
	if !ok {
		return portalapp.Session{}, portalapp.Principal{}, apperr.Unauthorized("portal session is invalid")
	}
	now := s.now()
	if session.RevokedAt != nil {
		return portalapp.Session{}, portalapp.Principal{}, apperr.Unauthorized("portal session is revoked")
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(now) {
		return portalapp.Session{}, portalapp.Principal{}, apperr.Unauthorized("portal session is expired")
	}
	principal := portalapp.Principal{
		TenantID:      session.TenantID,
		ProjectID:     session.ProjectID,
		APIKeyID:      session.APIKeyID,
		AllowedModels: append([]string(nil), session.AllowedModels...),
	}
	if err := s.ensureAPIKeyActive(ctx, principal); err != nil {
		return portalapp.Session{}, portalapp.Principal{}, err
	}
	if touch {
		session.LastSeenAt = now
		if err := s.sessions.Touch(ctx, session.ID, now); err != nil {
			return portalapp.Session{}, portalapp.Principal{}, err
		}
	}
	return session, principal, nil
}

func (s *Service) ensureAPIKeyActive(ctx context.Context, principal portalapp.Principal) error {
	keys, err := s.ListAPIKeys(ctx, principal)
	if err != nil {
		return err
	}
	for _, key := range keys.Data {
		if key.ID == principal.APIKeyID && key.Enabled && key.RevokedAt == nil {
			return nil
		}
	}
	return apperr.Unauthorized("portal session api key is revoked")
}

func (s *Service) currentSnapshot(ctx context.Context) (engine.SnapshotView, error) {
	if s == nil || s.snapshot == nil {
		return nil, apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	state := &engine.RequestState{Incoming: engine.IncomingRequest{Header: http.Header{}}, Metadata: map[string]string{}, Internal: map[string]any{}}
	if err := s.snapshot.Attach(ctx, state); err != nil {
		return nil, err
	}
	return state.Snapshot, nil
}

func principalFromState(state *engine.RequestState) portalapp.Principal {
	if state == nil || state.Principal == nil {
		return portalapp.Principal{}
	}
	return portalapp.Principal{
		TenantID:      state.TenantID,
		ProjectID:     state.ProjectID,
		APIKeyID:      state.APIKeyID,
		AllowedModels: append([]string(nil), state.Principal.AllowedModels...),
	}
}

func legacyPrincipal(principal portalapp.Principal) legacyportal.Principal {
	return legacyportal.Principal{
		TenantID:      principal.TenantID,
		ProjectID:     principal.ProjectID,
		APIKeyID:      principal.APIKeyID,
		AllowedModels: append([]string(nil), principal.AllowedModels...),
	}
}

func sessionResponse(session portalapp.Session, csrfToken string) portalapp.SessionResponse {
	return portalapp.SessionResponse{
		SessionID:     session.ID,
		Authenticated: true,
		TenantID:      session.TenantID,
		ProjectID:     session.ProjectID,
		APIKeyID:      session.APIKeyID,
		AllowedModels: append([]string(nil), session.AllowedModels...),
		ExpiresAt:     session.ExpiresAt,
		LastSeenAt:    session.LastSeenAt,
		CSRFToken:     csrfToken,
	}
}

func activeAPIKeyCount(keys []legacyportal.APIKey) int {
	var count int
	for _, key := range keys {
		if key.Enabled && key.RevokedAt == nil {
			count++
		}
	}
	return count
}

func summarizeTasks(tasks []map[string]any) portalapp.TaskSummary {
	summary := portalapp.TaskSummary{Total: len(tasks)}
	for _, task := range tasks {
		switch strings.ToLower(strings.TrimSpace(anyString(task["status"]))) {
		case "queued", "pending":
			summary.Queued++
		case "processing", "running":
			summary.Processing++
		case "completed", "succeeded":
			summary.Completed++
		case "failed", "cancelled", "canceled", "expired":
			summary.Failed++
		}
	}
	return summary
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
