package reporting

import (
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
)

// TimeRange bounds report rows by creation time.
type TimeRange struct {
	From time.Time `json:"from,omitempty"`
	To   time.Time `json:"to,omitempty"`
}

// TenantUsageFilter scopes customer-facing balance, usage, and ledger reports.
type TenantUsageFilter struct {
	TenantID  string    `json:"tenant_id"`
	ProjectID string    `json:"project_id,omitempty"`
	APIKeyID  string    `json:"api_key_id,omitempty"`
	Currency  string    `json:"currency,omitempty"`
	From      time.Time `json:"from,omitempty"`
	To        time.Time `json:"to,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

// BalanceSummary is a customer balance bucket snapshot.
type BalanceSummary struct {
	AccountID       string    `json:"account_id"`
	TenantID        string    `json:"tenant_id"`
	ProjectID       string    `json:"project_id"`
	Currency        string    `json:"currency"`
	OpeningMicros   int64     `json:"opening_micros"`
	AvailableMicros int64     `json:"available_micros"`
	HeldMicros      int64     `json:"held_micros"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// UsageSummary is usage and customer revenue grouped by model/provider/channel.
type UsageSummary struct {
	Model        string `json:"model"`
	ProviderType string `json:"provider_type"`
	ChannelID    string `json:"channel_id"`
	Currency     string `json:"currency"`
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	TotalTokens  int64  `json:"total_tokens"`
	AmountMicros int64  `json:"amount_micros"`
}

// LedgerLine is an immutable account movement included in reports.
type LedgerLine struct {
	ID                 string    `json:"id"`
	RequestID          string    `json:"request_id"`
	SettlementKind     string    `json:"settlement_kind"`
	TenantID           string    `json:"tenant_id"`
	ProjectID          string    `json:"project_id"`
	AccountID          string    `json:"account_id"`
	Currency           string    `json:"currency"`
	AmountMicros       int64     `json:"amount_micros"`
	BalanceAfterMicros int64     `json:"balance_after_micros"`
	Reason             string    `json:"reason"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
}

// ReportTotals summarizes token, request, revenue, cost, and profit totals.
type ReportTotals struct {
	Requests           int64  `json:"requests"`
	InputTokens        int64  `json:"input_tokens"`
	OutputTokens       int64  `json:"output_tokens"`
	TotalTokens        int64  `json:"total_tokens"`
	RevenueMicros      int64  `json:"revenue_micros"`
	ProviderCostMicros int64  `json:"provider_cost_micros"`
	ProfitMicros       int64  `json:"profit_micros"`
	Currency           string `json:"currency,omitempty"`
}

// TenantUsageReport is the customer balance, usage, and ledger dashboard source.
type TenantUsageReport struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Filter      TenantUsageFilter `json:"filter"`
	Balances    []BalanceSummary  `json:"balances"`
	Usage       []UsageSummary    `json:"usage"`
	Ledger      []LedgerLine      `json:"ledger"`
	Totals      ReportTotals      `json:"totals"`
}

// ProviderCostProfile pins provider-side cost assumptions for reports.
type ProviderCostProfile struct {
	ID                    string              `json:"id"`
	ProviderType          string              `json:"provider_type"`
	ChannelID             string              `json:"channel_id"`
	PublicModel           string              `json:"public_model"`
	Category              string              `json:"category,omitempty"`
	Currency              string              `json:"currency"`
	Components            []pricing.Component `json:"components,omitempty"`
	InputMicrosPerToken   int64               `json:"input_micros_per_token"`
	OutputMicrosPerToken  int64               `json:"output_micros_per_token"`
	FixedMicrosPerRequest int64               `json:"fixed_micros_per_request"`
	EffectiveFrom         time.Time           `json:"effective_from,omitempty"`
	Enabled               bool                `json:"enabled"`
	CreatedAt             time.Time           `json:"created_at,omitempty"`
	UpdatedAt             time.Time           `json:"updated_at,omitempty"`
}

// ProviderProfitFilter scopes operator cost and profit reports.
type ProviderProfitFilter struct {
	TenantID  string    `json:"tenant_id,omitempty"`
	ProjectID string    `json:"project_id,omitempty"`
	From      time.Time `json:"from,omitempty"`
	To        time.Time `json:"to,omitempty"`
}

// ProviderProfitRow is one provider/channel/model profitability aggregate.
type ProviderProfitRow struct {
	ProviderType       string `json:"provider_type"`
	ChannelID          string `json:"channel_id"`
	Model              string `json:"model"`
	Currency           string `json:"currency"`
	Requests           int64  `json:"requests"`
	InputTokens        int64  `json:"input_tokens"`
	OutputTokens       int64  `json:"output_tokens"`
	TotalTokens        int64  `json:"total_tokens"`
	RevenueMicros      int64  `json:"revenue_micros"`
	ProviderCostMicros int64  `json:"provider_cost_micros"`
	ProfitMicros       int64  `json:"profit_micros"`
	CostProfileMissing bool   `json:"cost_profile_missing"`
}

// ProviderProfitReport is the operator-facing cost, revenue, and profit report.
type ProviderProfitReport struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Filter      ProviderProfitFilter `json:"filter"`
	Rows        []ProviderProfitRow  `json:"rows"`
	Totals      ReportTotals         `json:"totals"`
}

// FailedSettlementSummary is a safe failed-settlement tracking row.
type FailedSettlementSummary struct {
	ID          string    `json:"id"`
	RequestID   string    `json:"request_id"`
	TenantID    string    `json:"tenant_id"`
	ProjectID   string    `json:"project_id"`
	HoldID      string    `json:"hold_id"`
	Status      string    `json:"status"`
	RetryCount  int       `json:"retry_count"`
	NextRetryAt time.Time `json:"next_retry_at,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	OwnerID     string    `json:"owner_id,omitempty"`
	ClaimedAt   time.Time `json:"claimed_at,omitempty"`
	HeartbeatAt time.Time `json:"heartbeat_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// UsageAttemptSummary is an operator-safe provider attempt audit row.
type UsageAttemptSummary struct {
	RequestID             string    `json:"request_id"`
	TaskID                string    `json:"task_id,omitempty"`
	TenantID              string    `json:"tenant_id"`
	ProjectID             string    `json:"project_id"`
	APIKeyID              string    `json:"api_key_id"`
	AttemptIndex          int       `json:"attempt_index"`
	ProviderType          string    `json:"provider_type"`
	ChannelID             string    `json:"channel_id"`
	Model                 string    `json:"model"`
	UpstreamModel         string    `json:"upstream_model,omitempty"`
	StatusCode            int       `json:"status_code"`
	ErrorCode             string    `json:"error_code,omitempty"`
	Success               bool      `json:"success"`
	Retryable             bool      `json:"retryable"`
	FallbackFromChannelID string    `json:"fallback_from_channel_id,omitempty"`
	Final                 bool      `json:"final"`
	CreatedAt             time.Time `json:"created_at,omitempty"`
}

// BudgetSemantics explains how admission budget guard and actual spend differ.
type BudgetSemantics struct {
	AdmissionGuardField string `json:"admission_guard_field"`
	ActualSpendSource   string `json:"actual_spend_source"`
	Notes               string `json:"notes"`
}

// ReconciliationReport combines balance/ledger mismatches with failed settlements.
type ReconciliationReport struct {
	GeneratedAt       time.Time                     `json:"generated_at"`
	Issues            []billing.ReconciliationIssue `json:"issues"`
	FailedSettlements []FailedSettlementSummary     `json:"failed_settlements"`
	UsageAttempts     []UsageAttemptSummary         `json:"usage_attempts,omitempty"`
	BudgetSemantics   BudgetSemantics               `json:"budget_semantics"`
}

// ManualAdjustmentRequest requests an idempotent operator balance correction.
type ManualAdjustmentRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	TenantID       string `json:"tenant_id"`
	ProjectID      string `json:"project_id"`
	Currency       string `json:"currency"`
	AmountMicros   int64  `json:"amount_micros"`
	Reason         string `json:"reason"`
	OperatorID     string `json:"operator_id"`
}

// ManualAdjustment is the durable audit row for a manual account movement.
type ManualAdjustment struct {
	ID             string    `json:"id"`
	IdempotencyKey string    `json:"idempotency_key"`
	TenantID       string    `json:"tenant_id"`
	ProjectID      string    `json:"project_id"`
	AccountID      string    `json:"account_id"`
	LedgerEntryID  string    `json:"ledger_entry_id"`
	Currency       string    `json:"currency"`
	AmountMicros   int64     `json:"amount_micros"`
	Reason         string    `json:"reason"`
	OperatorID     string    `json:"operator_id"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
}

// AgentMetadataFilter scopes workflow/scene/shot reports.
type AgentMetadataFilter struct {
	TenantID  string    `json:"tenant_id,omitempty"`
	ProjectID string    `json:"project_id,omitempty"`
	From      time.Time `json:"from,omitempty"`
	To        time.Time `json:"to,omitempty"`
}

// AgentMetadataRow aggregates async agent task metadata without prompt/response data.
type AgentMetadataRow struct {
	Workflow     string `json:"workflow"`
	Scene        string `json:"scene"`
	Shot         string `json:"shot"`
	Kind         string `json:"kind"`
	MediaType    string `json:"media_type"`
	Model        string `json:"model"`
	Tasks        int64  `json:"tasks"`
	Succeeded    int64  `json:"succeeded"`
	Failed       int64  `json:"failed"`
	AmountMicros int64  `json:"amount_micros"`
	Currency     string `json:"currency,omitempty"`
}

// AgentMetadataReport groups task metadata for workflow, scene, and shot analytics.
type AgentMetadataReport struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Filter      AgentMetadataFilter `json:"filter"`
	Rows        []AgentMetadataRow  `json:"rows"`
}
