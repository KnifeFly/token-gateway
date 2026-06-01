package portal

import (
	"context"
	"time"

	legacyportal "github.com/KnifeFly/token-gateway/internal/portal"
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
	GeneratedAt    time.Time                    `json:"generated_at"`
	Credits        legacyportal.CreditsResponse `json:"credits"`
	Usage          legacyportal.UsageResponse   `json:"usage"`
	APIKeyCount    int                          `json:"api_key_count"`
	ActiveKeyCount int                          `json:"active_key_count"`
	TaskSummary    TaskSummary                  `json:"task_summary"`
	RecentTasks    []map[string]any             `json:"recent_tasks"`
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
	Currency string
	From     time.Time
	To       time.Time
	Limit    int
}
