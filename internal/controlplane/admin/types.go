package admin

import (
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Tenant is a customer account boundary.
type Tenant struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Enabled    bool      `json:"enabled"`
	EnabledSet bool      `json:"-"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// Project groups API keys and usage under one tenant.
type Project struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Name       string    `json:"name"`
	Enabled    bool      `json:"enabled"`
	EnabledSet bool      `json:"-"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// APIKey stores only the hashed customer key.
type APIKey struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	ProjectID     string     `json:"project_id"`
	Name          string     `json:"name"`
	KeyHash       string     `json:"key_hash"`
	PlaintextKey  string     `json:"plaintext_key,omitempty"`
	Enabled       bool       `json:"enabled"`
	AllowedModels []string   `json:"allowed_models"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at,omitempty"`
}

// ModelConfig is a public model advertised by the gateway.
type ModelConfig struct {
	PublicModel string          `json:"public_model"`
	Aliases     []string        `json:"aliases,omitempty"`
	DisplayName string          `json:"display_name,omitempty"`
	Description string          `json:"description,omitempty"`
	Protocol    string          `json:"protocol"`
	Capability  string          `json:"capability"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Enabled     bool            `json:"enabled"`
	EnabledSet  bool            `json:"-"`
}

// ChannelConfig is a provider channel and encrypted credential reference.
type ChannelConfig struct {
	ID              string         `json:"id"`
	ProviderType    string         `json:"provider_type"`
	BaseURL         string         `json:"base_url"`
	APIKey          string         `json:"api_key,omitempty"`
	CredentialRef   string         `json:"credential_ref"`
	EncryptedAPIKey string         `json:"encrypted_api_key,omitempty"`
	Enabled         bool           `json:"enabled"`
	EnabledSet      bool           `json:"-"`
	Timeout         time.Duration  `json:"timeout,omitempty"`
	TimeoutMillis   int64          `json:"timeout_millis,omitempty"`
	Models          []ChannelModel `json:"models"`
}

// ChannelModel maps a public model to one upstream model.
type ChannelModel struct {
	PublicModel   string `json:"public_model"`
	UpstreamModel string `json:"upstream_model"`
}

// RoutePolicyConfig is the route candidate set for one public model.
type RoutePolicyConfig struct {
	ID          string           `json:"id"`
	PublicModel string           `json:"public_model"`
	Strategy    string           `json:"strategy"`
	Enabled     bool             `json:"enabled"`
	EnabledSet  bool             `json:"-"`
	Candidates  []RouteCandidate `json:"candidates"`
}

// RouteCandidate points a route policy at one channel.
type RouteCandidate struct {
	ChannelID string `json:"channel_id"`
	Priority  int    `json:"priority"`
	Weight    int    `json:"weight"`
}

// PriceRuleConfig pins customer-facing price for one model.
type PriceRuleConfig struct {
	PublicModel           string `json:"public_model"`
	Currency              string `json:"currency"`
	InputMicrosPerToken   int64  `json:"input_micros_per_token"`
	OutputMicrosPerToken  int64  `json:"output_micros_per_token"`
	EstimatedOutputTokens int64  `json:"estimated_output_tokens"`
	Enabled               bool   `json:"enabled"`
	EnabledSet            bool   `json:"-"`
}

// LimitRuleConfig pins request limits for one multi-dimensional scope.
type LimitRuleConfig struct {
	ID                         string `json:"id"`
	TenantID                   string `json:"tenant_id,omitempty"`
	ProjectID                  string `json:"project_id,omitempty"`
	APIKeyID                   string `json:"api_key_id,omitempty"`
	PublicModel                string `json:"public_model,omitempty"`
	ProviderType               string `json:"provider_type,omitempty"`
	ChannelID                  string `json:"channel_id,omitempty"`
	RPM                        int64  `json:"rpm,omitempty"`
	QPS                        int64  `json:"qps,omitempty"`
	TPM                        int64  `json:"tpm,omitempty"`
	Concurrency                int64  `json:"concurrency,omitempty"`
	DailyAdmissionBudgetMicros int64  `json:"daily_admission_budget_micros,omitempty"`
	DailyBudgetMicros          int64  `json:"daily_budget_micros,omitempty"`
	CostPerMinuteMicros        int64  `json:"cost_per_minute_micros,omitempty"`
	Enabled                    bool   `json:"enabled"`
	EnabledSet                 bool   `json:"-"`
}

// PluginBindingConfig binds one built-in plugin to a runtime phase and scope.
type PluginBindingConfig struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Phase         string          `json:"phase"`
	TenantID      string          `json:"tenant_id,omitempty"`
	ProjectID     string          `json:"project_id,omitempty"`
	Model         string          `json:"model,omitempty"`
	Priority      int             `json:"priority"`
	Enabled       bool            `json:"enabled"`
	EnabledSet    bool            `json:"-"`
	FailurePolicy string          `json:"failure_policy"`
	Config        json.RawMessage `json:"config"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at,omitempty"`
}

// ModelMarketplaceConfig controls which models are visible to a tenant or project.
type ModelMarketplaceConfig struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id,omitempty"`
	ProjectID   string          `json:"project_id,omitempty"`
	PublicModel string          `json:"public_model"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description,omitempty"`
	Enabled     bool            `json:"enabled"`
	EnabledSet  bool            `json:"-"`
	SortOrder   int             `json:"sort_order"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at,omitempty"`
	UpdatedAt   time.Time       `json:"updated_at,omitempty"`
}

// VisibleModel is a tenant-facing model marketplace row.
type VisibleModel struct {
	ID                    string          `json:"id"`
	TenantID              string          `json:"tenant_id,omitempty"`
	ProjectID             string          `json:"project_id,omitempty"`
	PublicModel           string          `json:"public_model"`
	DisplayName           string          `json:"display_name"`
	Description           string          `json:"description,omitempty"`
	Protocol              string          `json:"protocol"`
	Capability            string          `json:"capability"`
	Currency              string          `json:"currency,omitempty"`
	InputMicrosPerToken   int64           `json:"input_micros_per_token,omitempty"`
	OutputMicrosPerToken  int64           `json:"output_micros_per_token,omitempty"`
	EstimatedOutputTokens int64           `json:"estimated_output_tokens,omitempty"`
	SortOrder             int             `json:"sort_order"`
	Metadata              json.RawMessage `json:"metadata"`
}

// SnapshotRecord stores one published runtime snapshot payload.
type SnapshotRecord struct {
	Version   string     `json:"version"`
	Checksum  string     `json:"checksum"`
	Status    string     `json:"status"`
	Payload   []byte     `json:"payload,omitempty"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at,omitempty"`
	ActiveAt  *time.Time `json:"active_at,omitempty"`
}

// SnapshotConfig is the normalized admin config used by snapshot builder.
type SnapshotConfig struct {
	APIKeys     []APIKey
	Models      []ModelConfig
	Channels    []ChannelConfig
	Routes      []RoutePolicyConfig
	Prices      []PriceRuleConfig
	Limits      []LimitRuleConfig
	Plugins     []PluginBindingConfig
	RevokedKeys []APIKey
}

// UnmarshalJSON records whether enabled was explicitly present.
func (t *Tenant) UnmarshalJSON(data []byte) error {
	type alias Tenant
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*t = Tenant(value)
	t.EnabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

// UnmarshalJSON records whether enabled was explicitly present.
func (p *Project) UnmarshalJSON(data []byte) error {
	type alias Project
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*p = Project(value)
	p.EnabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

// UnmarshalJSON records whether enabled was explicitly present.
func (m *ModelConfig) UnmarshalJSON(data []byte) error {
	type alias ModelConfig
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*m = ModelConfig(value)
	m.EnabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

// UnmarshalJSON records whether enabled was explicitly present.
func (c *ChannelConfig) UnmarshalJSON(data []byte) error {
	type alias ChannelConfig
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*c = ChannelConfig(value)
	c.EnabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

// UnmarshalJSON records whether enabled was explicitly present.
func (r *RoutePolicyConfig) UnmarshalJSON(data []byte) error {
	type alias RoutePolicyConfig
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*r = RoutePolicyConfig(value)
	r.EnabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

// UnmarshalJSON records whether enabled was explicitly present.
func (p *PriceRuleConfig) UnmarshalJSON(data []byte) error {
	type alias PriceRuleConfig
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*p = PriceRuleConfig(value)
	p.EnabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

// UnmarshalJSON records whether enabled was explicitly present.
func (l *LimitRuleConfig) UnmarshalJSON(data []byte) error {
	type alias LimitRuleConfig
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	newField := jsonFieldPresent(data, "daily_admission_budget_micros")
	oldField := jsonFieldPresent(data, "daily_budget_micros")
	if newField && oldField && value.DailyAdmissionBudgetMicros != value.DailyBudgetMicros {
		return apperr.InvalidArgument("daily_admission_budget_micros and daily_budget_micros must match when both are provided")
	}
	if newField && !oldField {
		value.DailyBudgetMicros = value.DailyAdmissionBudgetMicros
	}
	if oldField && !newField {
		value.DailyAdmissionBudgetMicros = value.DailyBudgetMicros
	}
	*l = LimitRuleConfig(value)
	l.EnabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

// UnmarshalJSON records whether enabled was explicitly present.
func (b *PluginBindingConfig) UnmarshalJSON(data []byte) error {
	type alias PluginBindingConfig
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*b = PluginBindingConfig(value)
	b.EnabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

// UnmarshalJSON records whether enabled was explicitly present.
func (m *ModelMarketplaceConfig) UnmarshalJSON(data []byte) error {
	type alias ModelMarketplaceConfig
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*m = ModelMarketplaceConfig(value)
	m.EnabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

func jsonFieldPresent(data []byte, field string) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	_, ok := raw[field]
	return ok
}
