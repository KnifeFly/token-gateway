package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

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
		{Action: "read", Resource: "customer_account"}, {Action: "write", Resource: "customer_account"},
		{Action: "read", Resource: "api_key"}, {Action: "write", Resource: "api_key"},
		{Action: "read", Resource: "model"}, {Action: "write", Resource: "model"},
		{Action: "read", Resource: "channel"}, {Action: "write", Resource: "channel"},
		{Action: "read", Resource: "playground"}, {Action: "write", Resource: "playground"},
		{Action: "read", Resource: "usage"}, {Action: "read", Resource: "task"},
		{Action: "read", Resource: "route"}, {Action: "write", Resource: "route"},
		{Action: "read", Resource: "pricing"}, {Action: "write", Resource: "pricing"},
		{Action: "read", Resource: "limit"}, {Action: "write", Resource: "limit"},
		{Action: "read", Resource: "snapshot"}, {Action: "publish", Resource: "snapshot"},
	},
	adminapp.RoleFinanceAdmin: {
		{Action: "read", Resource: "dashboard"},
		{Action: "read", Resource: "tenant"}, {Action: "read", Resource: "project"},
		{Action: "read", Resource: "customer_account"}, {Action: "write", Resource: "customer_account"},
		{Action: "read", Resource: "api_key"}, {Action: "read", Resource: "pricing"},
		{Action: "read", Resource: "limit"}, {Action: "read", Resource: "settlement"},
		{Action: "replay", Resource: "settlement"}, {Action: "read", Resource: "hold"},
		{Action: "read", Resource: "usage"}, {Action: "read", Resource: "task"},
		{Action: "read", Resource: "audit"},
	},
	adminapp.RoleSupport: {
		{Action: "read", Resource: "dashboard"},
		{Action: "read", Resource: "tenant"}, {Action: "read", Resource: "project"},
		{Action: "read", Resource: "customer_account"},
		{Action: "read", Resource: "api_key"}, {Action: "read", Resource: "model"},
		{Action: "read", Resource: "playground"}, {Action: "write", Resource: "playground"},
		{Action: "read", Resource: "usage"}, {Action: "read", Resource: "task"},
		{Action: "read", Resource: "callback"},
	},
	adminapp.RoleOps: {
		{Action: "read", Resource: "dashboard"},
		{Action: "read", Resource: "snapshot"}, {Action: "publish", Resource: "snapshot"},
		{Action: "read", Resource: "settlement"}, {Action: "replay", Resource: "settlement"},
		{Action: "read", Resource: "callback"}, {Action: "retry", Resource: "callback"},
		{Action: "read", Resource: "worker"}, {Action: "read", Resource: "hold"},
		{Action: "read", Resource: "usage"}, {Action: "read", Resource: "task"},
		{Action: "read", Resource: "playground"}, {Action: "write", Resource: "playground"},
	},
	adminapp.RoleReadOnly: {
		{Action: "read", Resource: "dashboard"},
		{Action: "read", Resource: "tenant"}, {Action: "read", Resource: "project"},
		{Action: "read", Resource: "customer_account"},
		{Action: "read", Resource: "api_key"}, {Action: "read", Resource: "model"},
		{Action: "read", Resource: "channel"}, {Action: "read", Resource: "route"},
		{Action: "read", Resource: "pricing"}, {Action: "read", Resource: "limit"},
		{Action: "read", Resource: "snapshot"}, {Action: "read", Resource: "settlement"},
		{Action: "read", Resource: "callback"}, {Action: "read", Resource: "worker"},
		{Action: "read", Resource: "hold"}, {Action: "read", Resource: "usage"},
		{Action: "read", Resource: "task"}, {Action: "read", Resource: "playground"},
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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
