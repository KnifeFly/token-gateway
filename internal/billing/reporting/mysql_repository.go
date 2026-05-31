package reporting

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// MySQLRepository reads commercial reports from the operational MySQL schema.
type MySQLRepository struct {
	db *sql.DB
}

// NewMySQLRepository returns a MySQL commercial reporting repository.
func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

// TenantUsageReport reads balance, usage, and ledger rows from MySQL.
func (r *MySQLRepository) TenantUsageReport(ctx context.Context, filter TenantUsageFilter) (*TenantUsageReport, error) {
	if r == nil || r.db == nil {
		return nil, apperr.ConfigUnavailable("reporting database is unavailable")
	}
	report := &TenantUsageReport{GeneratedAt: time.Now().UTC(), Filter: filter}
	balances, err := r.listBalances(ctx, filter.TenantID, filter.ProjectID, filter.Currency)
	if err != nil {
		return nil, err
	}
	usage, err := r.listUsageSummaries(ctx, filter)
	if err != nil {
		return nil, err
	}
	ledger, err := r.listLedger(ctx, filter)
	if err != nil {
		return nil, err
	}
	report.Balances = balances
	report.Usage = usage
	report.Ledger = ledger
	for _, row := range usage {
		addTotals(&report.Totals, row)
	}
	return report, nil
}

// UpsertProviderCostProfile creates or updates provider cost assumptions.
func (r *MySQLRepository) UpsertProviderCostProfile(ctx context.Context, profile ProviderCostProfile) (*ProviderCostProfile, error) {
	if r == nil || r.db == nil {
		return nil, apperr.ConfigUnavailable("reporting database is unavailable")
	}
	if profile.ID == "" {
		profile.ID = newID("cost")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO provider_cost_profiles (
  id, provider_type, channel_id, public_model, currency, input_micros_per_token,
  output_micros_per_token, fixed_micros_per_request, effective_from, enabled
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  input_micros_per_token = VALUES(input_micros_per_token),
  output_micros_per_token = VALUES(output_micros_per_token),
  fixed_micros_per_request = VALUES(fixed_micros_per_request),
  effective_from = VALUES(effective_from),
  enabled = VALUES(enabled),
  updated_at = CURRENT_TIMESTAMP`,
		profile.ID, profile.ProviderType, profile.ChannelID, profile.PublicModel, profile.Currency,
		profile.InputMicrosPerToken, profile.OutputMicrosPerToken, profile.FixedMicrosPerRequest,
		profile.EffectiveFrom, profile.Enabled,
	)
	if err != nil {
		return nil, err
	}
	return r.getProviderCostProfile(ctx, profile.ProviderType, profile.ChannelID, profile.PublicModel, profile.Currency)
}

// ProviderProfitReport aggregates revenue, provider cost, and profit by channel.
func (r *MySQLRepository) ProviderProfitReport(ctx context.Context, filter ProviderProfitFilter) (*ProviderProfitReport, error) {
	if r == nil || r.db == nil {
		return nil, apperr.ConfigUnavailable("reporting database is unavailable")
	}
	profiles, err := r.providerCostProfiles(ctx)
	if err != nil {
		return nil, err
	}
	query := `
SELECT provider_type, channel_id, model, currency, COUNT(*),
       COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
       COALESCE(SUM(total_tokens), 0), COALESCE(SUM(amount_micros), 0)
FROM usage_records`
	where, args := reportWhere(filter.TenantID, filter.ProjectID, "", filter.From, filter.To)
	query += where + ` GROUP BY provider_type, channel_id, model, currency ORDER BY provider_type, channel_id, model`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	report := &ProviderProfitReport{GeneratedAt: time.Now().UTC(), Filter: filter}
	for rows.Next() {
		var row ProviderProfitRow
		if err := rows.Scan(
			&row.ProviderType, &row.ChannelID, &row.Model, &row.Currency, &row.Requests,
			&row.InputTokens, &row.OutputTokens, &row.TotalTokens, &row.RevenueMicros,
		); err != nil {
			return nil, err
		}
		profile, ok := profiles[costProfileKey(row.ProviderType, row.ChannelID, row.Model, row.Currency)]
		if !ok || !profile.Enabled {
			row.CostProfileMissing = true
		} else {
			row.ProviderCostMicros = row.InputTokens*profile.InputMicrosPerToken + row.OutputTokens*profile.OutputMicrosPerToken + row.Requests*profile.FixedMicrosPerRequest
		}
		row.ProfitMicros = row.RevenueMicros - row.ProviderCostMicros
		report.Rows = append(report.Rows, row)
		report.Totals.Requests += row.Requests
		report.Totals.InputTokens += row.InputTokens
		report.Totals.OutputTokens += row.OutputTokens
		report.Totals.TotalTokens += row.TotalTokens
		report.Totals.RevenueMicros += row.RevenueMicros
		report.Totals.ProviderCostMicros += row.ProviderCostMicros
		report.Totals.ProfitMicros += row.ProfitMicros
		report.Totals.Currency = row.Currency
	}
	return report, rows.Err()
}

// ReconciliationReport returns balance mismatches and failed settlement state.
func (r *MySQLRepository) ReconciliationReport(ctx context.Context) (*ReconciliationReport, error) {
	if r == nil || r.db == nil {
		return nil, apperr.ConfigUnavailable("reporting database is unavailable")
	}
	issues, err := billing.NewMySQLRepository(r.db).Reconcile(ctx)
	if err != nil {
		return nil, err
	}
	failed, err := r.listFailedSettlements(ctx)
	if err != nil {
		return nil, err
	}
	attempts, err := r.listUsageAttempts(ctx)
	if err != nil {
		return nil, err
	}
	return &ReconciliationReport{
		GeneratedAt:       time.Now().UTC(),
		Issues:            issues,
		FailedSettlements: failed,
		UsageAttempts:     attempts,
		BudgetSemantics:   admissionBudgetSemantics(),
	}, nil
}

// CreateManualAdjustment writes an idempotent operator balance adjustment.
func (r *MySQLRepository) CreateManualAdjustment(ctx context.Context, request ManualAdjustmentRequest) (*ManualAdjustment, error) {
	if r == nil || r.db == nil {
		return nil, apperr.ConfigUnavailable("reporting database is unavailable")
	}
	if existing, ok, err := r.getManualAdjustmentByKey(ctx, request.IdempotencyKey); err != nil {
		return nil, err
	} else if ok {
		return existing, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Step 1: lock or create the target balance account.
	account, err := selectBalanceForAdjustment(ctx, tx, request.TenantID, request.ProjectID, request.Currency)
	if errors.Is(err, sql.ErrNoRows) {
		if request.AmountMicros < 0 {
			return nil, apperr.InsufficientBalance("balance account is missing")
		}
		account = BalanceSummary{
			AccountID: newID("acct"),
			TenantID:  request.TenantID,
			ProjectID: request.ProjectID,
			Currency:  request.Currency,
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO balance_accounts (id, tenant_id, project_id, currency, opening_micros, available_micros, held_micros)
VALUES (?, ?, ?, ?, 0, 0, 0)`,
			account.AccountID, account.TenantID, account.ProjectID, account.Currency,
		); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	nextAvailable := account.AvailableMicros + request.AmountMicros
	if nextAvailable < 0 {
		return nil, apperr.InsufficientBalance("manual adjustment would make balance negative")
	}
	adjustmentID := newID("adj")
	ledgerID := newID("ledger")

	// Step 2: update balance and write the matching immutable ledger row.
	if _, err := tx.ExecContext(ctx, `
UPDATE balance_accounts
SET available_micros = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, nextAvailable, account.AccountID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_entries (
  id, request_id, settlement_kind, tenant_id, project_id, account_id, currency,
  amount_micros, balance_after_micros, reason
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ledgerID, adjustmentID, "manual_adjustment", request.TenantID, request.ProjectID,
		account.AccountID, request.Currency, request.AmountMicros, nextAvailable, request.Reason,
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO manual_adjustments (
  id, idempotency_key, tenant_id, project_id, account_id, ledger_entry_id,
  currency, amount_micros, reason, operator_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		adjustmentID, request.IdempotencyKey, request.TenantID, request.ProjectID, account.AccountID,
		ledgerID, request.Currency, request.AmountMicros, request.Reason, request.OperatorID,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	adjustment, ok, err := r.getManualAdjustmentByKey(ctx, request.IdempotencyKey)
	if err != nil || !ok {
		return nil, err
	}
	return adjustment, nil
}

// AgentMetadataReport aggregates task metadata for workflow and scene reports.
func (r *MySQLRepository) AgentMetadataReport(ctx context.Context, filter AgentMetadataFilter) (*AgentMetadataReport, error) {
	if r == nil || r.db == nil {
		return nil, apperr.ConfigUnavailable("reporting database is unavailable")
	}
	query := `
SELECT
  COALESCE(JSON_UNQUOTE(JSON_EXTRACT(metadata_json, '$.workflow')), '') AS workflow,
  COALESCE(JSON_UNQUOTE(JSON_EXTRACT(metadata_json, '$.scene')), '') AS scene,
  COALESCE(JSON_UNQUOTE(JSON_EXTRACT(metadata_json, '$.shot')), '') AS shot,
  kind, media_type, model, COUNT(*),
  COALESCE(SUM(CASE WHEN status = 'succeeded' THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(u.amount_micros), 0),
  COALESCE(MAX(u.currency), '')
FROM tasks t
LEFT JOIN usage_records u ON u.request_id = t.request_id`
	where, args := taskReportWhere(filter.TenantID, filter.ProjectID, filter.From, filter.To)
	query += where + `
GROUP BY workflow, scene, shot, kind, media_type, model
ORDER BY workflow, scene, shot, kind, media_type, model`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	report := &AgentMetadataReport{GeneratedAt: time.Now().UTC(), Filter: filter}
	for rows.Next() {
		var row AgentMetadataRow
		if err := rows.Scan(
			&row.Workflow, &row.Scene, &row.Shot, &row.Kind, &row.MediaType, &row.Model,
			&row.Tasks, &row.Succeeded, &row.Failed, &row.AmountMicros, &row.Currency,
		); err != nil {
			return nil, err
		}
		report.Rows = append(report.Rows, row)
	}
	return report, rows.Err()
}

func (r *MySQLRepository) listBalances(ctx context.Context, tenantID, projectID, currency string) ([]BalanceSummary, error) {
	query := `
SELECT id, tenant_id, project_id, currency, opening_micros, available_micros, held_micros, updated_at
FROM balance_accounts`
	where, args := scopeWhere(tenantID, projectID, currency)
	query += where + ` ORDER BY tenant_id, project_id, currency`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BalanceSummary
	for rows.Next() {
		var balance BalanceSummary
		if err := rows.Scan(
			&balance.AccountID, &balance.TenantID, &balance.ProjectID, &balance.Currency,
			&balance.OpeningMicros, &balance.AvailableMicros, &balance.HeldMicros, &balance.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, balance)
	}
	return out, rows.Err()
}

func (r *MySQLRepository) listUsageSummaries(ctx context.Context, filter TenantUsageFilter) ([]UsageSummary, error) {
	query := `
SELECT model, provider_type, channel_id, currency, COUNT(*),
       COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
       COALESCE(SUM(total_tokens), 0), COALESCE(SUM(amount_micros), 0)
FROM usage_records`
	where, args := reportWhere(filter.TenantID, filter.ProjectID, filter.Currency, filter.From, filter.To)
	query += where + ` GROUP BY model, provider_type, channel_id, currency ORDER BY model, provider_type, channel_id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageSummary
	for rows.Next() {
		var row UsageSummary
		if err := rows.Scan(
			&row.Model, &row.ProviderType, &row.ChannelID, &row.Currency, &row.Requests,
			&row.InputTokens, &row.OutputTokens, &row.TotalTokens, &row.AmountMicros,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *MySQLRepository) listLedger(ctx context.Context, filter TenantUsageFilter) ([]LedgerLine, error) {
	query := `
SELECT id, request_id, settlement_kind, tenant_id, project_id, account_id, currency,
       amount_micros, balance_after_micros, reason, created_at
FROM ledger_entries`
	where, args := reportWhere(filter.TenantID, filter.ProjectID, filter.Currency, filter.From, filter.To)
	query += where + ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, filter.Limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LedgerLine
	for rows.Next() {
		var line LedgerLine
		if err := rows.Scan(
			&line.ID, &line.RequestID, &line.SettlementKind, &line.TenantID, &line.ProjectID,
			&line.AccountID, &line.Currency, &line.AmountMicros, &line.BalanceAfterMicros,
			&line.Reason, &line.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, rows.Err()
}

func (r *MySQLRepository) providerCostProfiles(ctx context.Context) (map[string]ProviderCostProfile, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, provider_type, channel_id, public_model, currency, input_micros_per_token,
       output_micros_per_token, fixed_micros_per_request, effective_from, enabled, created_at, updated_at
FROM provider_cost_profiles
WHERE enabled = TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]ProviderCostProfile)
	for rows.Next() {
		profile, err := scanProviderCostProfile(rows)
		if err != nil {
			return nil, err
		}
		out[costProfileKey(profile.ProviderType, profile.ChannelID, profile.PublicModel, profile.Currency)] = *profile
	}
	return out, rows.Err()
}

func (r *MySQLRepository) getProviderCostProfile(ctx context.Context, providerType, channelID, model, currency string) (*ProviderCostProfile, error) {
	return scanProviderCostProfile(r.db.QueryRowContext(ctx, `
SELECT id, provider_type, channel_id, public_model, currency, input_micros_per_token,
       output_micros_per_token, fixed_micros_per_request, effective_from, enabled, created_at, updated_at
FROM provider_cost_profiles
WHERE provider_type = ? AND channel_id = ? AND public_model = ? AND currency = ?`,
		providerType, channelID, model, currency,
	))
}

func (r *MySQLRepository) listFailedSettlements(ctx context.Context) ([]FailedSettlementSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, request_id, tenant_id, project_id, hold_id, status, retry_count, next_retry_at,
       COALESCE(last_error, ''), COALESCE(owner_id, ''), claimed_at, heartbeat_at, updated_at
FROM failed_settlements
WHERE status IN (?, ?, ?)
ORDER BY updated_at DESC
LIMIT 100`, billing.FailedSettlementPending, billing.FailedSettlementFailed, billing.FailedSettlementProcessing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FailedSettlementSummary
	for rows.Next() {
		var row FailedSettlementSummary
		var claimedAt, heartbeatAt sql.NullTime
		if err := rows.Scan(
			&row.ID, &row.RequestID, &row.TenantID, &row.ProjectID, &row.HoldID, &row.Status,
			&row.RetryCount, &row.NextRetryAt, &row.LastError, &row.OwnerID, &claimedAt, &heartbeatAt, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if claimedAt.Valid {
			row.ClaimedAt = claimedAt.Time
		}
		if heartbeatAt.Valid {
			row.HeartbeatAt = heartbeatAt.Time
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *MySQLRepository) listUsageAttempts(ctx context.Context) ([]UsageAttemptSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT request_id, task_id, tenant_id, project_id, api_key_id, attempt_index, provider_type,
       channel_id, model, upstream_model, status_code, error_code, success, retryable,
       fallback_from_channel_id, final, created_at
FROM usage_attempts
WHERE final = TRUE OR success = FALSE OR task_id <> ''
ORDER BY created_at DESC
LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageAttemptSummary
	for rows.Next() {
		var row UsageAttemptSummary
		if err := rows.Scan(
			&row.RequestID, &row.TaskID, &row.TenantID, &row.ProjectID, &row.APIKeyID,
			&row.AttemptIndex, &row.ProviderType, &row.ChannelID, &row.Model, &row.UpstreamModel,
			&row.StatusCode, &row.ErrorCode, &row.Success, &row.Retryable,
			&row.FallbackFromChannelID, &row.Final, &row.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *MySQLRepository) getManualAdjustmentByKey(ctx context.Context, key string) (*ManualAdjustment, bool, error) {
	adjustment, err := scanManualAdjustment(r.db.QueryRowContext(ctx, `
SELECT id, idempotency_key, tenant_id, project_id, account_id, ledger_entry_id,
       currency, amount_micros, reason, operator_id, created_at
FROM manual_adjustments
WHERE idempotency_key = ?`, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return adjustment, true, nil
}

func scopeWhere(tenantID, projectID, currency string) (string, []any) {
	var clauses []string
	var args []any
	if tenantID != "" {
		clauses = append(clauses, "tenant_id = ?")
		args = append(args, tenantID)
	}
	if projectID != "" {
		clauses = append(clauses, "project_id = ?")
		args = append(args, projectID)
	}
	if currency != "" {
		clauses = append(clauses, "currency = ?")
		args = append(args, currency)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func reportWhere(tenantID, projectID, currency string, from, to time.Time) (string, []any) {
	where, args := scopeWhere(tenantID, projectID, currency)
	var clauses []string
	if where != "" {
		clauses = append(clauses, strings.TrimPrefix(where, " WHERE "))
	}
	if !from.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, from)
	}
	if !to.IsZero() {
		clauses = append(clauses, "created_at < ?")
		args = append(args, to)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func taskReportWhere(tenantID, projectID string, from, to time.Time) (string, []any) {
	var clauses []string
	var args []any
	if tenantID != "" {
		clauses = append(clauses, "t.tenant_id = ?")
		args = append(args, tenantID)
	}
	if projectID != "" {
		clauses = append(clauses, "t.project_id = ?")
		args = append(args, projectID)
	}
	if !from.IsZero() {
		clauses = append(clauses, "t.created_at >= ?")
		args = append(args, from)
	}
	if !to.IsZero() {
		clauses = append(clauses, "t.created_at < ?")
		args = append(args, to)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func selectBalanceForAdjustment(ctx context.Context, tx *sql.Tx, tenantID, projectID, currency string) (BalanceSummary, error) {
	var balance BalanceSummary
	err := tx.QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, currency, opening_micros, available_micros, held_micros, updated_at
FROM balance_accounts
WHERE tenant_id = ? AND project_id = ? AND currency = ?
FOR UPDATE`, tenantID, projectID, currency).Scan(
		&balance.AccountID, &balance.TenantID, &balance.ProjectID, &balance.Currency,
		&balance.OpeningMicros, &balance.AvailableMicros, &balance.HeldMicros, &balance.UpdatedAt,
	)
	return balance, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProviderCostProfile(row rowScanner) (*ProviderCostProfile, error) {
	var profile ProviderCostProfile
	err := row.Scan(
		&profile.ID, &profile.ProviderType, &profile.ChannelID, &profile.PublicModel,
		&profile.Currency, &profile.InputMicrosPerToken, &profile.OutputMicrosPerToken,
		&profile.FixedMicrosPerRequest, &profile.EffectiveFrom, &profile.Enabled,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func scanManualAdjustment(row rowScanner) (*ManualAdjustment, error) {
	var adjustment ManualAdjustment
	err := row.Scan(
		&adjustment.ID, &adjustment.IdempotencyKey, &adjustment.TenantID, &adjustment.ProjectID,
		&adjustment.AccountID, &adjustment.LedgerEntryID, &adjustment.Currency, &adjustment.AmountMicros,
		&adjustment.Reason, &adjustment.OperatorID, &adjustment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &adjustment, nil
}

var _ Repository = (*MySQLRepository)(nil)
