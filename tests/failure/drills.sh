#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"
API_KEY="${API_KEY:-tg-local-dev-key}"
MODEL="${MODEL:-gpt-4o-mini}"

fail() {
  echo "FAIL $1" >&2
  exit 1
}

status_code() {
  curl -sS -o /tmp/token-gateway-drill-response.json -w "%{http_code}" "$@"
}

echo "drill=healthz"
code="$(status_code "${GATEWAY_URL}/healthz")"
[[ "${code}" == "200" ]] || fail "healthz status ${code}"

echo "drill=auth_rejection"
code="$(status_code \
  -H "Content-Type: application/json" \
  -d '{"model":"'"${MODEL}"'","messages":[{"role":"user","content":"ping"}]}' \
  "${GATEWAY_URL}/v1/chat/completions")"
[[ "${code}" == "401" ]] || fail "auth rejection status ${code}"

echo "drill=provider_or_route_path"
code="$(status_code \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"model":"'"${MODEL}"'","messages":[{"role":"user","content":"ping"}]}' \
  "${GATEWAY_URL}/v1/chat/completions")"
[[ "${code}" =~ ^(200|402|429|502|503)$ ]] || fail "provider path status ${code}"

echo "drill=metrics_contract"
metrics="$(curl -sS "${GATEWAY_URL}/metrics")"
grep -q "token_gateway_http_requests_total" <<<"${metrics}" || fail "missing http request metric"
grep -q "token_gateway_provider_attempts_total" <<<"${metrics}" || fail "missing provider attempt metric"
grep -q "token_gateway_snapshot_staleness_seconds" <<<"${metrics}" || fail "missing snapshot staleness metric"

echo "drills=passed"
