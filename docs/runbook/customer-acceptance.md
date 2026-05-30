# Customer Acceptance Runbook

## Scope

This runbook verifies the customer-facing surface after P8 Portal API. It uses customer Bearer API keys and public gateway endpoints. Do not use admin tokens for the Portal smoke.

P9 does not add a new product surface. It closes the customer acceptance loop for Portal, OpenAPI import preflight, and RC smoke.

## Portal Smoke

Run against a live gateway:

```bash
GATEWAY_URL=http://127.0.0.1:9501 API_KEY=tg-rc-key make portal-smoke
```

Run the full Portal API key lifecycle in staging or RC:

```bash
go run ./tools/portal-smoke \
  -gateway-url http://127.0.0.1:9501 \
  -api-key tg-rc-key \
  -model gpt-4o-mini \
  -create-derived-key
```

The smoke validates:

- `GET /v1/portal/models`
- `GET /v1/portal/models/{model}/schema`
- `GET /v1/portal/credits`
- `GET /v1/portal/usage`
- `GET /v1/portal/api-keys`
- optional `POST /v1/portal/api-keys` and disable lifecycle
- `GET /v1/portal/tasks`

It also rejects response markers such as `provider_cost`, `failed_settlement`, `plaintext_key` in list responses, `key_hash`, provider secrets, passwords, and credentials.

## RC Smoke

The clean dependency smoke now runs the Portal customer acceptance step after configd publishes a snapshot and before rollback:

```bash
tests/rc/clean_env_smoke.sh
```

For syntax-only validation:

```bash
bash -n tests/rc/clean_env_smoke.sh
```

## OpenAPI Import Preflight

Run the contract preflight before importing the OpenAPI file into Apifox or another client tool:

```bash
go test ./tests/contract -run TestOpenAPIImportPreflight
```

The preflight verifies that the OpenAPI document parses, local `$ref` targets resolve, operation IDs are unique, security schemes exist, and Portal operations require or inherit `bearerAuth`.

## Expected Evidence

A successful customer acceptance closeout should include:

- `portal_smoke=passed`
- `go test ./tools/portal-smoke ./tests/contract`
- `go test ./...`
- `bash -n tests/rc/clean_env_smoke.sh`
- If a real RC run is required, `tests/rc/clean_env_smoke.sh` output with `rc_smoke=portal_customer_acceptance` and `rc_smoke=passed`
