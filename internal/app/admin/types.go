package admin

import (
	"context"
	"encoding/json"
	"time"
)

// Role identifies an Admin operator role.
type Role string

const (
	// RoleSuperAdmin can perform every Admin Web action.
	RoleSuperAdmin Role = "super_admin"
	// RoleConfigAdmin can manage control-plane configuration and snapshots.
	RoleConfigAdmin Role = "config_admin"
	// RoleFinanceAdmin can inspect and repair commercial operations.
	RoleFinanceAdmin Role = "finance_admin"
	// RoleSupport can inspect customer and task state without mutations.
	RoleSupport Role = "support"
	// RoleOps can inspect and repair worker, callback, and settlement operations.
	RoleOps Role = "ops"
	// RoleReadOnly can inspect dashboard and configuration read models.
	RoleReadOnly Role = "read_only"
)

// Permission identifies one action/resource authorization check.
type Permission struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
}

// Actor is the authenticated Admin operator principal.
type Actor struct {
	OperatorID  string `json:"operator_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	Roles       []Role `json:"roles"`
}

// Operator is the durable Admin operator account.
type Operator struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"display_name,omitempty"`
	PasswordHash string     `json:"-"`
	Roles        []Role     `json:"roles"`
	Enabled      bool       `json:"enabled"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at,omitempty"`
}

// OperatorCreateRequest creates a browser Admin operator account.
type OperatorCreateRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	Password    string `json:"password"`
	Roles       []Role `json:"roles"`
	Enabled     bool   `json:"enabled"`
}

// Session is the server-side Admin browser session record.
type Session struct {
	ID            string
	OperatorID    string
	CSRFHash      string
	UserAgentHash string
	RemoteAddr    string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	LastSeenAt    time.Time
}

// OperatorStore persists Admin operators.
type OperatorStore interface {
	SaveOperator(ctx context.Context, operator Operator) (Operator, error)
	GetOperator(ctx context.Context, operatorID string) (Operator, bool, error)
	GetOperatorByEmail(ctx context.Context, email string) (Operator, bool, error)
	ListOperators(ctx context.Context) ([]Operator, error)
	DisableOperator(ctx context.Context, operatorID string, disabledAt time.Time) (Operator, bool, error)
	UpdateOperatorLastLogin(ctx context.Context, operatorID string, seenAt time.Time) error
}

// SessionStore persists Admin browser sessions.
type SessionStore interface {
	CreateSession(ctx context.Context, session Session) (Session, error)
	GetSession(ctx context.Context, sessionID string) (Session, bool, error)
	TouchSession(ctx context.Context, sessionID string, seenAt time.Time) error
	RevokeSession(ctx context.Context, sessionID string, revokedAt time.Time) (Session, bool, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// AuditStore persists Admin mutation audit events.
type AuditStore interface {
	CreateAuditEvent(ctx context.Context, event AuditEvent) (AuditEvent, error)
	ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, error)
}

// Repository combines Admin account, session, and audit storage.
type Repository interface {
	OperatorStore
	SessionStore
	AuditStore
}

// AuditEvent records one Admin mutation attempt with redacted payloads.
type AuditEvent struct {
	ID             string          `json:"id"`
	Actor          Actor           `json:"actor"`
	Action         string          `json:"action"`
	Resource       string          `json:"resource"`
	ResourceID     string          `json:"resource_id,omitempty"`
	RequestID      string          `json:"request_id"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	Status         string          `json:"status"`
	ErrorCode      string          `json:"error_code,omitempty"`
	Before         json.RawMessage `json:"before,omitempty"`
	After          json.RawMessage `json:"after,omitempty"`
	RemoteAddr     string          `json:"remote_addr,omitempty"`
	UserAgentHash  string          `json:"user_agent_hash,omitempty"`
	CreatedAt      time.Time       `json:"created_at,omitempty"`
}

// AuditFilter scopes Admin audit queries.
type AuditFilter struct {
	OperatorID string
	Action     string
	Resource   string
	From       time.Time
	To         time.Time
	Limit      int
}

// MutationOptions carries browser mutation control metadata.
type MutationOptions struct {
	RequestID      string
	IdempotencyKey string
	Reason         string
	RemoteAddr     string
	UserAgent      string
}

// LoginRequest authenticates an Admin operator browser session.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse returns safe Admin session metadata and a CSRF token.
type LoginResponse struct {
	Authenticated bool            `json:"authenticated"`
	Session       SessionResponse `json:"session"`
	CSRFToken     string          `json:"csrf_token"`
}

// SessionResponse is safe session metadata returned to the Admin browser.
type SessionResponse struct {
	SessionID     string       `json:"-"`
	Authenticated bool         `json:"authenticated"`
	OperatorID    string       `json:"operator_id,omitempty"`
	Email         string       `json:"email,omitempty"`
	DisplayName   string       `json:"display_name,omitempty"`
	Roles         []Role       `json:"roles,omitempty"`
	Permissions   []Permission `json:"permissions,omitempty"`
	ExpiresAt     time.Time    `json:"expires_at,omitempty"`
	LastSeenAt    time.Time    `json:"last_seen_at,omitempty"`
	CSRFToken     string       `json:"csrf_token,omitempty"`
}

// Dashboard summarizes Admin-visible operational state.
type Dashboard struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Counts      DashboardCounts `json:"counts"`
}

// DashboardCounts contains Admin home counters.
type DashboardCounts struct {
	Tenants           int `json:"tenants"`
	Projects          int `json:"projects"`
	APIKeys           int `json:"api_keys"`
	Models            int `json:"models"`
	Channels          int `json:"channels"`
	Routes            int `json:"routes"`
	PricingRules      int `json:"pricing_rules"`
	LimitRules        int `json:"limit_rules"`
	Tasks             int `json:"tasks"`
	FailedSettlements int `json:"failed_settlements"`
	DueCallbacks      int `json:"due_callbacks"`
}

// APIKeyView is safe Admin API key metadata without hashes or plaintext.
type APIKeyView struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	ProjectID     string     `json:"project_id"`
	Name          string     `json:"name"`
	Enabled       bool       `json:"enabled"`
	AllowedModels []string   `json:"allowed_models,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at,omitempty"`
}

// ChannelView is safe provider channel metadata without credential material.
type ChannelView struct {
	ID                   string             `json:"id"`
	ProviderType         string             `json:"provider_type"`
	BaseURL              string             `json:"base_url"`
	CredentialConfigured bool               `json:"credential_configured"`
	Enabled              bool               `json:"enabled"`
	TimeoutMillis        int64              `json:"timeout_millis,omitempty"`
	ModelCount           int                `json:"model_count"`
	HealthStatus         string             `json:"health_status,omitempty"`
	TestStatus           string             `json:"test_status,omitempty"`
	CostConfigStatus     string             `json:"cost_config_status,omitempty"`
	RoutePolicyHints     []RoutePolicyHint  `json:"route_policy_hints,omitempty"`
	Models               []ChannelModelView `json:"models,omitempty"`
}

// ChannelModelView is safe channel-model mapping metadata.
type ChannelModelView struct {
	PublicModel         string          `json:"public_model"`
	UpstreamModel       string          `json:"upstream_model"`
	Capabilities        []string        `json:"capabilities,omitempty"`
	SupportedParameters []string        `json:"supported_parameters,omitempty"`
	HealthStatus        string          `json:"health_status,omitempty"`
	TestStatus          string          `json:"test_status,omitempty"`
	CostConfigStatus    string          `json:"cost_config_status,omitempty"`
	Metadata            json.RawMessage `json:"metadata,omitempty"`
}

// RoutePolicyHint is a safe read-only route candidate summary for one channel.
type RoutePolicyHint struct {
	RouteID     string `json:"route_id"`
	PublicModel string `json:"public_model"`
	Strategy    string `json:"strategy,omitempty"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
	Weight      int    `json:"weight"`
}

// ChannelHealthEvent is a synthetic safe channel health read model.
type ChannelHealthEvent struct {
	ID         string    `json:"id"`
	ChannelID  string    `json:"channel_id"`
	Status     string    `json:"status"`
	Source     string    `json:"source"`
	Message    string    `json:"message"`
	ObservedAt time.Time `json:"observed_at"`
}

// ChannelTestResult is a safe result for Admin channel test workflow.
type ChannelTestResult struct {
	ChannelID            string    `json:"channel_id"`
	Status               string    `json:"status"`
	Message              string    `json:"message"`
	CredentialConfigured bool      `json:"credential_configured"`
	ModelCount           int       `json:"model_count"`
	TestedAt             time.Time `json:"tested_at"`
}

// ChannelSyncApplyResult summarizes persisted channel model sync changes.
type ChannelSyncApplyResult struct {
	ChannelID string      `json:"channel_id"`
	AppliedAt time.Time   `json:"applied_at"`
	Preview   any         `json:"preview"`
	Channel   ChannelView `json:"channel"`
}

// SnapshotSummary describes active and rollback runtime snapshot state.
type SnapshotSummary struct {
	Active   any `json:"active,omitempty"`
	Previous any `json:"previous,omitempty"`
}

// SnapshotOperationResult is a safe publish/rollback response.
type SnapshotOperationResult struct {
	Version       string    `json:"version"`
	Checksum      string    `json:"checksum,omitempty"`
	SchemaVersion string    `json:"schema_version,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

// CallbackEventView is safe callback outbox metadata without payload or URL.
type CallbackEventView struct {
	ID             string    `json:"id"`
	TaskID         string    `json:"task_id"`
	TenantID       string    `json:"tenant_id"`
	ProjectID      string    `json:"project_id"`
	Status         string    `json:"status"`
	RetryCount     int       `json:"retry_count"`
	NextRetryAt    time.Time `json:"next_retry_at,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	OwnerID        string    `json:"owner_id,omitempty"`
	LastStatusCode int       `json:"last_status_code,omitempty"`
	LastLatencyMS  int64     `json:"last_latency_ms,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// WorkerJobView is a read-only worker job status placeholder for P22 UI.
type WorkerJobView struct {
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen,omitempty"`
}

// HoldAgingView is safe hold-aging metadata for operations dashboards.
type HoldAgingView struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// ReplayResult summarizes an operator-triggered repair attempt.
type ReplayResult struct {
	RequestedID string `json:"requested_id,omitempty"`
	Replayed    int    `json:"replayed"`
}

// ListResponse wraps collection responses for generated clients.
type ListResponse[T any] struct {
	Data []T `json:"data"`
}
