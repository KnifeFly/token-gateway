package snapshot

import "time"

// RuntimeSnapshot is the control-plane output consumed by data-plane indexes.
type RuntimeSnapshot struct {
	Version       string
	SchemaVersion string
	Checksum      string
	CreatedAt     time.Time
	APIKeys       []APIKeyRuntime
	Models        []ModelRuntime
	Channels      []ChannelRuntime
	RoutePolicies []RoutePolicyRuntime
	PriceRules    []PriceRuleRuntime
	LimitRules    []LimitRuleRuntime
	RevokedKeys   []RevokedKeyRuntime
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
	PublicModel string
	Protocol    string
	Capability  string
	Enabled     bool
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

// LimitRuleRuntime pins request limits for one public model.
type LimitRuleRuntime struct {
	PublicModel string
	QPS         int64
	TPM         int64
	Concurrency int64
	Enabled     bool
}

// RevokedKeyRuntime records fast API key revocations included in snapshots.
type RevokedKeyRuntime struct {
	KeyHash   string
	RevokedAt time.Time
}
