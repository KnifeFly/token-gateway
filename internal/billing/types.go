package billing

import (
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/money"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const (
	// HoldStatusActive means reserved balance has not been released or settled.
	HoldStatusActive = "active"
	// HoldStatusReleased means reserved balance was returned without settlement.
	HoldStatusReleased = "released"
	// HoldStatusSettled means reserved balance has been finalized into usage.
	HoldStatusSettled = "settled"

	// FailedSettlementPending marks repair records waiting for first replay.
	FailedSettlementPending = "pending"
	// FailedSettlementReplayed marks repair records successfully replayed.
	FailedSettlementReplayed = "replayed"
	// FailedSettlementFailed marks repair records whose replay failed and should retry.
	FailedSettlementFailed = "failed"
	// FailedSettlementProcessing marks repair records claimed by one worker.
	FailedSettlementProcessing = "processing"
)

// BalanceAccount is a tenant/project balance bucket.
type BalanceAccount struct {
	ID              string
	TenantID        string
	ProjectID       string
	Currency        string
	OpeningMicros   int64
	AvailableMicros int64
	HeldMicros      int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// HoldRequest asks the balance service to reserve money.
type HoldRequest struct {
	RequestID    string
	TenantID     string
	ProjectID    string
	APIKeyID     string
	Currency     string
	AmountMicros int64
	ExpiresAt    time.Time
}

// BalanceHold reserves balance before a provider call.
type BalanceHold struct {
	ID            string
	RequestID     string
	TenantID      string
	ProjectID     string
	APIKeyID      string
	AccountID     string
	Currency      string
	AmountMicros  int64
	Status        string
	ReleaseReason string
	ExpiresAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UsageAttempt records one provider attempt.
type UsageAttempt struct {
	ID                    string
	RequestID             string
	AttemptIndex          int
	TaskID                string
	TenantID              string
	ProjectID             string
	APIKeyID              string
	ChannelID             string
	ProviderType          string
	Model                 string
	UpstreamModel         string
	StatusCode            int
	ErrorCode             string
	Success               bool
	EstimatedInputTokens  int64
	EstimatedOutputTokens int64
	ActualInputTokens     int64
	ActualOutputTokens    int64
	Retryable             bool
	RetryBudgetConsumed   int
	RetryBudgetRemaining  int
	FallbackFromChannelID string
	FallbackFromProvider  string
	CircuitState          string
	Final                 bool
	CreatedAt             time.Time
}

// SettlementPlan contains the durable inputs for final settlement.
type SettlementPlan struct {
	RequestID      string            `json:"request_id"`
	TenantID       string            `json:"tenant_id"`
	ProjectID      string            `json:"project_id"`
	APIKeyID       string            `json:"api_key_id"`
	HoldID         string            `json:"hold_id"`
	Model          string            `json:"model"`
	ProviderType   string            `json:"provider_type"`
	ChannelID      string            `json:"channel_id"`
	Usage          tokenusage.Actual `json:"usage"`
	AmountMicros   int64             `json:"amount_micros"`
	Currency       string            `json:"currency"`
	Billable       bool              `json:"billable"`
	BillableReason string            `json:"billable_reason,omitempty"`
}

// UsageRecord is the final customer usage record.
type UsageRecord struct {
	ID           string
	RequestID    string
	TenantID     string
	ProjectID    string
	APIKeyID     string
	Model        string
	ProviderType string
	ChannelID    string
	Usage        tokenusage.Actual
	Amount       money.Amount
	CreatedAt    time.Time
}

// LedgerEntry is an immutable balance movement.
type LedgerEntry struct {
	ID                 string
	RequestID          string
	SettlementKind     string
	TenantID           string
	ProjectID          string
	AccountID          string
	Currency           string
	AmountMicros       int64
	BalanceAfterMicros int64
	Reason             string
	CreatedAt          time.Time
}

// SettlementResult summarizes a completed settlement.
type SettlementResult struct {
	UsageRecordID string
	LedgerEntryID string
	AccountID     string
	Amount        money.Amount
	AlreadyDone   bool
}

// FailedSettlement stores replayable settlement inputs.
type FailedSettlement struct {
	ID          string
	RequestID   string
	TenantID    string
	ProjectID   string
	APIKeyID    string
	HoldID      string
	Payload     json.RawMessage
	Status      string
	RetryCount  int
	NextRetryAt time.Time
	LastError   string
	OwnerID     string
	ClaimedAt   time.Time
	HeartbeatAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ReconciliationIssue reports a balance/ledger mismatch.
type ReconciliationIssue struct {
	AccountID           string
	Currency            string
	AvailableMicros     int64
	HeldMicros          int64
	LedgerDebitsMicros  int64
	LedgerCreditsMicros int64
	Message             string
}
