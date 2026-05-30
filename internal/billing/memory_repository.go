package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/money"
)

// MemoryRepository is a deterministic test repository.
type MemoryRepository struct {
	mu            sync.Mutex
	accounts      map[string]BalanceAccount
	holds         map[string]BalanceHold
	holdByRequest map[string]string
	attempts      map[string]UsageAttempt
	records       map[string]UsageRecord
	ledger        map[string]LedgerEntry
	failed        map[string]FailedSettlement

	FailNextSettle bool
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		accounts:      make(map[string]BalanceAccount),
		holds:         make(map[string]BalanceHold),
		holdByRequest: make(map[string]string),
		attempts:      make(map[string]UsageAttempt),
		records:       make(map[string]UsageRecord),
		ledger:        make(map[string]LedgerEntry),
		failed:        make(map[string]FailedSettlement),
	}
}

func (r *MemoryRepository) EnsureBalanceAccount(_ context.Context, account BalanceAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if account.ID == "" {
		account.ID = newID("acct")
	}
	if account.OpeningMicros == 0 {
		account.OpeningMicros = account.AvailableMicros + account.HeldMicros
	}
	r.accounts[accountKey(account.TenantID, account.ProjectID, account.Currency)] = account
	return nil
}

func (r *MemoryRepository) CreateHold(_ context.Context, request HoldRequest) (*BalanceHold, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if holdID := r.holdByRequest[request.RequestID]; holdID != "" {
		hold := r.holds[holdID]
		return &hold, nil
	}
	key := accountKey(request.TenantID, request.ProjectID, request.Currency)
	account, ok := r.accounts[key]
	if !ok {
		return nil, apperr.InsufficientBalance("balance account is missing")
	}
	if account.AvailableMicros < request.AmountMicros {
		return nil, apperr.InsufficientBalance("insufficient balance")
	}
	account.AvailableMicros -= request.AmountMicros
	account.HeldMicros += request.AmountMicros
	r.accounts[key] = account
	hold := BalanceHold{
		ID:           newID("hold"),
		RequestID:    request.RequestID,
		TenantID:     request.TenantID,
		ProjectID:    request.ProjectID,
		APIKeyID:     request.APIKeyID,
		AccountID:    account.ID,
		Currency:     request.Currency,
		AmountMicros: request.AmountMicros,
		Status:       HoldStatusActive,
		ExpiresAt:    request.ExpiresAt,
	}
	r.holds[hold.ID] = hold
	r.holdByRequest[request.RequestID] = hold.ID
	return &hold, nil
}

func (r *MemoryRepository) GetHoldByRequestID(_ context.Context, requestID string) (*BalanceHold, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	holdID := r.holdByRequest[requestID]
	if holdID == "" {
		return nil, false, nil
	}
	hold := r.holds[holdID]
	return &hold, true, nil
}

func (r *MemoryRepository) ReleaseHold(_ context.Context, holdID string, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	hold := r.holds[holdID]
	if hold.ID == "" || hold.Status != HoldStatusActive {
		return nil
	}
	for key, account := range r.accounts {
		if account.ID != hold.AccountID {
			continue
		}
		account.AvailableMicros += hold.AmountMicros
		account.HeldMicros -= hold.AmountMicros
		r.accounts[key] = account
	}
	hold.Status = HoldStatusReleased
	hold.ReleaseReason = reason
	r.holds[holdID] = hold
	return nil
}

func (r *MemoryRepository) RecordUsageAttempt(_ context.Context, attempt UsageAttempt) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if attempt.ID == "" {
		attempt.ID = newID("uattempt")
	}
	r.attempts[fmt.Sprintf("%s:%d:%s", attempt.RequestID, attempt.AttemptIndex, attempt.ChannelID)] = attempt
	return nil
}

func (r *MemoryRepository) Settle(_ context.Context, plan SettlementPlan) (*SettlementResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.FailNextSettle {
		r.FailNextSettle = false
		return nil, fmt.Errorf("injected settlement failure")
	}
	if record, ok := r.records[plan.RequestID]; ok {
		entry := r.ledger[plan.RequestID]
		return &SettlementResult{
			UsageRecordID: record.ID,
			LedgerEntryID: entry.ID,
			AccountID:     entry.AccountID,
			Amount:        record.Amount,
			AlreadyDone:   true,
		}, nil
	}
	hold := r.holds[plan.HoldID]
	if hold.ID == "" {
		return nil, fmt.Errorf("hold not found")
	}
	if hold.Status != HoldStatusActive {
		return nil, fmt.Errorf("hold is %s", hold.Status)
	}
	var accountKeyValue string
	var account BalanceAccount
	for key, value := range r.accounts {
		if value.ID == hold.AccountID {
			accountKeyValue = key
			account = value
			break
		}
	}
	charge := plan.AmountMicros
	if !plan.Billable {
		charge = 0
	}
	if charge < 0 {
		charge = 0
	}
	fromHeld := minInt64(charge, hold.AmountMicros)
	refund := hold.AmountMicros - fromHeld
	extra := charge - fromHeld
	if account.AvailableMicros < extra {
		return nil, apperr.InsufficientBalance("insufficient balance for final settlement")
	}
	account.AvailableMicros = account.AvailableMicros - extra + refund
	account.HeldMicros -= hold.AmountMicros
	r.accounts[accountKeyValue] = account
	hold.Status = HoldStatusSettled
	r.holds[hold.ID] = hold
	record := UsageRecord{
		ID:           newID("usage"),
		RequestID:    plan.RequestID,
		TenantID:     plan.TenantID,
		ProjectID:    plan.ProjectID,
		APIKeyID:     plan.APIKeyID,
		Model:        plan.Model,
		ProviderType: plan.ProviderType,
		ChannelID:    plan.ChannelID,
		Usage:        plan.Usage,
		Amount:       money.New(plan.Currency, charge),
	}
	entry := LedgerEntry{
		ID:                 newID("ledger"),
		RequestID:          plan.RequestID,
		SettlementKind:     "usage_debit",
		TenantID:           plan.TenantID,
		ProjectID:          plan.ProjectID,
		AccountID:          account.ID,
		Currency:           plan.Currency,
		AmountMicros:       -charge,
		BalanceAfterMicros: account.AvailableMicros,
		Reason:             settlementReason(plan),
	}
	r.records[plan.RequestID] = record
	r.ledger[plan.RequestID] = entry
	return &SettlementResult{UsageRecordID: record.ID, LedgerEntryID: entry.ID, AccountID: account.ID, Amount: record.Amount}, nil
}

func (r *MemoryRepository) SaveFailedSettlement(_ context.Context, failed FailedSettlement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if failed.ID == "" {
		failed.ID = newID("failed")
	}
	if len(failed.Payload) == 0 {
		payload, _ := json.Marshal(failed)
		failed.Payload = payload
	}
	if failed.Status == "" {
		failed.Status = FailedSettlementPending
	}
	r.failed[failed.ID] = failed
	return nil
}

func (r *MemoryRepository) ListPendingFailedSettlements(_ context.Context, limit int) ([]FailedSettlement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []FailedSettlement
	now := time.Now().UTC()
	for _, failed := range r.failed {
		if len(out) >= limit {
			break
		}
		if (failed.Status == FailedSettlementPending || failed.Status == FailedSettlementFailed) && !failed.NextRetryAt.After(now) {
			out = append(out, failed)
		}
	}
	return out, nil
}

func (r *MemoryRepository) MarkFailedSettlementReplayed(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	failed := r.failed[id]
	failed.Status = FailedSettlementReplayed
	r.failed[id] = failed
	return nil
}

func (r *MemoryRepository) MarkFailedSettlementFailed(_ context.Context, id string, nextRetryAt time.Time, lastError string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	failed := r.failed[id]
	failed.Status = FailedSettlementFailed
	failed.RetryCount++
	failed.NextRetryAt = nextRetryAt
	failed.LastError = lastError
	r.failed[id] = failed
	return nil
}

func (r *MemoryRepository) Reconcile(_ context.Context) ([]ReconciliationIssue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var issues []ReconciliationIssue
	for _, account := range r.accounts {
		var debits, credits int64
		for _, entry := range r.ledger {
			if entry.AccountID != account.ID {
				continue
			}
			if entry.AmountMicros < 0 {
				debits += -entry.AmountMicros
			} else {
				credits += entry.AmountMicros
			}
		}
		expected := account.OpeningMicros + credits - debits
		actual := account.AvailableMicros + account.HeldMicros
		if expected != actual {
			issues = append(issues, ReconciliationIssue{
				AccountID:           account.ID,
				Currency:            account.Currency,
				AvailableMicros:     account.AvailableMicros,
				HeldMicros:          account.HeldMicros,
				LedgerDebitsMicros:  debits,
				LedgerCreditsMicros: credits,
				Message:             fmt.Sprintf("balance total %d does not match ledger expected total %d", actual, expected),
			})
		}
	}
	return issues, nil
}

func accountKey(tenantID, projectID, currency string) string {
	return tenantID + ":" + projectID + ":" + currency
}

var _ Repository = (*MemoryRepository)(nil)
