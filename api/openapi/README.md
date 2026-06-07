# OpenAPI Contracts

`api/openapi/` is the forward source directory for split API contracts. During the P19 transition, `docs/design/ai_gateway_openapi.yaml` remains the compatibility aggregate for existing public and control-plane checks.

## Contract Ownership

| Contract | Process | Path ownership | Consumer |
|---|---|---|---|
| `gateway-public.yaml` | `cmd/gateway` | `/v1/*` except `/v1/portal/*` | SDKs and customer automation |
| `portal-public.yaml` | `cmd/gateway` | `/v1/portal/*` | customer automation |
| `portal-bff.yaml` | `cmd/console` | `/api/portal/v1/*` | Portal SPA |
| `admin-bff.yaml` | `cmd/console` | `/api/admin/v1/*` | Admin SPA |
| `control.yaml` | `cmd/control-api` | `/admin/*` | internal automation |

P19 only introduces the split baseline and generated TypeScript types for the BFF contracts. P20 and P21 fill the Portal/Admin BFF operations without moving `/v1/*`, `/v1/portal/*`, or `/admin/*` out of their existing processes.

## Generation

Frontend types are generated from the BFF contracts:

```bash
pnpm generate:api
```

The generated files are committed under `web/packages/api-client/src/generated/`. `make api-check` regenerates them and fails if the generated output drifts.
