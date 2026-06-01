# Console Development

P19 adds the Human Console Plane without moving existing machine APIs.

## Route Ownership

| Route | Process | Notes |
|---|---|---|
| `/v1/*` | `cmd/gateway` | Data Plane public API |
| `/v1/portal/*` | `cmd/gateway` | Programmatic Portal API with customer Bearer API key |
| `/admin/*` | `cmd/control-api` | Machine Control API, not a browser API |
| `/api/portal/v1/*` | `cmd/console` | Portal Web BFF, session/CSRF in P20 |
| `/api/admin/v1/*` | `cmd/console` | Admin Web BFF, operator session/RBAC/audit in P21 |
| `/portal/*` | Vite dev server or `cmd/console` static | Portal SPA |
| `/admin-ui/*` | Vite dev server or `cmd/console` static | Admin SPA |

## Local Process Ports

```text
gateway      9501
control-api  9502
worker       9503
configd      9504
console      9505
portal vite  9511
admin vite   9512
```

## Startup

```bash
make compose-up
make migrate-up
go run ./cmd/configd -config configs/local.yaml
go run ./cmd/control-api -config configs/local.yaml
go run ./cmd/gateway -config configs/local.yaml
go run ./cmd/console -config configs/local.yaml
```

Frontend dev servers proxy `/api/*` to the console process:

```bash
pnpm install --frozen-lockfile
pnpm generate:api
pnpm --filter @token-gateway/portal dev
pnpm --filter @token-gateway/admin dev
```

## Checks

```bash
make run-console
curl -fsS http://127.0.0.1:9505/healthz
curl -i http://127.0.0.1:9505/api/portal/v1/dashboard
curl -i http://127.0.0.1:9505/api/admin/v1/dashboard
pnpm lint
pnpm typecheck
pnpm test
pnpm build
make api-check
```

P19 BFF handlers intentionally return `501 not_implemented` until P20/P21 attach session services and use-case handlers.
