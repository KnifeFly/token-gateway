# P18 File Asset Boundary Runbook

## Scope

P18 keeps `/v1/files/*` as transient input asset metadata registration:

- URL registration stores `source_url` and metadata only.
- Base64 registration stores decoded size, MIME, and content hash metadata only.
- Stream upload stays disabled with `feature_not_enabled`.
- The gateway does not persist object bytes, proxy downloads, mint gateway-hosted file URLs, or provide storage lifecycle SLA.
- Quota counts only active metadata rows; expired rows are removed by the worker cleanup job.

## Operator Queries

Inspect active transient input asset metadata:

```sql
SELECT id, tenant_id, project_id, source, content_hash, source_url,
       transient, size_bytes, created_at, expires_at
FROM file_assets
WHERE tenant_id = 'tenant_id'
  AND project_id = 'project_id'
  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
ORDER BY created_at DESC
LIMIT 100;
```

Find expired metadata rows awaiting cleanup:

```sql
SELECT id, tenant_id, project_id, source, expires_at
FROM file_assets
WHERE expires_at IS NOT NULL
  AND expires_at <= CURRENT_TIMESTAMP
ORDER BY expires_at ASC
LIMIT 100;
```

Watch cleanup metrics:

```promql
sum by (outcome) (rate(token_gateway_file_cleanup_runs_total[15m]))
sum(rate(token_gateway_file_cleanup_deleted_total[15m]))
token_gateway_file_cleanup_max_age_seconds
token_gateway_file_cleanup_next_run_timestamp_seconds
```

## Focused Verification

```bash
go test ./internal/task ./internal/dataplane/parser ./internal/worker/jobs ./tests/contract
tests/failure/file_asset_boundary_drills.sh
```

## Full Verification

```bash
go test ./...
go vet ./...
git diff --check
```

## Expected Semantics

- `file_id` is a metadata identifier, not a durable downloadable object id.
- URL registration returns `source_url` and omits `file_url`/`download_url`.
- Base64 registration derives metadata and rejects oversized decoded input.
- Expired rows do not count toward active quota.
- Cleanup deletes expired transient metadata rows and related file idempotency records in bounded batches.
- Production egress guard must remain enabled for URL registration.
