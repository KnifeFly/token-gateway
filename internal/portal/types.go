package portal

import "time"

// Principal is the authenticated customer scope for Portal APIs.
type Principal struct {
	TenantID      string
	ProjectID     string
	APIKeyID      string
	AllowedModels []string
}

// ModelListResponse is the Portal model list shape.
type ModelListResponse struct {
	Object string         `json:"object"`
	Data   []ModelSummary `json:"data"`
}

// ModelSummary is one customer-visible model.
type ModelSummary struct {
	ID               string   `json:"id"`
	Object           string   `json:"object"`
	Type             string   `json:"type"`
	Category         string   `json:"category,omitempty"`
	DisplayName      string   `json:"display_name"`
	Description      string   `json:"description,omitempty"`
	Aliases          []string `json:"aliases,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	ProviderFamily   string   `json:"provider_family,omitempty"`
	Owner            string   `json:"owner"`
	Capabilities     []string `json:"capabilities"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	ContextWindow    int64    `json:"context_window,omitempty"`
	MaxOutputTokens  int64    `json:"max_output_tokens,omitempty"`
	Status           string   `json:"status,omitempty"`
	Async            bool     `json:"async"`
	Deprecated       bool     `json:"deprecated"`
}

// ModelSchemaResponse returns the dynamic model schema for Portal clients.
type ModelSchemaResponse struct {
	Model   string         `json:"model"`
	Version string         `json:"version"`
	Schema  map[string]any `json:"schema"`
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
	Status       string    `json:"status"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	CreditsUsed  float64   `json:"credits_used"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

// APIKey is safe Portal API key metadata.
type APIKey struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Enabled       bool       `json:"enabled"`
	AllowedModels []string   `json:"allowed_models"`
	CreatedAt     time.Time  `json:"created_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

// APIKeyListResponse is the Portal API key list shape.
type APIKeyListResponse struct {
	Data []APIKey `json:"data"`
}

// APIKeyCreateRequest requests a derived Portal API key.
type APIKeyCreateRequest struct {
	Name          string   `json:"name"`
	AllowedModels []string `json:"allowed_models"`
}

// APIKeyCreateResponse returns the derived API key and one-time plaintext.
type APIKeyCreateResponse struct {
	APIKey       APIKey `json:"api_key"`
	PlaintextKey string `json:"plaintext_key"`
}

// TaskListResponse is the Portal task list shape.
type TaskListResponse struct {
	Data       []map[string]any `json:"data"`
	NextCursor *string          `json:"next_cursor"`
}
