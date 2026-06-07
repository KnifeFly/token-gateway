package portal

import (
	"context"
	"encoding/json"
	"time"
)

// Principal is the browser-session customer scope for Portal Web BFF APIs.
type Principal struct {
	TenantID      string   `json:"tenant_id"`
	ProjectID     string   `json:"project_id"`
	APIKeyID      string   `json:"api_key_id"`
	AllowedModels []string `json:"allowed_models"`
}

// Session is the server-side Portal browser session record.
type Session struct {
	ID            string
	TenantID      string
	ProjectID     string
	APIKeyID      string
	AllowedModels []string
	CSRFHash      string
	UserAgent     string
	RemoteAddr    string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	LastSeenAt    time.Time
}

// SessionStore persists Portal browser sessions.
type SessionStore interface {
	Create(ctx context.Context, session Session) (Session, error)
	Get(ctx context.Context, sessionID string) (Session, bool, error)
	Touch(ctx context.Context, sessionID string, seenAt time.Time) error
	Revoke(ctx context.Context, sessionID string, revokedAt time.Time) (Session, bool, error)
	Delete(ctx context.Context, sessionID string) error
}

// ScopedSessionStore can revoke Portal sessions for one tenant/project/API key scope.
type ScopedSessionStore interface {
	RevokeByScope(ctx context.Context, tenantID string, projectID string, apiKeyID string, revokedAt time.Time) (int, error)
}

// APIKeyLoginRequest exchanges a customer API key for a browser session.
type APIKeyLoginRequest struct {
	APIKey string `json:"api_key"`
}

// LoginResponse returns browser session metadata and the CSRF token.
type LoginResponse struct {
	Authenticated bool            `json:"authenticated"`
	Session       SessionResponse `json:"session"`
	CSRFToken     string          `json:"csrf_token"`
}

// SessionResponse is safe session metadata returned to the browser.
type SessionResponse struct {
	SessionID     string    `json:"-"`
	Authenticated bool      `json:"authenticated"`
	TenantID      string    `json:"tenant_id,omitempty"`
	ProjectID     string    `json:"project_id,omitempty"`
	APIKeyID      string    `json:"api_key_id,omitempty"`
	AllowedModels []string  `json:"allowed_models,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	LastSeenAt    time.Time `json:"last_seen_at,omitempty"`
	CSRFToken     string    `json:"csrf_token,omitempty"`
}

// Dashboard summarizes customer self-service state for the Portal home view.
type Dashboard struct {
	GeneratedAt    time.Time        `json:"generated_at"`
	Credits        CreditsResponse  `json:"credits"`
	Usage          UsageResponse    `json:"usage"`
	APIKeyCount    int              `json:"api_key_count"`
	ActiveKeyCount int              `json:"active_key_count"`
	TaskSummary    TaskSummary      `json:"task_summary"`
	RecentTasks    []map[string]any `json:"recent_tasks"`
}

// TaskSummary counts task states visible to the current project.
type TaskSummary struct {
	Total      int `json:"total"`
	Queued     int `json:"queued"`
	Processing int `json:"processing"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
}

// OnboardingState tells the Portal UI which first-run steps are complete.
type OnboardingState struct {
	GeneratedAt time.Time        `json:"generated_at"`
	Steps       []OnboardingStep `json:"steps"`
}

// OnboardingStep is one first-run checklist item.
type OnboardingStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Complete    bool   `json:"complete"`
	Description string `json:"description,omitempty"`
}

// ProjectSettings contains safe project metadata for Portal settings.
type ProjectSettings struct {
	TenantID      string    `json:"tenant_id"`
	ProjectID     string    `json:"project_id"`
	APIKeyID      string    `json:"api_key_id"`
	AllowedModels []string  `json:"allowed_models"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// UsageFilter scopes Portal Web usage queries.
type UsageFilter struct {
	APIKeyID     string
	RequestID    string
	Model        string
	ProviderType string
	ChannelID    string
	Status       string
	Currency     string
	From         time.Time
	To           time.Time
	Limit        int
}

// TaskFilter scopes Portal Web task queries.
type TaskFilter struct {
	APIKeyID     string
	RequestID    string
	Model        string
	ProviderType string
	ChannelID    string
	Status       string
	Cursor       string
	From         time.Time
	To           time.Time
	Limit        int
}

// ModelListResponse is the Portal model list shape.
type ModelListResponse struct {
	Object string         `json:"object"`
	Data   []ModelSummary `json:"data"`
}

// ModelSummary is one customer-visible model.
type ModelSummary struct {
	ID               string              `json:"id"`
	Object           string              `json:"object"`
	Type             string              `json:"type"`
	Category         string              `json:"category,omitempty"`
	DisplayName      string              `json:"display_name"`
	Description      string              `json:"description,omitempty"`
	Aliases          []string            `json:"aliases,omitempty"`
	Tags             []string            `json:"tags,omitempty"`
	ProviderFamily   string              `json:"provider_family,omitempty"`
	Owner            string              `json:"owner"`
	Capabilities     []string            `json:"capabilities"`
	InputModalities  []string            `json:"input_modalities"`
	OutputModalities []string            `json:"output_modalities"`
	ContextWindow    int64               `json:"context_window,omitempty"`
	MaxOutputTokens  int64               `json:"max_output_tokens,omitempty"`
	Status           string              `json:"status,omitempty"`
	Async            bool                `json:"async"`
	Deprecated       bool                `json:"deprecated"`
	PricingSummary   ModelPricingSummary `json:"pricing_summary"`
}

// ModelPricingSummary is the customer-visible model price summary without ratio fields.
type ModelPricingSummary struct {
	Configured            bool                        `json:"configured"`
	Currency              string                      `json:"currency,omitempty"`
	Category              string                      `json:"category,omitempty"`
	Components            []ModelPricingComponentView `json:"components,omitempty"`
	InputMicrosPerToken   int64                       `json:"input_micros_per_token,omitempty"`
	OutputMicrosPerToken  int64                       `json:"output_micros_per_token,omitempty"`
	EstimatedOutputTokens int64                       `json:"estimated_output_tokens,omitempty"`
	ComponentPriceCount   int                         `json:"component_price_count"`
}

// ModelPricingComponentView is one customer-visible price component.
type ModelPricingComponentView struct {
	Unit          string `json:"unit"`
	MicrosPerUnit int64  `json:"micros_per_unit"`
}

// ModelDetailResponse returns a visible model summary and schema preview.
type ModelDetailResponse struct {
	Model  ModelSummary        `json:"model"`
	Schema ModelSchemaResponse `json:"schema"`
}

// ModelSchemaResponse returns the dynamic model schema for Portal clients.
type ModelSchemaResponse struct {
	Model   string         `json:"model"`
	Version string         `json:"version"`
	Schema  map[string]any `json:"schema"`
}

// PlaygroundRunRequest asks Portal Playground to validate and dry-run one customer-scoped payload.
type PlaygroundRunRequest struct {
	Model   string          `json:"model"`
	Mode    string          `json:"mode,omitempty"`
	Stream  bool            `json:"stream,omitempty"`
	Debug   bool            `json:"debug,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// PlaygroundRunResult returns safe customer-visible debug metadata.
type PlaygroundRunResult struct {
	RequestID     string                  `json:"request_id"`
	Scope         string                  `json:"scope"`
	Status        string                  `json:"status"`
	Message       string                  `json:"message"`
	Model         string                  `json:"model"`
	Mode          string                  `json:"mode"`
	Stream        bool                    `json:"stream"`
	PayloadFields []string                `json:"payload_fields"`
	Schema        PlaygroundSchemaSummary `json:"schema"`
	Debug         PlaygroundDebug         `json:"debug"`
	Result        PlaygroundSafeResult    `json:"result"`
	RanAt         time.Time               `json:"ran_at"`
}

// PlaygroundSchemaSummary summarizes schema-driven validation.
type PlaygroundSchemaSummary struct {
	Required        []string `json:"required"`
	AcceptedFields  []string `json:"accepted_fields"`
	MissingRequired []string `json:"missing_required,omitempty"`
}

// PlaygroundDebug is a redacted route and usage summary.
type PlaygroundDebug struct {
	RouteID          string          `json:"route_id,omitempty"`
	ChannelID        string          `json:"channel_id,omitempty"`
	ProviderType     string          `json:"provider_type,omitempty"`
	LatencyMillis    int64           `json:"latency_ms"`
	Usage            PlaygroundUsage `json:"usage"`
	SafeErrorCode    string          `json:"safe_error_code,omitempty"`
	SafeErrorMessage string          `json:"safe_error_message,omitempty"`
}

// PlaygroundUsage is a coarse safe token estimate for dry-run debug.
type PlaygroundUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// PlaygroundSafeResult is the sanitized dry-run result shown in Portal.
type PlaygroundSafeResult struct {
	Object  string `json:"object"`
	Summary string `json:"summary"`
}

// CreditsResponse is the customer-facing credits shape.
type CreditsResponse struct {
	Success bool                     `json:"success"`
	Message string                   `json:"message"`
	Data    map[string]CreditsBucket `json:"data"`
}

// CreditsBucket summarizes available, used, and held customer credits.
type CreditsBucket struct {
	RemainingCredits float64 `json:"remaining_credits"`
	UsedCredits      float64 `json:"used_credits"`
	HeldCredits      float64 `json:"held_credits,omitempty"`
	UnlimitedCredits bool    `json:"unlimited_credits"`
	Currency         string  `json:"currency"`
}

// CreditLedgerResponse returns safe customer ledger rows.
type CreditLedgerResponse struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Currency    string             `json:"currency"`
	Items       []CreditLedgerItem `json:"items"`
	NextCursor  *string            `json:"next_cursor"`
}

// CreditLedgerItem is one customer-visible ledger movement.
type CreditLedgerItem struct {
	ID                  string    `json:"id"`
	RequestID           string    `json:"request_id,omitempty"`
	SettlementKind      string    `json:"settlement_kind"`
	Currency            string    `json:"currency"`
	AmountCredits       float64   `json:"amount_credits"`
	BalanceAfterCredits float64   `json:"balance_after_credits"`
	Reason              string    `json:"reason,omitempty"`
	CreatedAt           time.Time `json:"created_at,omitempty"`
}

// UsageResponse is the Portal usage report shape.
type UsageResponse struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Currency    string      `json:"currency"`
	Totals      UsageTotals `json:"totals"`
	Items       []UsageItem `json:"items"`
	NextCursor  *string     `json:"next_cursor"`
}

// UsageTotals summarizes customer-visible token and credit usage.
type UsageTotals struct {
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CreditsUsed  float64 `json:"credits_used"`
}

// UsageItem is one customer-visible usage aggregate row.
type UsageItem struct {
	RequestID    string    `json:"request_id,omitempty"`
	APIKeyID     string    `json:"api_key_id,omitempty"`
	Model        string    `json:"model,omitempty"`
	Capability   string    `json:"capability,omitempty"`
	ProviderType string    `json:"provider_type,omitempty"`
	ChannelID    string    `json:"channel_id,omitempty"`
	Status       string    `json:"status"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	TotalTokens  int64     `json:"total_tokens,omitempty"`
	CreditsUsed  float64   `json:"credits_used"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

// UsageExportResponse returns safe Portal usage and ledger export rows.
type UsageExportResponse struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Format      string             `json:"format"`
	Filename    string             `json:"filename"`
	Currency    string             `json:"currency"`
	Totals      UsageTotals        `json:"totals"`
	Usage       []UsageItem        `json:"usage"`
	Ledger      []CreditLedgerItem `json:"ledger"`
	SafeFields  []string           `json:"safe_fields"`
}

// APIKey is safe Portal API key metadata.
type APIKey struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Enabled       bool        `json:"enabled"`
	AllowedModels []string    `json:"allowed_models"`
	IPAllowlist   []string    `json:"ip_allowlist,omitempty"`
	ExpiresAt     *time.Time  `json:"expires_at,omitempty"`
	LastUsedAt    *time.Time  `json:"last_used_at,omitempty"`
	UsageSummary  UsageTotals `json:"usage_summary"`
	CreatedAt     time.Time   `json:"created_at,omitempty"`
	RevokedAt     *time.Time  `json:"revoked_at,omitempty"`
}

// APIKeyListResponse is the Portal API key list shape.
type APIKeyListResponse struct {
	Data []APIKey `json:"data"`
}

// APIKeyCreateRequest requests a derived Portal API key.
type APIKeyCreateRequest struct {
	Name          string     `json:"name"`
	AllowedModels []string   `json:"allowed_models"`
	IPAllowlist   []string   `json:"ip_allowlist,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// APIKeyCreateResponse returns the derived API key and one-time plaintext.
type APIKeyCreateResponse struct {
	APIKey       APIKey `json:"api_key"`
	PlaintextKey string `json:"plaintext_key"`
}

// APIKeyRotateResponse returns the rotated API key and one-time plaintext.
type APIKeyRotateResponse struct {
	APIKey       APIKey `json:"api_key"`
	PlaintextKey string `json:"plaintext_key"`
}

// TaskListResponse is the Portal task list shape.
type TaskListResponse struct {
	Data       []map[string]any `json:"data"`
	NextCursor *string          `json:"next_cursor"`
}
