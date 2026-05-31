# Commercial Boundaries Runbook

## Async Price Pin

Async task creation persists `price_snapshot_json` on `tasks`. The snapshot includes public model, currency, customer input/output micros per token, estimated output, estimated hold amount, route policy id, and runtime snapshot version.

Workers must settle async tasks from this pinned snapshot when it contains customer rates. A later control-plane price or route update affects only new tasks.

## Budget Semantics

Redis `daily_admission_budget_micros` and `cost_per_minute_micros` are admission guards. They use the estimated request charge to prevent obvious overspend bursts before provider dispatch, but they are not final spend ledgers.

`daily_budget_micros` is still accepted as a compatibility alias. Prefer `daily_admission_budget_micros` for new control-plane and local config.

Final commercial truth is:

- `usage_records` for normalized actual usage.
- `ledger_entries` for charged amount.
- failed settlement repair records when local settlement cannot complete.
- reconciliation reports for differences between balance, ledger, and usage.

When explaining a budget incident, separate estimated admission decisions from actual settlement and reconciliation output.

## Egress Guard

Outbound URLs for file source URLs, callback URLs, and provider base URLs are checked by `egressguard`. Runtime clients also use a guarded dialer so a DNS answer that changes to a blocked IP is rejected at connect time.

Default blocked targets include loopback, private, link-local, multicast, reserved/documentation ranges, cloud metadata endpoints, and non-HTTP schemes. Enterprise private integrations require explicit deployment allowlists through `gateway.egress.allowed_hosts` and `gateway.egress.allowed_cidrs`.

## Callback Delivery

Callback requests include:

- `X-Gateway-Task-ID`
- `X-Gateway-Callback-ID`
- `X-Gateway-Callback-Timestamp`
- `X-Gateway-Callback-Signature`

The signature is `sha256=<hex_hmac_sha256(secret, timestamp + "." + payload)>`. Configure the secret with `worker.callback_signing_secret` or `TOKEN_GATEWAY_CALLBACK_SIGNING_SECRET`; replace the local default before production use.

Callbacks retry with bounded backoff. After `worker.callback_max_retries`, the outbox row moves to `dead_letter` and is no longer selected by the dispatcher.
