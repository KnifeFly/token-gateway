package task

import (
	"context"
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

// Settlement settles completed async tasks.
type Settlement interface {
	Settle(ctx context.Context, task Task, usage tokenusage.Actual) error
	RecordFailed(ctx context.Context, task Task, usage tokenusage.Actual, cause error) error
}

// NoopSettlement skips task settlement when billing is disabled.
type NoopSettlement struct{}

// Settle implements Settlement.
func (NoopSettlement) Settle(context.Context, Task, tokenusage.Actual) error {
	return nil
}

// RecordFailed implements Settlement.
func (NoopSettlement) RecordFailed(_ context.Context, _ Task, _ tokenusage.Actual, cause error) error {
	return cause
}

// BillingSettlement settles task usage through the billing repository.
type BillingSettlement struct {
	repo  billing.Repository
	price pricing.TokenPrice
}

// NewBillingSettlement returns a billing-backed task settlement service.
func NewBillingSettlement(repo billing.Repository, price pricing.TokenPrice) *BillingSettlement {
	return &BillingSettlement{repo: repo, price: price}
}

// Settle debits the task hold once and writes usage/ledger records.
func (s *BillingSettlement) Settle(ctx context.Context, task Task, usage tokenusage.Actual) error {
	if s == nil || s.repo == nil || task.BalanceHoldID == "" {
		return nil
	}
	_, err := s.repo.Settle(ctx, s.plan(task, usage))
	return err
}

// RecordFailed writes repairable failed settlement state.
func (s *BillingSettlement) RecordFailed(ctx context.Context, task Task, usage tokenusage.Actual, cause error) error {
	if s == nil || s.repo == nil || task.BalanceHoldID == "" {
		return cause
	}
	plan := s.plan(task, usage)
	payload, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	return s.repo.SaveFailedSettlement(ctx, billing.FailedSettlement{
		RequestID:   task.RequestID,
		TenantID:    task.TenantID,
		ProjectID:   task.ProjectID,
		APIKeyID:    task.APIKeyID,
		HoldID:      task.BalanceHoldID,
		Payload:     payload,
		Status:      billing.FailedSettlementPending,
		NextRetryAt: time.Now().UTC(),
		LastError:   cause.Error(),
	})
}

func (s *BillingSettlement) plan(task Task, usage tokenusage.Actual) billing.SettlementPlan {
	amount := s.price.QuoteActual(usage)
	return billing.SettlementPlan{
		RequestID:    task.RequestID,
		TenantID:     task.TenantID,
		ProjectID:    task.ProjectID,
		APIKeyID:     task.APIKeyID,
		HoldID:       task.BalanceHoldID,
		Model:        task.Model,
		ProviderType: task.ProviderType,
		ChannelID:    task.ChannelID,
		Usage:        usage,
		AmountMicros: amount.Micros,
		Currency:     amount.Currency,
		Billable:     true,
	}
}
