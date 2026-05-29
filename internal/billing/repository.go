package billing

import (
	"context"
	"errors"
	"time"
)

var ErrAlreadySettled = errors.New("request already settled")

// Repository persists billing state.
type Repository interface {
	EnsureBalanceAccount(ctx context.Context, account BalanceAccount) error
	CreateHold(ctx context.Context, request HoldRequest) (*BalanceHold, error)
	GetHoldByRequestID(ctx context.Context, requestID string) (*BalanceHold, bool, error)
	ReleaseHold(ctx context.Context, holdID string, reason string) error
	RecordUsageAttempt(ctx context.Context, attempt UsageAttempt) error
	Settle(ctx context.Context, plan SettlementPlan) (*SettlementResult, error)
	SaveFailedSettlement(ctx context.Context, failed FailedSettlement) error
	ListPendingFailedSettlements(ctx context.Context, limit int) ([]FailedSettlement, error)
	MarkFailedSettlementReplayed(ctx context.Context, id string) error
	MarkFailedSettlementFailed(ctx context.Context, id string, nextRetryAt time.Time, lastError string) error
	Reconcile(ctx context.Context) ([]ReconciliationIssue, error)
}
