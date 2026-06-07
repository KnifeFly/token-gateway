# token-gateway

`token-gateway` is a commercial AI Gateway, not a thin provider proxy. It exposes one customer-facing API surface and routes requests to multiple upstream providers while keeping tenant/project/API-key identity, routing policy, billing, ledger repair, callbacks, limits, and observability in one auditable flow.

Authoritative design and execution state live in:

- `CLAUDE.md` for repository rules and command conventions.
- `docs/tasks.md` for the active task board and latest verification status.
- `docs/plan/` for milestone and remediation plans.
- `docs/design/` for system, architecture, blueprint, and OpenAPI truth.

## Local Start

The default local config is `configs/local.yaml`.

```bash
make compose-up
make migrate-up
go run ./cmd/configd -config configs/local.yaml
go run ./cmd/control-api -config configs/local.yaml
go run ./cmd/gateway -config configs/local.yaml
go run ./cmd/console -config configs/local.yaml
```

Optional worker process:

```bash
go run ./cmd/worker -config configs/local.yaml
```

Console frontend workspace:

```bash
pnpm install --frozen-lockfile
pnpm generate:api
pnpm --filter @token-gateway/portal dev
pnpm --filter @token-gateway/admin dev
```

Local console routes:

```text
console process  http://127.0.0.1:9505
Portal SPA       http://127.0.0.1:9511/portal/
Admin SPA        http://127.0.0.1:9512/admin-ui/
Vite proxy       /api/* -> http://localhost:9505/api/*
```

Health checks:

```bash
curl -fsS http://127.0.0.1:9501/healthz
curl -fsS http://127.0.0.1:9501/readyz
curl -fsS http://127.0.0.1:9501/metrics
curl -fsS http://127.0.0.1:9505/healthz
```

Minimal chat request against the local mock OpenAI-compatible channel:

```bash
curl -sS http://127.0.0.1:9501/v1/chat/completions \
  -H 'Authorization: Bearer tg-local-dev-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

## Supported API Surface

Current implemented public surfaces include:

- OpenAI-compatible chat completions, Responses, embeddings, moderations, image generation/edit, audio speech, and audio transcription.
- Claude Messages and Gemini GenerateContent-compatible native paths.
- Unified media task APIs for image, video, audio, music, files, task query/control, callbacks, and task settlement.
- Portal APIs for model catalog/schema, credits, usage, API key self-service, and task lookup.
- Realtime reserved extension returns stable disabled-contract responses unless explicitly enabled by a later milestone.

See `docs/design/ai_gateway_openapi.yaml` and `docs/runbook/provider-protocol-compatibility.md` for protocol details.
P19 introduces split contracts under `api/openapi/`; `docs/design/ai_gateway_openapi.yaml` remains the compatibility aggregate during the transition.

## File Input Boundary

`/v1/files/*` is a transient input asset metadata registry. It records URL/base64 metadata for request normalization, idempotency, quota protection, provider forwarding, and audit. It does not store object bytes, proxy downloads, produce gateway-hosted file URLs, or provide media lifecycle/storage SLA. `/v1/files/upload/stream` remains disabled until a separate object-reference/spool stage is designed.

## Configuration

Important local and production settings:

- `gateway.auth.api_key_hash_secret` or `TOKEN_GATEWAY_API_KEY_HASH_SECRET`: server secret for customer API key HMAC-SHA256 hashes. Local/test can use the local default; non-local environments fail closed if this remains empty or the local default.
- `control.admin_token` or `TOKEN_GATEWAY_CONTROL_ADMIN_TOKEN`: static control-plane token. It is a P14 baseline, not full RBAC/OIDC/mTLS.
- `control.credential_key` or `TOKEN_GATEWAY_CONTROL_CREDENTIAL_KEY`: encrypts provider credentials at rest.
- `http.trusted_proxy_cidrs` or `TOKEN_GATEWAY_HTTP_TRUSTED_PROXY_CIDRS`: comma-separated CIDRs whose `X-Forwarded-For`/`X-Real-IP` headers are trusted. Empty by default.
- `gateway.billing.*`: local pricing, hold TTL, and seed balance controls.
- `gateway.limits.*`: Redis-backed RPM/QPS/TPM/concurrency/budget controls.
- `gateway.egress.*`: outbound URL safety controls for media inputs and callbacks.

Customer API keys are never stored in plaintext. New keys are stored as `hmac-sha256:` hashes; old `sha256:` hashes remain readable during the compatibility window.

## Billing And Repair

Billable requests reserve balance before provider dispatch, write provider attempts, settle usage and ledger entries after provider success, and persist failed settlements for worker repair. Streaming requests settle at stream close with a bounded background timeout so client disconnects do not cancel accounting and slow settlement does not block forever.

Worker jobs cover failed settlement replay, provider task polling, callback delivery, hold reaping, and reconciliation. Run the worker in production-like environments whenever billing or async media tasks are enabled.

## Security Baseline

- Public data-plane auth uses customer API keys only; plaintext keys must not appear in logs, metrics, traces, or errors.
- Control API and configd must be reachable only from private admin networks or a hardened ingress layer. P14 static-token hardening does not replace RBAC/OIDC/mTLS.
- Control write APIs support `Idempotency-Key`; reuse with a different body returns `idempotency_conflict`.
- Trusted proxy parsing is opt-in. Without trusted CIDRs, forwarded headers are ignored.
- Egress guard should stay enabled for URL and callback flows.

Additional control-plane deployment notes: `docs/runbook/control-plane-security.md`.

## Verification

Common local checks:

```bash
go test ./...
go vet ./...
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm generate:api
git diff --check
```

Release-oriented checks and customer smoke workflows are documented in `docs/runbook/release-handoff.md`, `docs/runbook/customer-acceptance.md`, and `tests/rc/clean_env_smoke.sh`.

## Known Limits

- Full RBAC/OIDC/mTLS control-plane identity is not implemented in this route.
- Full Realtime WebSocket/WebRTC/provider adapter support is out of scope; only the disabled contract and reserved interfaces exist.
- The gateway does not provide object storage or media asset lifecycle guarantees.
- Dynamic/WASM plugins, semantic routing/cache, multi-region active-active, invoices, and complex finance operations are not in the current route.
