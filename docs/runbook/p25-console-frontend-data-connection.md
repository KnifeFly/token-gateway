# P25 Console Frontend Data Connection Runbook

P25 hardens the existing P24 Portal/Admin console frontend. It connects Admin production panels to the real browser BFF, adds Portal route/list state handling, extracts minimal shared UI primitives, and aligns Portal/Admin visual style to a light SiliconFlow-like cloud console theme.

P25 does not add NewAPI cut-scope modules such as groups, ratios, subscriptions, payments, redemptions, referral, model deployment, or broad system settings.

## Local No-DB Console

Start the console against the local seed snapshot:

```bash
TOKEN_GATEWAY_CONSOLE_ADDR=:9515 \
TOKEN_GATEWAY_DATABASE_ENABLED=false \
TOKEN_GATEWAY_REDIS_ENABLED=false \
TOKEN_GATEWAY_LIMITS_ENABLED=false \
TOKEN_GATEWAY_BILLING_ENABLED=false \
go run ./cmd/console -config configs/local.yaml
```

Rebuild frontend assets before browser verification:

```bash
pnpm build
```

## Automated Smoke

Run the existing P24 smoke with P25 frontend expectations:

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

## Browser Smoke

Admin browser checks:

- Directly open `/admin-ui/workbench`, `/admin-ui/routing`, `/admin-ui/catalog`, `/admin-ui/accounts`, `/admin-ui/activity`, `/admin-ui/tools`, and `/admin-ui/settings`.
- Confirm Admin login restores the requested route and browser back/forward keeps the correct view.
- Confirm Channel, Model, Customer Account, API Key, Activity, Credit Operations, and Tools pages render BFF data from `http://127.0.0.1:9515/api/admin/v1`.
- Confirm Workbench uses the shared `ConfirmDialog`, Model schema preview uses `Drawer`, Activity IDs use `CopyButton`, and loading/empty states come from shared UI primitives.

Portal browser checks:

- Directly open `/portal/usage`, log in with `tg-local-dev-key`, and confirm the route is preserved.
- Open `/portal/models`, `/portal/playground`, `/portal/api-keys`, `/portal/usage`, `/portal/tasks`, `/portal/credits`, and `/portal/settings`.
- Confirm usage/tasks/ledger limit controls render as shared `Pagination` inside `FilterBar` and trigger a reload with the selected limit.
- Confirm browser back/forward between `/portal/tasks` and `/portal/credits` restores the correct view.

Visual checks:

- Portal/Admin both use a light 200px sidebar, purple primary action color, compact panels/tables/tags/filter controls, 8px radii, and low shadows.
- Do not copy SiliconFlow logo, brand assets, or marketing copy.

## Guardrails

Run the static and contract guardrails before release:

```bash
pnpm typecheck
pnpm lint
pnpm test
pnpm build
make p24-cut-scope-check
make boundary-check
make api-check
go test ./internal/app/admin/... ./internal/transport/adminhttp ./internal/app/portal/... ./internal/transport/portalwebhttp ./tests/contract ./tools/release-handoff
```

## Release Evidence

Record these fields in release handoff:

| Field | Example |
|---|---|
| P25 console frontend smoke result | `p24_console_smoke=passed`, Browser Admin/Portal route/list/style smoke passed |
| P24 cut-scope result | `make p24-cut-scope-check` passed |
| API drift result | `make api-check` passed |
| Known limits | no-DB smoke uses local seed snapshot; DB/Redis/billing/limits are disabled |
