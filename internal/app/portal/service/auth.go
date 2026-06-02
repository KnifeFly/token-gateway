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
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

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
		if key.ID == principal.APIKeyID && key.Enabled && key.RevokedAt == nil && (key.ExpiresAt == nil || key.ExpiresAt.After(s.now())) {
			return nil
		}
	}
	return apperr.Unauthorized("portal session api key is revoked")
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
