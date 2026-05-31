# P17 Worker Callback Stability Runbook

## Scope

P17 hardens the worker and callback loop for multi-replica operation:

- Worker leases are renewed by heartbeat while jobs run.
- `MaxConcurrency()` now controls runner slots; provider polling stays single-slot and callback dispatch defaults to four slots.
- Provider task polling isolates per-task provider/settlement/completion errors and continues the batch.
- Callback rows are durably claimed with `processing`, `owner_id`, `claimed_at`, `heartbeat_at`, and `delivery_id`.
- Callback HTTP delivery drains and closes response bodies, records status and latency, and signs timestamp plus delivery id plus payload.

## Operator Queries

Inspect callback ownership and retry state:

```sql
SELECT id, task_id, status, owner_id, claimed_at, heartbeat_at,
       delivery_id, retry_count, next_retry_at, last_status_code,
       last_latency_ms, last_error
FROM callback_outbox
WHERE status IN ('pending', 'processing', 'dead_letter')
ORDER BY updated_at DESC
LIMIT 100;
```

Find stale processing rows that are eligible for reclaim after the configured claim timeout:

```sql
SELECT id, task_id, owner_id, heartbeat_at, retry_count, last_error
FROM callback_outbox
WHERE status = 'processing'
  AND heartbeat_at < NOW() - INTERVAL 2 MINUTE
ORDER BY heartbeat_at ASC;
```

Watch the new Prometheus signals:

```promql
sum by (job, outcome) (rate(token_gateway_worker_lease_heartbeats_total[5m]))
sum by (status_class, outcome) (rate(token_gateway_callback_deliveries_total[5m]))
sum by (reason) (rate(token_gateway_callback_retries_total[5m]))
```

## Focused Verification

```bash
go test ./internal/worker ./internal/worker/jobs ./internal/task ./internal/bootstrap
tests/failure/worker_callback_drills.sh
```

## Full Verification

```bash
go test ./...
go vet ./...
git diff --check
```

## Expected Semantics

- A job running near or beyond the original lease TTL renews its lease before another worker can acquire the same slot.
- Heartbeat failure cancels the current job context and records a heartbeat error metric.
- Callback dispatcher claims rows before delivery; another owner cannot deliver the same row until the claim timeout expires.
- Failed callback attempts clear `owner_id`, increment retry count, keep the delivery id, and move back to `pending` or `dead_letter`.
- Provider task poll errors affect only the current task; later tasks in the same batch continue.
