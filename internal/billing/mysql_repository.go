package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/money"
)

// MySQLRepository persists billing state in MySQL.
type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) EnsureBalanceAccount(ctx context.Context, account BalanceAccount) error {
	if r == nil || r.db == nil {
		return apperr.ConfigUnavailable("billing database is unavailable")
	}
	if account.ID == "" {
		account.ID = newID("acct")
	}
	if account.OpeningMicros == 0 {
		account.OpeningMicros = account.AvailableMicros + account.HeldMicros
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO balance_accounts (
  id, tenant_id, project_id, currency, opening_micros, available_micros, held_micros
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP`,
		account.ID, account.TenantID, account.ProjectID, account.Currency,
		account.OpeningMicros, account.AvailableMicros, account.HeldMicros,
	)
	return err
}

func (r *MySQLRepository) CreateHold(ctx context.Context, request HoldRequest) (*BalanceHold, error) {
	if r == nil || r.db == nil {
		return nil, apperr.ConfigUnavailable("billing database is unavailable")
	}
	if hold, ok, err := r.GetHoldByRequestID(ctx, request.RequestID); err != nil {
		return nil, err
	} else if ok {
		return hold, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	account, err := selectBalanceAccountForUpdate(ctx, tx, request.TenantID, request.ProjectID, request.Currency)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.InsufficientBalance("balance account is missing")
	}
	if err != nil {
		return nil, err
	}
	if account.AvailableMicros < request.AmountMicros {
		return nil, apperr.InsufficientBalance("insufficient balance")
	}
	hold := &BalanceHold{
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
	if _, err := tx.ExecContext(ctx, `
UPDATE balance_accounts
SET available_micros = available_micros - ?, held_micros = held_micros + ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, request.AmountMicros, request.AmountMicros, account.ID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO balance_holds (
  id, request_id, tenant_id, project_id, api_key_id, account_id, currency, amount_micros, status, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		hold.ID, hold.RequestID, hold.TenantID, hold.ProjectID, hold.APIKeyID, hold.AccountID,
		hold.Currency, hold.AmountMicros, hold.Status, hold.ExpiresAt,
	); err != nil {
		if existing, ok, getErr := r.GetHoldByRequestID(ctx, request.RequestID); getErr == nil && ok {
			return existing, nil
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return hold, nil
}

func (r *MySQLRepository) GetHoldByRequestID(ctx context.Context, requestID string) (*BalanceHold, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, apperr.ConfigUnavailable("billing database is unavailable")
	}
	hold, err := scanHold(r.db.QueryRowContext(ctx, `
SELECT id, request_id, tenant_id, project_id, api_key_id, account_id, currency, amount_micros,
       status, COALESCE(release_reason, ''), expires_at, created_at, updated_at
FROM balance_holds WHERE request_id = ?`, requestID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return hold, true, nil
}

func (r *MySQLRepository) ReleaseHold(ctx context.Context, holdID string, reason string) error {
	if holdID == "" || r == nil || r.db == nil {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	hold, err := selectHoldForUpdate(ctx, tx, holdID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if hold.Status != HoldStatusActive {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE balance_accounts
SET available_micros = available_micros + ?, held_micros = held_micros - ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, hold.AmountMicros, hold.AmountMicros, hold.AccountID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE balance_holds
SET status = ?, release_reason = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, HoldStatusReleased, reason, holdID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *MySQLRepository) RecordUsageAttempt(ctx context.Context, attempt UsageAttempt) error {
	if r == nil || r.db == nil {
		return nil
	}
	if attempt.ID == "" {
		attempt.ID = newID("uattempt")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO usage_attempts (
  id, request_id, attempt_index, tenant_id, project_id, api_key_id, channel_id, provider_type,
  model, status_code, error_code, success, estimated_input_tokens, estimated_output_tokens,
  actual_input_tokens, actual_output_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP`,
		attempt.ID, attempt.RequestID, attempt.AttemptIndex, attempt.TenantID, attempt.ProjectID,
		attempt.APIKeyID, attempt.ChannelID, attempt.ProviderType, attempt.Model, attempt.StatusCode,
		attempt.ErrorCode, attempt.Success, attempt.EstimatedInputTokens, attempt.EstimatedOutputTokens,
		attempt.ActualInputTokens, attempt.ActualOutputTokens,
	)
	return err
}

func (r *MySQLRepository) Settle(ctx context.Context, plan SettlementPlan) (*SettlementResult, error) {
	if r == nil || r.db == nil {
		return nil, apperr.ConfigUnavailable("billing database is unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if result, ok, err := existingSettlement(ctx, tx, plan.RequestID); err != nil {
		return nil, err
	} else if ok {
		result.AlreadyDone = true
		return result, tx.Commit()
	}
	hold, err := selectHoldForUpdate(ctx, tx, plan.HoldID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("balance hold %q not found", plan.HoldID)
	}
	if err != nil {
		return nil, err
	}
	if hold.Status == HoldStatusSettled {
		return &SettlementResult{AlreadyDone: true}, tx.Commit()
	}
	if hold.Status != HoldStatusActive {
		return nil, fmt.Errorf("balance hold %q is %s", hold.ID, hold.Status)
	}
	account, err := selectBalanceAccountByIDForUpdate(ctx, tx, hold.AccountID)
	if err != nil {
		return nil, err
	}
	charge := plan.AmountMicros
	if !plan.Billable {
		charge = 0
	}
	if charge < 0 {
		charge = 0
	}
	release := hold.AmountMicros
	fromHeld := minInt64(charge, release)
	refund := release - fromHeld
	extra := charge - fromHeld
	if account.AvailableMicros < extra {
		return nil, apperr.InsufficientBalance("insufficient balance for final settlement")
	}
	newAvailable := account.AvailableMicros - extra + refund
	newHeld := account.HeldMicros - release
	if newHeld < 0 {
		newHeld = 0
	}
	usageRecordID := newID("usage")
	ledgerEntryID := newID("ledger")
	if _, err := tx.ExecContext(ctx, `
INSERT INTO usage_records (
  id, request_id, tenant_id, project_id, api_key_id, model, provider_type, channel_id,
  input_tokens, output_tokens, total_tokens, amount_micros, currency
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		usageRecordID, plan.RequestID, plan.TenantID, plan.ProjectID, plan.APIKeyID, plan.Model,
		plan.ProviderType, plan.ChannelID, plan.Usage.InputTokens, plan.Usage.OutputTokens,
		plan.Usage.TotalTokens, charge, plan.Currency,
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_entries (
  id, request_id, settlement_kind, tenant_id, project_id, account_id, currency,
  amount_micros, balance_after_micros, reason
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ledgerEntryID, plan.RequestID, "usage_debit", plan.TenantID, plan.ProjectID, account.ID,
		plan.Currency, -charge, newAvailable, settlementReason(plan),
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE balance_accounts
SET available_micros = ?, held_micros = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, newAvailable, newHeld, account.ID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE balance_holds
SET status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, HoldStatusSettled, hold.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &SettlementResult{
		UsageRecordID: usageRecordID,
		LedgerEntryID: ledgerEntryID,
		AccountID:     account.ID,
		Amount:        money.New(plan.Currency, charge),
	}, nil
}

func (r *MySQLRepository) SaveFailedSettlement(ctx context.Context, failed FailedSettlement) error {
	if r == nil || r.db == nil {
		return apperr.ConfigUnavailable("billing database is unavailable")
	}
	if failed.ID == "" {
		failed.ID = newID("failed")
	}
	if failed.Status == "" {
		failed.Status = FailedSettlementPending
	}
	if failed.NextRetryAt.IsZero() {
		failed.NextRetryAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO failed_settlements (
  id, request_id, tenant_id, project_id, api_key_id, hold_id, payload_json, status, retry_count, next_retry_at, last_error
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  payload_json = VALUES(payload_json),
  status = VALUES(status),
  retry_count = retry_count + 1,
  next_retry_at = VALUES(next_retry_at),
  last_error = VALUES(last_error),
  updated_at = CURRENT_TIMESTAMP`,
		failed.ID, failed.RequestID, failed.TenantID, failed.ProjectID, failed.APIKeyID, failed.HoldID,
		[]byte(failed.Payload), failed.Status, failed.RetryCount, failed.NextRetryAt, failed.LastError,
	)
	return err
}

func (r *MySQLRepository) ListPendingFailedSettlements(ctx context.Context, limit int) ([]FailedSettlement, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, request_id, tenant_id, project_id, api_key_id, hold_id, payload_json, status,
       retry_count, next_retry_at, COALESCE(last_error, ''), created_at, updated_at
FROM failed_settlements
WHERE status IN (?, ?) AND next_retry_at <= CURRENT_TIMESTAMP
ORDER BY next_retry_at ASC, created_at ASC
LIMIT ?`, FailedSettlementPending, FailedSettlementFailed, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FailedSettlement
	for rows.Next() {
		var failed FailedSettlement
		if err := rows.Scan(
			&failed.ID, &failed.RequestID, &failed.TenantID, &failed.ProjectID, &failed.APIKeyID,
			&failed.HoldID, &failed.Payload, &failed.Status, &failed.RetryCount, &failed.NextRetryAt,
			&failed.LastError, &failed.CreatedAt, &failed.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, failed)
	}
	return out, rows.Err()
}

func (r *MySQLRepository) MarkFailedSettlementReplayed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE failed_settlements
SET status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, FailedSettlementReplayed, id)
	return err
}

func (r *MySQLRepository) MarkFailedSettlementFailed(ctx context.Context, id string, nextRetryAt time.Time, lastError string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE failed_settlements
SET status = ?, retry_count = retry_count + 1, next_retry_at = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, FailedSettlementFailed, nextRetryAt, lastError, id)
	return err
}

func (r *MySQLRepository) Reconcile(ctx context.Context) ([]ReconciliationIssue, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT a.id, a.currency, a.opening_micros, a.available_micros, a.held_micros,
       COALESCE(SUM(CASE WHEN l.amount_micros < 0 THEN -l.amount_micros ELSE 0 END), 0) AS debits,
       COALESCE(SUM(CASE WHEN l.amount_micros > 0 THEN l.amount_micros ELSE 0 END), 0) AS credits
FROM balance_accounts a
LEFT JOIN ledger_entries l ON l.account_id = a.id
GROUP BY a.id, a.currency, a.opening_micros, a.available_micros, a.held_micros`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var issues []ReconciliationIssue
	for rows.Next() {
		var accountID, currency string
		var opening, available, held, debits, credits int64
		if err := rows.Scan(&accountID, &currency, &opening, &available, &held, &debits, &credits); err != nil {
			return nil, err
		}
		expected := opening + credits - debits
		actual := available + held
		if expected != actual || available < 0 || held < 0 {
			issues = append(issues, ReconciliationIssue{
				AccountID:           accountID,
				Currency:            currency,
				AvailableMicros:     available,
				HeldMicros:          held,
				LedgerDebitsMicros:  debits,
				LedgerCreditsMicros: credits,
				Message:             fmt.Sprintf("balance total %d does not match ledger expected total %d", actual, expected),
			})
		}
	}
	return issues, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHold(row rowScanner) (*BalanceHold, error) {
	var hold BalanceHold
	err := row.Scan(
		&hold.ID, &hold.RequestID, &hold.TenantID, &hold.ProjectID, &hold.APIKeyID,
		&hold.AccountID, &hold.Currency, &hold.AmountMicros, &hold.Status,
		&hold.ReleaseReason, &hold.ExpiresAt, &hold.CreatedAt, &hold.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &hold, nil
}

func selectHoldForUpdate(ctx context.Context, tx *sql.Tx, holdID string) (*BalanceHold, error) {
	return scanHold(tx.QueryRowContext(ctx, `
SELECT id, request_id, tenant_id, project_id, api_key_id, account_id, currency, amount_micros,
       status, COALESCE(release_reason, ''), expires_at, created_at, updated_at
FROM balance_holds WHERE id = ? FOR UPDATE`, holdID))
}

func selectBalanceAccountForUpdate(ctx context.Context, tx *sql.Tx, tenantID, projectID, currency string) (*BalanceAccount, error) {
	account := &BalanceAccount{}
	err := tx.QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, currency, opening_micros, available_micros, held_micros, created_at, updated_at
FROM balance_accounts
WHERE tenant_id = ? AND project_id = ? AND currency = ?
FOR UPDATE`, tenantID, projectID, currency).Scan(
		&account.ID, &account.TenantID, &account.ProjectID, &account.Currency, &account.OpeningMicros,
		&account.AvailableMicros, &account.HeldMicros, &account.CreatedAt, &account.UpdatedAt,
	)
	return account, err
}

func selectBalanceAccountByIDForUpdate(ctx context.Context, tx *sql.Tx, accountID string) (*BalanceAccount, error) {
	account := &BalanceAccount{}
	err := tx.QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, currency, opening_micros, available_micros, held_micros, created_at, updated_at
FROM balance_accounts
WHERE id = ?
FOR UPDATE`, accountID).Scan(
		&account.ID, &account.TenantID, &account.ProjectID, &account.Currency, &account.OpeningMicros,
		&account.AvailableMicros, &account.HeldMicros, &account.CreatedAt, &account.UpdatedAt,
	)
	return account, err
}

func existingSettlement(ctx context.Context, tx *sql.Tx, requestID string) (*SettlementResult, bool, error) {
	var usageID string
	err := tx.QueryRowContext(ctx, "SELECT id FROM usage_records WHERE request_id = ?", requestID).Scan(&usageID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	result := &SettlementResult{UsageRecordID: usageID, AlreadyDone: true}
	_ = tx.QueryRowContext(ctx, `
SELECT id, account_id, currency, amount_micros
FROM ledger_entries WHERE request_id = ? AND settlement_kind = ?`,
		requestID, "usage_debit").Scan(&result.LedgerEntryID, &result.AccountID, &result.Amount.Currency, &result.Amount.Micros)
	if result.Amount.Micros < 0 {
		result.Amount.Micros = -result.Amount.Micros
	}
	return result, true, nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

var _ Repository = (*MySQLRepository)(nil)
