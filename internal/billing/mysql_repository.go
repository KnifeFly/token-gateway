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

// NewMySQLRepository returns a MySQL-backed billing repository.
func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

// EnsureBalanceAccount inserts the balance account if it does not already exist.
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

// CreateHold reserves balance in a transaction and returns an idempotent hold.
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

// GetHoldByRequestID finds the hold associated with requestID.
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

// ReleaseHold returns active held funds to available balance.
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

// ReleaseExpiredHolds returns expired active holds to available balance.
func (r *MySQLRepository) ReleaseExpiredHolds(ctx context.Context, now time.Time, limit int) (int, error) {
	if r == nil || r.db == nil {
		return 0, apperr.ConfigUnavailable("billing database is unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `
SELECT id, request_id, tenant_id, project_id, api_key_id, account_id, currency, amount_micros,
       status, COALESCE(release_reason, ''), expires_at, created_at, updated_at
FROM balance_holds
WHERE status = ? AND expires_at <= ?
  AND NOT EXISTS (
    SELECT 1
    FROM tasks
    WHERE tasks.balance_hold_id = balance_holds.id
      AND tasks.status IN ('queued', 'running')
  )
ORDER BY expires_at ASC, created_at ASC
LIMIT ? FOR UPDATE`, HoldStatusActive, now.UTC(), limit)
	if err != nil {
		return 0, err
	}
	var holds []BalanceHold
	for rows.Next() {
		var hold BalanceHold
		if err := rows.Scan(
			&hold.ID, &hold.RequestID, &hold.TenantID, &hold.ProjectID, &hold.APIKeyID,
			&hold.AccountID, &hold.Currency, &hold.AmountMicros, &hold.Status,
			&hold.ReleaseReason, &hold.ExpiresAt, &hold.CreatedAt, &hold.UpdatedAt,
		); err != nil {
			_ = rows.Close()
			return 0, err
		}
		holds = append(holds, hold)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, hold := range holds {
		if _, err := tx.ExecContext(ctx, `
UPDATE balance_accounts
SET available_micros = available_micros + ?,
    held_micros = GREATEST(held_micros - ?, 0),
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, hold.AmountMicros, hold.AmountMicros, hold.AccountID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE balance_holds
SET status = ?, release_reason = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND status = ?`, HoldStatusReleased, "expired hold reaper", hold.ID, HoldStatusActive); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(holds), nil
}

// RecordUsageAttempt upserts one provider attempt for the request.
func (r *MySQLRepository) RecordUsageAttempt(ctx context.Context, attempt UsageAttempt) error {
	if r == nil || r.db == nil {
		return nil
	}
	if attempt.ID == "" {
		attempt.ID = newID("uattempt")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO usage_attempts (
  id, request_id, attempt_index, task_id, tenant_id, project_id, api_key_id, channel_id, provider_type,
  model, upstream_model, status_code, error_code, success, estimated_input_tokens, estimated_output_tokens,
  actual_input_tokens, actual_output_tokens, retryable, retry_budget_consumed,
  retry_budget_remaining, fallback_from_channel_id, fallback_from_provider, circuit_state, final
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  task_id = VALUES(task_id),
  upstream_model = VALUES(upstream_model),
  status_code = VALUES(status_code),
  error_code = VALUES(error_code),
  success = VALUES(success),
  actual_input_tokens = VALUES(actual_input_tokens),
  actual_output_tokens = VALUES(actual_output_tokens),
  retryable = VALUES(retryable),
  retry_budget_consumed = VALUES(retry_budget_consumed),
  retry_budget_remaining = VALUES(retry_budget_remaining),
  fallback_from_channel_id = VALUES(fallback_from_channel_id),
  fallback_from_provider = VALUES(fallback_from_provider),
  circuit_state = VALUES(circuit_state),
  final = VALUES(final),
  updated_at = CURRENT_TIMESTAMP`,
		attempt.ID, attempt.RequestID, attempt.AttemptIndex, attempt.TaskID, attempt.TenantID, attempt.ProjectID,
		attempt.APIKeyID, attempt.ChannelID, attempt.ProviderType, attempt.Model, attempt.UpstreamModel, attempt.StatusCode,
		attempt.ErrorCode, attempt.Success, attempt.EstimatedInputTokens, attempt.EstimatedOutputTokens,
		attempt.ActualInputTokens, attempt.ActualOutputTokens, attempt.Retryable,
		attempt.RetryBudgetConsumed, attempt.RetryBudgetRemaining, attempt.FallbackFromChannelID,
		attempt.FallbackFromProvider, attempt.CircuitState, attempt.Final,
	)
	return err
}

// Settle applies a settlement plan with ledger and usage writes in one transaction.
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
	charge := settlementCharge(plan)
	if plan.HoldID == "" {
		result, err := settleWithoutHold(ctx, tx, plan, charge)
		if err != nil {
			return nil, err
		}
		return result, tx.Commit()
	}

	// Step 1: lock the hold and balance row to keep settlement idempotent.
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

	// Step 2: calculate held-fund consumption, refund, and extra debit.
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

	// Step 3: persist usage, ledger, account, and hold state atomically.
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

func settleWithoutHold(ctx context.Context, tx *sql.Tx, plan SettlementPlan, charge int64) (*SettlementResult, error) {
	if charge > 0 {
		return nil, fmt.Errorf("billable settlement requires a balance hold")
	}
	account, err := selectBalanceAccountForUpdate(ctx, tx, plan.TenantID, plan.ProjectID, plan.Currency)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO balance_accounts (
  id, tenant_id, project_id, currency, opening_micros, available_micros, held_micros
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP`,
			newID("acct"), plan.TenantID, plan.ProjectID, plan.Currency, int64(0), int64(0), int64(0),
		); err != nil {
			return nil, err
		}
		account, err = selectBalanceAccountForUpdate(ctx, tx, plan.TenantID, plan.ProjectID, plan.Currency)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
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
		plan.Currency, -charge, account.AvailableMicros, settlementReason(plan),
	); err != nil {
		return nil, err
	}
	return &SettlementResult{
		UsageRecordID: usageRecordID,
		LedgerEntryID: ledgerEntryID,
		AccountID:     account.ID,
		Amount:        money.New(plan.Currency, charge),
	}, nil
}

// SaveFailedSettlement stores a failed settlement for repair replay.
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

// ClaimPendingFailedSettlements assigns due failed settlements to one worker owner.
func (r *MySQLRepository) ClaimPendingFailedSettlements(ctx context.Context, ownerID string, claimTimeout time.Duration, limit int) ([]FailedSettlement, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if ownerID == "" {
		ownerID = newID("repair_owner")
	}
	if claimTimeout <= 0 {
		claimTimeout = 5 * time.Minute
	}
	if limit <= 0 {
		limit = 100
	}
	now := time.Now().UTC()
	claimExpiredBefore := now.Add(-claimTimeout)
	if _, err := r.db.ExecContext(ctx, `
UPDATE failed_settlements
SET status = ?, owner_id = ?, claimed_at = ?, heartbeat_at = ?, updated_at = CURRENT_TIMESTAMP
WHERE ((status IN (?, ?) AND next_retry_at <= ?)
   OR (status = ? AND heartbeat_at <= ?))
ORDER BY next_retry_at ASC, created_at ASC
LIMIT ?`,
		FailedSettlementProcessing, ownerID, now, now,
		FailedSettlementPending, FailedSettlementFailed, now,
		FailedSettlementProcessing, claimExpiredBefore, limit); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, request_id, tenant_id, project_id, api_key_id, hold_id, payload_json, status,
       retry_count, next_retry_at, COALESCE(last_error, ''), COALESCE(owner_id, ''),
       claimed_at, heartbeat_at, created_at, updated_at
FROM failed_settlements
WHERE status = ? AND owner_id = ?
ORDER BY next_retry_at ASC, created_at ASC
LIMIT ?`, FailedSettlementProcessing, ownerID, limit)
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
			&failed.LastError, &failed.OwnerID, &failed.ClaimedAt, &failed.HeartbeatAt,
			&failed.CreatedAt, &failed.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, failed)
	}
	return out, rows.Err()
}

// MarkFailedSettlementReplayed marks a failed settlement as successfully repaired.
func (r *MySQLRepository) MarkFailedSettlementReplayed(ctx context.Context, id string, ownerID string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE failed_settlements
SET status = ?, owner_id = '', updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND (owner_id = ? OR ? = '')`, FailedSettlementReplayed, id, ownerID, ownerID)
	return err
}

// MarkFailedSettlementFailed records a failed replay attempt and retry schedule.
func (r *MySQLRepository) MarkFailedSettlementFailed(ctx context.Context, id string, ownerID string, nextRetryAt time.Time, lastError string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE failed_settlements
SET status = ?, owner_id = '', retry_count = retry_count + 1, next_retry_at = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND (owner_id = ? OR ? = '')`, FailedSettlementFailed, nextRetryAt, lastError, id, ownerID, ownerID)
	return err
}

// Reconcile reports balance accounts whose totals diverge from ledger movement.
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
