#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LIVE="${TOKEN_GATEWAY_RELEASE_GATE_LIVE:-false}"
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:9501}"
API_KEY="${API_KEY:-tg-local-dev-key}"
MODEL="${MODEL:-gpt-4o-mini}"
REDIS_ADDR="${TOKEN_GATEWAY_RELEASE_GATE_REDIS_ADDR:-}"

fail() {
  echo "FAIL $*" >&2
  exit 1
}

require_file_contains() {
  local file="$1"
  local pattern="$2"
  rg -q "${pattern}" "${ROOT_DIR}/${file}" || fail "${file} missing ${pattern}"
}

echo "release_gate=observability_contract"
require_file_contains deployments/observability/prometheus-alerts.yaml "TokenGatewayFailedSettlementBacklog"
require_file_contains deployments/observability/prometheus-alerts.yaml "TokenGatewaySnapshotStale"
require_file_contains deployments/observability/prometheus-alerts.yaml "TokenGatewayProvider5xx"
require_file_contains deployments/observability/prometheus-alerts.yaml "TokenGatewayWorkerJobFailures"
require_file_contains deployments/observability/prometheus-alerts.yaml "token_gateway_worker_job_runs_total"
require_file_contains deployments/observability/grafana-dashboard.json "Provider Attempts"
require_file_contains deployments/observability/grafana-dashboard.json "Billing Repair Backlog"
require_file_contains deployments/observability/grafana-dashboard.json "Worker Job Outcomes"
require_file_contains deployments/observability/grafana-dashboard.json "Snapshot Staleness"
require_file_contains tools/loadtest/main.go "redis_latency_p95_ms"

echo "release_gate=json_assets"
node -e 'JSON.parse(require("fs").readFileSync("deployments/observability/grafana-dashboard.json", "utf8"))' >/dev/null

echo "release_gate=secret_scan"
if rg -n "(sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9-]{20,}|ghp_[A-Za-z0-9]{36})" \
  "${ROOT_DIR}/configs" "${ROOT_DIR}/deployments" "${ROOT_DIR}/docs" "${ROOT_DIR}/tests"; then
  fail "potential secret found in durable repo assets"
fi

echo "release_gate=focused_tests"
(cd "${ROOT_DIR}" && go test ./pkg/redaction ./internal/infra/telemetry ./internal/dataplane/observe ./internal/worker)

if [[ "${REDIS_ADDR}" != "" ]]; then
  echo "release_gate=redis_latency"
  output="$(cd "${ROOT_DIR}" && go run ./tools/loadtest -requests 1 -concurrency 1 -redis "${REDIS_ADDR}")"
  grep -q "redis_latency_p95_ms" <<<"${output}" || fail "missing redis latency output"
fi

if [[ "${LIVE}" == "true" ]]; then
  echo "release_gate=live_failure_drills"
  (cd "${ROOT_DIR}" && GATEWAY_URL="${GATEWAY_URL}" API_KEY="${API_KEY}" MODEL="${MODEL}" tests/failure/drills.sh)

  echo "release_gate=live_loadtest"
  output="$(cd "${ROOT_DIR}" && go run ./tools/loadtest -url "${GATEWAY_URL}/v1/chat/completions" -api-key "${API_KEY}" -model "${MODEL}" -requests 10 -concurrency 2)"
  grep -q "latency_p95_ms" <<<"${output}" || fail "missing loadtest latency output"
fi

echo "release_gate=passed"
