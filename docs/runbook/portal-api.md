# Portal API Runbook

## Scope

Portal API is customer self-service API under `/v1/portal/*`. It reuses the same Bearer API key authentication as data-plane requests and never accepts admin tokens. The authenticated API key defines the tenant, project, current API key id, and allowed model set.

Portal is not a second control plane. It does not expose provider channels, routes, prices, limits, plugins, snapshots, emergency actions, provider secrets, or internal settlement repair state.

## Endpoints

| Endpoint | Behavior |
|---|---|
| `GET /v1/portal/models` | Lists enabled models visible to the current API key. |
| `GET /v1/portal/models/{model}/schema` | Returns dynamic schema only when the current API key can access that model or alias. |
| `GET /v1/portal/credits` | Returns customer-facing remaining, used, held credits and currency for the current tenant/project. |
| `GET /v1/portal/usage` | Returns customer-visible usage/charge summaries for the current tenant/project. Provider cost and repair internals are omitted. |
| `GET /v1/portal/api-keys` | Lists safe API key metadata in the current tenant/project. Plaintext keys and hashes are never returned. |
| `POST /v1/portal/api-keys` | Creates a derived key in the current tenant/project. `allowed_models` must be a subset of the caller key. |
| `POST /v1/portal/api-keys/{key_id}/disable` | Disables a same-tenant/project key other than the caller key. |
| `GET /v1/portal/tasks` | Lists async tasks in the current tenant/project. |
| `GET /v1/portal/tasks/{task_id}` | Returns one same-tenant/project task and filters sensitive metadata keys. |

## Permission Rules

- The caller cannot expand model permissions. If the current key is limited to `["gpt-public"]`, derived keys cannot request `image-public` or `*`.
- A Portal caller cannot disable its own current key.
- Cross-tenant or cross-project API key and task access returns a standard not-found or permission error without exposing whether another tenant owns the resource.
- Usage and credits use customer-facing totals only. Do not add provider cost, failed settlement details, raw ledger repair state, or plaintext secrets to Portal responses.

## Validation

Run these focused checks after touching Portal behavior:

```bash
go test ./internal/app/portal/... ./internal/transport/portalhttp ./internal/task ./internal/bootstrap
go test ./tools/portal-smoke ./tests/contract
GATEWAY_URL=http://127.0.0.1:9501 API_KEY=tg-rc-key make portal-smoke
go test ./...
```

The Portal handler tests cover model visibility, schema access, credits/usage redaction, API key subset enforcement, self-disable prevention, cross-project denial, task scope filtering, sensitive metadata filtering, and OpenAPI contract coverage.

For customer acceptance and RC evidence, see [Customer Acceptance Runbook](./customer-acceptance.md).
