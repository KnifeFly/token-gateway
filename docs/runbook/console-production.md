# Console Production Runbook

P22 turns the Portal/Admin browser console into a production-deliverable surface. It does not add object storage, payments, invoices, complete Realtime, dynamic plugins, or multi-region operations.

## Local Production Smoke

Start a no-DB console for browser/BFF checks:

```bash
TOKEN_GATEWAY_CONSOLE_ADDR=:9515 \
TOKEN_GATEWAY_DATABASE_ENABLED=false \
TOKEN_GATEWAY_REDIS_ENABLED=false \
TOKEN_GATEWAY_LIMITS_ENABLED=false \
TOKEN_GATEWAY_BILLING_ENABLED=false \
go run ./cmd/console -config configs/local.yaml
```

Run the P22 production smoke:

```bash
go run ./tools/p22-console-smoke \
  -console-url http://127.0.0.1:9515 \
  -api-key tg-local-dev-key \
  -admin-email admin@example.com \
  -admin-password admin-local
```

Expected final marker:

```text
p22_console_smoke=passed
```

## Staging Smoke

Use a scoped non-production Admin operator and a disposable customer key:

```bash
go run ./tools/p22-console-smoke \
  -console-url "${CONSOLE_URL}" \
  -api-key "${API_KEY}" \
  -admin-email "${ADMIN_EMAIL}" \
  -admin-password "${ADMIN_PASSWORD}"
```

The smoke validates:

- `/admin-ui/*` static shell security headers and HTML cache policy.
- Admin login, session cookie, dashboard, tenants, projects, models, channels, routes, pricing, limits, snapshots, operations, audit, operators, CSRF denial, snapshot validate, and logout.
- Portal API key login, dashboard, API keys, usage, tasks, CSRF denial, and logout.

## Static Assets

Preferred production deployment:

```text
CDN/Nginx:
  /portal/*
  /admin-ui/*

cmd/console:
  /api/portal/v1/*
  /api/admin/v1/*
```

Simplified deployment can serve Vite dist directories from `cmd/console`:

```text
/portal/*      -> web/apps/portal/dist
/admin-ui/*    -> web/apps/admin/dist
/api/*         -> browser BFF
```

The repository Docker image builds the Portal/Admin dist artifacts and includes the
`console` binary. The local Compose stack exposes it as the `console` service:

```bash
docker compose config
docker compose up console
```

Cache policy:

- HTML and SPA fallbacks: `Cache-Control: no-cache`
- Hashed JS/CSS/assets: `Cache-Control: public, max-age=31536000, immutable`

## Security Headers

Console responses must include:

- `Content-Security-Policy` with `frame-ancestors 'none'`
- `Strict-Transport-Security`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: no-referrer`
- `X-Frame-Options: DENY`

Admin and Portal session cookies are HttpOnly. In HTTPS deployments they must be Secure and SameSite. Mutations require CSRF, and Admin mutations also require reason and idempotency evidence.

## Operator Bootstrap

Local no-DB mode bootstraps:

```text
admin@example.com / admin-local
```

Staging and production should provision explicit operator accounts through the Admin owner workflow. Do not reuse the machine control-plane admin token as a browser credential.

## Session Secret Rotation

Rotate browser session and CSRF secrets by:

1. Deploying the new secret to console instances.
2. Restarting console instances in a rolling window.
3. Expecting existing browser sessions to be invalidated.
4. Running `p22-console-smoke` with a fresh login.

## Rollback

For frontend-only rollback:

1. Restore the previous `/portal/*` and `/admin-ui/*` dist artifact.
2. Purge CDN/Nginx HTML cache.
3. Keep hashed old assets available until all no-cache HTML references have aged out.
4. Run `p22-console-smoke`.

For config/runtime rollback, prefer snapshot rollback before code rollback.

## Release Evidence

Record these fields in release handoff:

| Field | Expected evidence |
|---|---|
| P22 console production smoke result | `p22_console_smoke=passed` |
| Static cache policy | HTML no-cache and hashed asset immutable check |
| Security header check | P22 smoke and Go tests passed |
| Admin E2E | login, snapshot validate, audit, logout |
| Portal E2E | login, dashboard, API keys, usage, tasks, logout |
| Rollback/cache purge | Artifact rollback or CDN purge command tested |
