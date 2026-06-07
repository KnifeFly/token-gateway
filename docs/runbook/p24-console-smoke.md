# P24 Console Smoke Runbook

P24 console smoke validates the lean NewAPI parity surface without turning the product into a NewAPI clone. It covers Admin channels, models, customer accounts, API keys, activity logs, playground, and Portal models, playground, API keys, usage, tasks, credits, ledger, and export.

## Local No-DB Smoke

Start a disposable console process with local memory stores:

```bash
TOKEN_GATEWAY_CONSOLE_ADDR=:9515 \
TOKEN_GATEWAY_DATABASE_ENABLED=false \
TOKEN_GATEWAY_REDIS_ENABLED=false \
TOKEN_GATEWAY_LIMITS_ENABLED=false \
TOKEN_GATEWAY_BILLING_ENABLED=false \
go run ./cmd/console -config configs/local.yaml
```

Run the P24 smoke:

```bash
go run ./tools/p24-console-smoke \
  -console-url http://127.0.0.1:9515 \
  -api-key tg-local-dev-key \
  -admin-email admin@example.com \
  -admin-password admin-local \
  -model gpt-4o-mini \
  -create-derived-key
```

Expected final marker:

```text
p24_console_smoke=passed
```

## Staging Smoke

Use a disposable customer API key and a non-production Admin operator:

```bash
go run ./tools/p24-console-smoke \
  -console-url "${CONSOLE_URL}" \
  -api-key "${API_KEY}" \
  -admin-email "${ADMIN_EMAIL}" \
  -admin-password "${ADMIN_PASSWORD}" \
  -model "${MODEL}" \
  -seed-fixtures=false
```

Use `-seed-fixtures=false` in shared environments unless the operator explicitly wants the smoke to create minimal Admin model/channel/customer fixtures when those lists are empty.

## Coverage

- Admin login/session/CSRF.
- Admin channel list/detail/test.
- Admin model list/detail/schema preview.
- Admin customer account detail, credit report, usage export, and ledger export.
- Admin API key list and safe metadata.
- Admin usage/task log list and optional drilldown.
- Admin playground safe run.
- Portal API key login/session/CSRF.
- Portal model list/detail/schema.
- Portal playground safe run.
- Portal credits and ledger.
- Portal API key list and optional create/disable lifecycle.
- Portal usage, usage export, and task list.

## Known Limits

- The smoke validates BFF routes and safe DTOs; it does not call real upstream providers.
- Playground checks are safe dry-runs and assert prompt/key material is not echoed.
- Payment, subscription, redemption, invite reward, model deployment, group, and ratio modules remain out of scope and are guarded by `make p24-cut-scope-check`.
- Staging runs need a disposable customer key and a scoped Admin operator. Do not use production customer credentials.

## Release Evidence

Record these fields in release handoff:

| Field | Expected evidence |
|---|---|
| P24 console smoke result | `p24_console_smoke=passed` |
| P24 cut-scope result | `P24 cut-scope check passed` |
| Console URL | Target console base URL |
| Admin operator | Non-production operator email |
| Customer key scope | Tenant/project/API key ID from smoke output |
| Known limits | Copy the Known Limits section above if unchanged |
