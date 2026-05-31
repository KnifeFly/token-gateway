# P3 Production Hardening Runbook

## Scope

This runbook verifies the P3 production semantics for:

- Redis rate limiting: token buckets, TPM pre-charge, daily admission budget guard, cost-per-minute, and concurrency leases.
- Billability policy across sync, stream, and async task settlement.
- Native OpenAI images/audio routing and model rewrite.
- Unified Media provider task adapter registration.
- Redis RouteSignals.
- Configd snapshot publish, watch, rollback, diagnostics, restart, and gateway stale policy.

## Focused Verification

```bash
go test ./internal/dataplane/limit
go test ./internal/billing ./internal/task ./internal/worker/jobs ./internal/dataplane/stream
go test ./internal/provider/openai ./internal/dataplane/classifier ./internal/dataplane/parser ./internal/dataplane/dispatch
go test ./internal/task ./internal/bootstrap ./internal/worker/jobs
go test ./internal/dataplane/router ./internal/infra/redis ./internal/bootstrap
go test ./internal/transport/configdhttp ./internal/controlplane/snapshot ./internal/dataplane/snapshot ./internal/bootstrap
```

## Full Verification

```bash
go test ./...
make lint
make build
git diff --check
```

## Redis Integration

Start Redis locally or through compose, then run:

```bash
TOKEN_GATEWAY_REDIS_ADDR=127.0.0.1:6379 go test ./internal/dataplane/limit -run TestRedisEnforcerIntegrationCoversP1LimitTypes -count=1
```

Expected result: the first acquire succeeds, the second request is denied by the shared Redis state, and the concurrency lease can be released.

## Compose Smoke

```bash
make compose-up
go run ./cmd/gateway -config configs/local.yaml
curl -fsS http://127.0.0.1:9501/healthz
curl -fsS http://127.0.0.1:9501/readyz
make compose-down
```

When DB-backed configd is enabled in the environment, also run:

```bash
go run ./cmd/configd -config configs/local.yaml
curl -fsS -X POST -H "X-Admin-Token: local-admin-token" http://127.0.0.1:9504/configd/snapshots/publish
curl -fsS -H "Authorization: Bearer local-admin-token" http://127.0.0.1:9504/configd/snapshots/diagnostics
```

## Failure Drills

```bash
GATEWAY_URL=http://127.0.0.1:9501 API_KEY=tg-local-dev-key make failure-drills
go run ./tools/loadtest -url http://127.0.0.1:9501/v1/chat/completions -api-key tg-local-dev-key -requests 1000 -concurrency 100
```

Record snapshot version, Redis integration result, load-test p95/p99 latency, failed-settlement backlog, and any skipped external dependency.
