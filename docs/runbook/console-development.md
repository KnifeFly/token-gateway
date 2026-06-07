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

## P23 Frontend Structure

Portal/Admin frontend apps keep the Vite root entrypoints, but app logic is split by role:

```text
web/apps/portal/src/main.tsx
web/apps/portal/src/App.tsx
web/apps/portal/src/app/
web/apps/portal/src/features/
web/apps/portal/src/shared/

web/apps/admin/src/main.tsx
web/apps/admin/src/App.tsx
web/apps/admin/src/app/
web/apps/admin/src/features/
web/apps/admin/src/shared/
```

Shared packages keep their root public exports stable while implementations are split:

```text
web/packages/api-client/src/fetcher/
web/packages/api-client/src/{admin-bff,portal-bff,client,errors}.ts
web/packages/auth/src/{session,csrf,permissions}.ts
web/packages/format/src/{date,number,money,tokens,status}.ts
web/packages/ui/src/{button,status-badge}.ts
web/packages/ui/src/primitives/
```

Do not move Portal/Admin business feature components into `web/packages/ui`, and do not add shared package imports back into `web/apps/*`.

## P24 Route Plan

P24 的 Admin/Portal 信息架构、中文文案策略、i18n 预留和 NewAPI 裁剪边界记录在 `docs/runbook/p24-console-route-plan.md`。后续渠道、模型、客户账户、API 密钥、日志、任务、操练场和额度页面应按该 route plan 接入。

For a local no-DB/no-Redis console render smoke:

```bash
TOKEN_GATEWAY_DATABASE_ENABLED=false \
TOKEN_GATEWAY_REDIS_ENABLED=false \
TOKEN_GATEWAY_LIMITS_ENABLED=false \
TOKEN_GATEWAY_BILLING_ENABLED=false \
go run ./cmd/console -config configs/local.yaml

go run ./tools/p24-console-smoke \
  -console-url http://127.0.0.1:9505 \
  -api-key tg-local-dev-key \
  -admin-email admin@example.com \
  -admin-password admin-local \
  -model gpt-4o-mini \
  -create-derived-key
```

For P22 production-console checks, use the same running console:

```bash
go run ./tools/p22-console-smoke \
  -console-url http://127.0.0.1:9505 \
  -api-key tg-local-dev-key \
  -admin-email admin@example.com \
  -admin-password admin-local
```
