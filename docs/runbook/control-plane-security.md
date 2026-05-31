# Control Plane Security Baseline

P14 hardens the existing static-token control plane enough for engineering readiness. It does not implement full RBAC, OIDC, mTLS, approval workflow, or a dedicated audit platform.

## Scope

Covered by this baseline:

- Constant-time static token comparison for control API requests.
- Control write body size limit.
- `Idempotency-Key` replay for control writes and conflict detection for reused keys with different bodies.
- Structured operator audit log events for mutating control operations.
- Network isolation requirements for control-api and configd.
- Opt-in trusted proxy parsing for public data-plane client IP.

Not covered:

- User identity lifecycle, RBAC policies, SSO, mTLS client identity, or approval chains.
- A durable external audit store. P14 writes structured logs that operators must ship to the production log backend.

## Required Production Settings

Set these through the deployment secret manager:

```bash
TOKEN_GATEWAY_CONTROL_ADMIN_TOKEN=...
TOKEN_GATEWAY_CONTROL_CREDENTIAL_KEY=...
TOKEN_GATEWAY_API_KEY_HASH_SECRET=...
```

`TOKEN_GATEWAY_API_KEY_HASH_SECRET` must not be empty or the local default outside local/test environments. New customer API keys are stored with the `hmac-sha256:` prefix. Existing `sha256:` hashes remain valid during the migration window.

## Network Isolation

Expose public data-plane routes separately from control routes.

- Gateway public port: customer ingress only.
- Control API port: private admin network, VPN, bastion, or equivalent internal ingress only.
- Configd port: private admin/operator network only.
- Database and Redis: not exposed to customer networks.

Recommended ingress behavior:

- Terminate TLS before the service or at the service boundary.
- Strip untrusted forwarded headers at the edge.
- Rate-limit control-plane paths separately from public API paths.
- Ship `control_plane_audit_event` logs to the production log backend.

## Control Write Idempotency

For mutating control requests, operators should provide `Idempotency-Key`:

```bash
curl -sS http://127.0.0.1:9502/admin/api-keys \
  -H "X-Admin-Token: ${TOKEN_GATEWAY_CONTROL_ADMIN_TOKEN}" \
  -H "Idempotency-Key: create-key-20260531-001" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"tenant_1","project_id":"project_1","name":"sdk"}'
```

Behavior:

- Same method, path, key, and body replays the original response.
- Same method, path, and key with a different body returns `idempotency_conflict`.
- Failed 5xx responses are not cached.

## Trusted Proxy Client IP

Forwarded client IP headers are ignored by default. Configure trusted proxies only for the direct proxy addresses that connect to the gateway:

```yaml
http:
  trusted_proxy_cidrs:
    - 10.0.0.0/8
    - 2001:db8::/32
```

Equivalent environment variable:

```bash
TOKEN_GATEWAY_HTTP_TRUSTED_PROXY_CIDRS=10.0.0.0/8,2001:db8::/32
```

Rules:

- If `RemoteAddr` is not in a trusted CIDR, `X-Forwarded-For` and `X-Real-IP` are ignored.
- If `RemoteAddr` is trusted, the first valid `X-Forwarded-For` IP is used.
- If `X-Forwarded-For` is missing, `X-Real-IP` is used.
- If neither header has a valid IP, the direct remote IP is used.

This resolved IP feeds the data-plane `ClientIP` value used by the IP allowlist plugin.
