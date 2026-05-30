# M7 Failure Drills

Run against a local or staging gateway:

```bash
GATEWAY_URL=http://127.0.0.1:8080 API_KEY=tg-local-dev-key tests/failure/drills.sh
```

The script checks health, auth rejection, provider/route behavior, and the M7
metrics contract. Redis, DB, provider, settlement, callback, snapshot, and
worker restart drills should be executed by changing the target environment and
then re-running the same script plus the load test.

P6 provider reliability drills use configured mock or staging routes for
429/5xx/timeout/slow/bad JSON/stream interruption scenarios:

```bash
GATEWAY_URL=http://127.0.0.1:8080 \
API_KEY=tg-local-dev-key \
PROVIDER_429_MODEL=gpt-4o-mini-429 \
PROVIDER_5XX_MODEL=gpt-4o-mini-5xx \
PROVIDER_TIMEOUT_MODEL=gpt-4o-mini-timeout \
tests/failure/provider_reliability_drills.sh
```

Set `CONTROL_URL`, `ADMIN_TOKEN`, and `EMERGENCY_CHANNEL_ID` to include the
hot emergency disable drill.
