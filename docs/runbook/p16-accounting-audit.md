# P16 Accounting Audit Runbook

## Scope

P16 makes sync and async provider accounting auditable with the same durable signals:

- Async idempotency duplicate races replay the existing task or return the stable request-hash conflict.
- Async provider submit attempts are written to `usage_attempts` with `task_id`, provider/channel, public model, upstream model, fallback source, retryability, and final status.
- Sync dispatcher final failed attempts are durable even when elapsed retry budget stops fallback after the first failed attempt.
- Failed settlement replay claims rows with `processing`, `owner_id`, `claimed_at`, and `heartbeat_at` before repair work.
- Redis `daily_admission_budget_micros` is an estimated admission guard. Actual customer spend is explained by `usage_records`, `ledger_entries`, and reconciliation.

`daily_budget_micros` remains a compatibility alias for existing control-plane and local config input. New configuration should use `daily_admission_budget_micros`.

## Operator Queries

Find async attempt audit rows:

```sql
SELECT request_id, task_id, attempt_index, provider_type, channel_id,
       model, upstream_model, error_code, retryable,
       fallback_from_channel_id, final, created_at
FROM usage_attempts
WHERE task_id <> ''
ORDER BY created_at DESC
LIMIT 100;
```

Find final failed attempts:

```sql
SELECT request_id, task_id, attempt_index, provider_type, channel_id,
       error_code, retryable, final, created_at
FROM usage_attempts
WHERE final = TRUE AND success = FALSE
ORDER BY created_at DESC
LIMIT 100;
```

Inspect failed settlement claims:

```sql
SELECT id, request_id, status, owner_id, claimed_at, heartbeat_at,
       retry_count, next_retry_at, last_error
FROM failed_settlements
WHERE status IN ('pending', 'failed', 'processing')
ORDER BY updated_at DESC
LIMIT 100;
```

Compare actual spend:

```sql
SELECT tenant_id, project_id, currency, SUM(amount_micros) AS actual_spend_micros
FROM usage_records
GROUP BY tenant_id, project_id, currency;

SELECT tenant_id, project_id, currency, -SUM(amount_micros) AS ledger_debit_micros
FROM ledger_entries
WHERE settlement_kind = 'usage_debit'
GROUP BY tenant_id, project_id, currency;
```

## Focused Verification

```bash
go test ./internal/task ./internal/dataplane/dispatch ./internal/billing ./internal/billing/reporting
go test ./internal/bootstrap ./internal/dataplane/limit ./internal/controlplane/configadmin ./internal/transport/controlhttp
```

## Full Verification

```bash
go test ./...
go vet ./...
git diff --check
```

## Expected Semantics

- Same tenant/API key/endpoint/idempotency key with the same request hash returns the original task after a duplicate create race.
- Same idempotency key with a different request hash returns `idempotency_conflict`.
- Async fallback produces one durable attempt per candidate that was submitted or evaluated.
- Exactly one failed settlement worker owns a `processing` row at a time; stale claims can be reclaimed after the claim timeout.
- Admission budget rejection means "estimated cost would exceed the Redis guard", not "actual ledger spend exceeded a hard invoice budget".
