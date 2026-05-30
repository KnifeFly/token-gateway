# M7 Observability Assets

- `prometheus-alerts.yaml`: alert rules for failed settlement backlog, snapshot publish errors, provider failures, provider rate limits, and callback retries.
- `grafana-dashboard.json`: starter dashboard covering HTTP, provider, billing, task/callback, snapshot, fallback, and degradation signals.

Metrics use the `token_gateway_` prefix and avoid request IDs, API keys,
prompts, responses, and other high-cardinality or sensitive values in labels.
