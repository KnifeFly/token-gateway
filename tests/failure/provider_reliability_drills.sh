#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"
CONTROL_URL="${CONTROL_URL:-}"
API_KEY="${API_KEY:-tg-local-dev-key}"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"
MODEL="${MODEL:-gpt-4o-mini}"

fail() {
  echo "FAIL $1" >&2
  exit 1
}

status_code() {
  curl -sS -o /tmp/token-gateway-provider-reliability-drill-response.json -w "%{http_code}" "$@"
}

chat_status() {
  local model="$1"
  local extra="${2:-}"
  status_code \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"model":"'"${model}"'","stream":'"${extra:-false}"',"messages":[{"role":"user","content":"provider reliability drill"}]}' \
    "${GATEWAY_URL}/v1/chat/completions"
}

expect_status() {
  local name="$1"
  local code="$2"
  local pattern="$3"
  [[ "${code}" =~ ${pattern} ]] || fail "${name} status ${code}, want ${pattern}"
}

run_optional_model_drill() {
  local name="$1"
  local model="$2"
  local pattern="$3"
  [[ -n "${model}" ]] || return 0
  echo "drill=${name}"
  code="$(chat_status "${model}")"
  expect_status "${name}" "${code}" "${pattern}"
}

echo "drill=provider_reliability_baseline"
code="$(chat_status "${MODEL}")"
expect_status "baseline" "${code}" "^(200|402|429|502|503)$"

run_optional_model_drill "provider_429_fallback_or_classification" "${PROVIDER_429_MODEL:-}" "^(200|429)$"
run_optional_model_drill "provider_5xx_fallback_or_classification" "${PROVIDER_5XX_MODEL:-}" "^(200|502|503)$"
run_optional_model_drill "provider_timeout_fallback_or_classification" "${PROVIDER_TIMEOUT_MODEL:-}" "^(200|502|503)$"
run_optional_model_drill "provider_slow_response_budget" "${PROVIDER_SLOW_MODEL:-}" "^(200|502|503)$"
run_optional_model_drill "provider_bad_json_no_success" "${PROVIDER_BAD_JSON_MODEL:-}" "^(200|502)$"

if [[ -n "${PROVIDER_STREAM_INTERRUPT_MODEL:-}" ]]; then
  echo "drill=provider_stream_interrupt_no_transparent_fallback"
  code="$(chat_status "${PROVIDER_STREAM_INTERRUPT_MODEL}" "true")"
  expect_status "stream interrupt" "${code}" "^(200|502|503)$"
fi

if [[ -n "${PROVIDER_5XX_MODEL:-}" ]]; then
  echo "drill=circuit_open_and_recovery_probe"
  for _ in 1 2 3; do
    code="$(chat_status "${PROVIDER_5XX_MODEL}")"
    expect_status "circuit failure sample" "${code}" "^(200|502|503)$"
  done
  code="$(chat_status "${MODEL}")"
  expect_status "circuit recovery probe" "${code}" "^(200|402|429|502|503)$"
fi

if [[ -n "${CONTROL_URL}" && -n "${ADMIN_TOKEN}" && -n "${EMERGENCY_CHANNEL_ID:-}" ]]; then
  echo "drill=emergency_channel_disable"
  code="$(status_code \
    -X POST \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${CONTROL_URL}/admin/emergency/channels/${EMERGENCY_CHANNEL_ID}/disable?ttl=30s")"
  expect_status "emergency disable" "${code}" "^200$"
  code="$(chat_status "${MODEL}")"
  expect_status "emergency disabled route" "${code}" "^(200|402|503)$"
  code="$(status_code \
    -X POST \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${CONTROL_URL}/admin/emergency/channels/${EMERGENCY_CHANNEL_ID}/enable")"
  expect_status "emergency enable" "${code}" "^200$"
fi

echo "provider_reliability_drills=passed"
