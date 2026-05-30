# Provider Reliability Runbook

P6 keeps provider reliability in the data-plane hot path:

- Route signals now include success rate, error rate, 429 count, 5xx count, timeout count, stream interruption count, latency, manual disable, and circuit state.
- The in-process circuit breaker is keyed by provider type, channel ID, and public model. It opens on retryable provider reliability failures, allows half-open probes after the open timeout, and closes after a successful probe.
- Dispatcher fallback is request-local and budgeted. A fallback is allowed only when the request body is replayable, no downstream output has started, the provider error is retryable, and the retry budget has remaining attempts/time.
- Provider attempt records include retryability, retry budget consumed/remaining, fallback source channel/provider, circuit state, and whether the attempt was final.

## Operational Checks

Run focused reliability tests before changing routing or provider adapters:

```bash
go test ./internal/dataplane/router ./internal/dataplane/dispatch ./internal/billing ./internal/infra/redis
```

Run the live drill against a local or staging gateway. Optional model variables should point at routes configured to return the named upstream behavior.

```bash
GATEWAY_URL=http://127.0.0.1:8080 \
API_KEY=tg-local-dev-key \
MODEL=gpt-4o-mini \
PROVIDER_429_MODEL=gpt-4o-mini-429 \
PROVIDER_5XX_MODEL=gpt-4o-mini-5xx \
PROVIDER_TIMEOUT_MODEL=gpt-4o-mini-timeout \
PROVIDER_BAD_JSON_MODEL=gpt-4o-mini-bad-json \
PROVIDER_STREAM_INTERRUPT_MODEL=gpt-4o-mini-stream-interrupt \
tests/failure/provider_reliability_drills.sh
```

Emergency disable can be included when the control API is available:

```bash
CONTROL_URL=http://127.0.0.1:9090 \
ADMIN_TOKEN=local-admin-token \
EMERGENCY_CHANNEL_ID=channel_local_openai \
tests/failure/provider_reliability_drills.sh
```

## Expected Evidence

- Retryable 429, 5xx, and timeout attempts either succeed through fallback or return their stable provider error class without trying non-replayable requests.
- 401/403 and schema/provider request errors do not fallback.
- Circuit state enters `open`, transitions to `half_open`, and closes after a successful probe in focused tests.
- `usage_attempts` and provider attempt logs contain retry budget, fallback source, circuit state, and final attempt fields.
