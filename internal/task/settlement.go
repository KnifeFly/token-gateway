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

// SettleTerminalTask settles or records repair state for a terminal async task.
func SettleTerminalTask(ctx context.Context, settlement Settlement, task Task, usage tokenusage.Actual) error {
	if !IsTerminal(task.Status) {
		return nil
	}
	if settlement == nil {
		settlement = NoopSettlement{}
	}
	if err := settlement.Settle(ctx, task, usage); err != nil {
		if recordErr := settlement.RecordFailed(ctx, task, usage, err); recordErr != nil {
			return recordErr
		}
	}
	return nil
}

// BillingSettlement settles task usage through the billing repository.
type BillingSettlement struct {
	repo   billing.Repository
	price  pricing.TokenPrice
	policy billing.BillabilityPolicy
}

// NewBillingSettlement returns a billing-backed task settlement service.
func NewBillingSettlement(repo billing.Repository, price pricing.TokenPrice) *BillingSettlement {
	return &BillingSettlement{repo: repo, price: price, policy: billing.NewBillabilityPolicy()}
}

// Settle debits the task hold once and writes usage/ledger records.
func (s *BillingSettlement) Settle(ctx context.Context, task Task, usage tokenusage.Actual) error {
	if s == nil || s.repo == nil {
		return nil
	}
	_, err := s.repo.Settle(ctx, s.plan(task, usage))
	return err
}

// RecordFailed writes repairable failed settlement state.
func (s *BillingSettlement) RecordFailed(ctx context.Context, task Task, usage tokenusage.Actual, cause error) error {
	if s == nil || s.repo == nil {
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
	decision := s.policy.Decide(billing.BillabilityContext{
		Operation:       billing.BillabilityOperationTask,
		Usage:           usage,
		TaskResultBytes: int64(len(task.Result)),
		TaskStatus:      string(task.Status),
		ProviderError:   task.ErrorCode,
	})
	book := s.price.PriceBook(pricing.CategoryChat)
	if task.PriceSnapshot.Source == "runtime_price_rule" || task.PriceSnapshot.Source == "gateway_default_price" {
		book = task.PriceSnapshot.PriceBook(s.price)
	}
	amount := book.QuoteActual(usage)
	if amount.Currency == "" && task.PriceSnapshot.Currency != "" {
		amount.Currency = task.PriceSnapshot.Currency
	}
	if !decision.Billable {
		amount.Micros = 0
	}
	return billing.SettlementPlan{
		RequestID:      task.RequestID,
		TenantID:       task.TenantID,
		ProjectID:      task.ProjectID,
		APIKeyID:       task.APIKeyID,
		HoldID:         task.BalanceHoldID,
		Model:          task.Model,
		ProviderType:   task.ProviderType,
		ChannelID:      task.ChannelID,
		Usage:          usage,
		AmountMicros:   amount.Micros,
		Currency:       amount.Currency,
		Billable:       decision.Billable,
		BillableReason: decision.Reason,
	}
}
