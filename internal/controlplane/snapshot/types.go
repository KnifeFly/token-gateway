package snapshot

import (
	"encoding/json"
	"time"
)

// RuntimeSnapshot is the control-plane output consumed by data-plane indexes.
type RuntimeSnapshot struct {
	Version        string
	SchemaVersion  string
	Checksum       string
	CreatedAt      time.Time
	APIKeys        []APIKeyRuntime
	Models         []ModelRuntime
	Channels       []ChannelRuntime
	RoutePolicies  []RoutePolicyRuntime
	PriceRules     []PriceRuleRuntime
	LimitRules     []LimitRuleRuntime
	PluginBindings []PluginBindingRuntime
	RevokedKeys    []RevokedKeyRuntime
}

// APIKeyRuntime contains no plaintext API key, only the stable hash.
type APIKeyRuntime struct {
	ID            string
	TenantID      string
	ProjectID     string
	Name          string
	KeyHash       string
	Enabled       bool
	AllowedModels []string
}

// ModelRuntime describes one public model.
type ModelRuntime struct {
	PublicModel      string
	Aliases          []string
	DisplayName      string
	Description      string
	Protocol         string
	Capability       string
	Schema           json.RawMessage
	ProviderMappings []ProviderModelMappingRuntime
	Enabled          bool
}

// ProviderModelMappingRuntime maps a catalog model to one provider channel.
type ProviderModelMappingRuntime struct {
	ProviderType  string
	ChannelID     string
	PublicModel   string
	UpstreamModel string
}

// ChannelRuntime describes one provider channel.
type ChannelRuntime struct {
	ID              string
	ProviderType    string
	BaseURL         string
	APIKey          string
	CredentialRef   string
	EncryptedAPIKey string
	Enabled         bool
	Timeout         time.Duration
	Models          []ChannelModelRuntime
}

// ChannelModelRuntime maps a public model to the provider model.
type ChannelModelRuntime struct {
	PublicModel   string
	UpstreamModel string
}

// RoutePolicyRuntime describes the candidate set for a public model.
type RoutePolicyRuntime struct {
	ID          string
	PublicModel string
	Strategy    string
	Candidates  []RouteCandidateRuntime
}

// RouteCandidateRuntime points to one provider channel candidate.
type RouteCandidateRuntime struct {
	ChannelID string
	Priority  int
	Weight    int
}

// PriceRuleRuntime pins customer-facing price for one public model.
type PriceRuleRuntime struct {
	PublicModel           string
	Currency              string
	InputMicrosPerToken   int64
	OutputMicrosPerToken  int64
	EstimatedOutputTokens int64
	Enabled               bool
}

// LimitRuleRuntime pins request limits for one multi-dimensional scope.
type LimitRuleRuntime struct {
	ID                  string
	TenantID            string
	ProjectID           string
	APIKeyID            string
	PublicModel         string
	ProviderType        string
	ChannelID           string
	RPM                 int64
	QPS                 int64
	TPM                 int64
	Concurrency         int64
	DailyBudgetMicros   int64
	CostPerMinuteMicros int64
	Enabled             bool
}

// PluginBindingRuntime is a validated data-plane plugin binding.
type PluginBindingRuntime struct {
	ID            string
	Name          string
	Phase         string
	TenantID      string
	ProjectID     string
	Model         string
	Priority      int
	Enabled       bool
	FailurePolicy string
	Config        json.RawMessage
}

// RevokedKeyRuntime records fast API key revocations included in snapshots.
type RevokedKeyRuntime struct {
	KeyHash   string
	RevokedAt time.Time
}
