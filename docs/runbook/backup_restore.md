# Backup / Restore Runbook

## Scope

This runbook covers commercial recovery for token-gateway state:

- MySQL control-plane configuration, runtime snapshots, billing ledger, usage records, task state, callback outbox, and failed settlements.
- Redis revocation and rate-limit state.
- Provider tasks that may still be running upstream.

## Backup Cadence

- MySQL full backup: daily.
- MySQL binlog / PITR: continuous, target RPO <= 15 minutes.
- Redis snapshot: daily for revocation state. Rate-limit counters are disposable.
- Runtime snapshot export: after every successful publish, keep the active and previous payload.
- Config secrets: back up encrypted provider credentials and the control-plane credential key together; a credential backup without the key is not restorable.

## Restore Order

1. Restore MySQL schema and data to the selected point in time.
2. Verify `ledger_entries`, `usage_records`, `balance_accounts`, `balance_holds`, `failed_settlements`, `tasks`, and `callback_outbox` row counts.
3. Restore the control-plane credential key and validate provider credential decryption on a non-production channel.
4. Restore Redis revocation keys if available. Do not restore old rate-limit counters.
5. Start control API first, then publish or activate the last known-good runtime snapshot.
6. Start gateway replicas after `/readyz` confirms DB, Redis, and active snapshot availability.
7. Start workers last, with failed-settlement replay and callback dispatch enabled.

## Financial Safety Checks

- Run reconciliation before accepting traffic:
  `GET /admin/reports/reconciliation`.
- Check `failed_settlements` for `pending` or `failed` rows and confirm replay ownership.
- Confirm every `manual_adjustment` has a matching `ledger_entries` row with `settlement_kind = manual_adjustment`.
- Do not replay provider requests for records that already have `usage_records` or `ledger_entries`.
- If a balance mismatch is detected, create an idempotent manual adjustment with an operator reason rather than editing balances directly.

## Task And Callback Recovery

- `succeeded` tasks with usage records are final and must not be re-dispatched.
- `running` or `queued` tasks with `provider_task_id` should be polled before re-dispatch.
- Callback retries can resume from `callback_outbox.next_retry_at`; do not regenerate callback payloads from raw prompts or responses.

## Disaster Recovery Drill

1. Restore a backup into a disposable environment.
2. Run migrations and start control API, gateway, and workers.
3. Publish the restored active snapshot.
4. Execute:
   - `GET /readyz`
   - `GET /admin/reports/reconciliation`
   - tenant usage report for a known tenant
   - provider profit report for a known provider/channel
5. Confirm no duplicate ledger entries were created during replay.
6. Record restore timestamp, snapshot version, reconciliation result, and failed-settlement backlog.
