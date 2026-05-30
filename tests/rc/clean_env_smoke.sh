#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADMIN_TOKEN="${TOKEN_GATEWAY_RC_ADMIN_TOKEN:-local-admin-token}"
API_KEY="${TOKEN_GATEWAY_RC_API_KEY:-tg-rc-key}"
PROJECT_NAME="${TOKEN_GATEWAY_RC_PROJECT:-token_gateway_rc_$(date +%s)}"
KEEP="${TOKEN_GATEWAY_RC_KEEP:-false}"

TEMP_DIR="$(mktemp -d)"
LOG_DIR="${TEMP_DIR}/logs"
mkdir -p "${LOG_DIR}"
PIDS=()

fail() {
  echo "FAIL $*" >&2
  exit 1
}

port_available() {
  ! lsof -ti "tcp:$1" >/dev/null 2>&1
}

choose_port() {
  local env_name="$1"
  local base="$2"
  local explicit="${!env_name:-}"
  if [[ -n "${explicit}" ]]; then
    echo "${explicit}"
    return
  fi
  local offset
  for offset in $(seq 0 200); do
    local candidate=$((base + offset))
    if port_available "${candidate}"; then
      echo "${candidate}"
      return
    fi
  done
  fail "no free port found near ${base}"
}

MYSQL_PORT="$(choose_port TOKEN_GATEWAY_RC_MYSQL_PORT 13306)"
REDIS_PORT="$(choose_port TOKEN_GATEWAY_RC_REDIS_PORT 16379)"
GATEWAY_PORT="$(choose_port TOKEN_GATEWAY_RC_GATEWAY_PORT 19501)"
CONTROL_PORT="$(choose_port TOKEN_GATEWAY_RC_CONTROL_PORT 19502)"
WORKER_PORT="$(choose_port TOKEN_GATEWAY_RC_WORKER_PORT 19503)"
CONFIGD_PORT="$(choose_port TOKEN_GATEWAY_RC_CONFIGD_PORT 19504)"

resolve_docker() {
  if [[ -n "${DOCKER_BIN:-}" && -x "${DOCKER_BIN}" ]]; then
    echo "${DOCKER_BIN}"
    return
  fi
  if command -v docker >/dev/null 2>&1 && docker version >/dev/null 2>&1; then
    command -v docker
    return
  fi
  local docker_desktop="/Applications/Docker.app/Contents/Resources/bin/docker"
  if [[ -x "${docker_desktop}" ]] && "${docker_desktop}" version >/dev/null 2>&1; then
    echo "${docker_desktop}"
    return
  fi
  fail "docker CLI is unavailable; set DOCKER_BIN to a working docker binary"
}

DOCKER_CMD="$(resolve_docker)"

docker_compose() {
  "${DOCKER_CMD}" compose -p "${PROJECT_NAME}" -f "${TEMP_DIR}/compose.yaml" "$@"
}

cleanup() {
  local status=$?
  for pid in "${PIDS[@]:-}"; do
    if kill -0 "${pid}" >/dev/null 2>&1; then
      kill "${pid}" >/dev/null 2>&1 || true
    fi
  done
  for pid in "${PIDS[@]:-}"; do
    wait "${pid}" >/dev/null 2>&1 || true
  done
  if [[ -f "${TEMP_DIR}/compose.yaml" ]]; then
    docker_compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  if [[ "${KEEP}" == "true" ]]; then
    echo "kept temp dir: ${TEMP_DIR}" >&2
  else
    rm -rf "${TEMP_DIR}"
  fi
  exit "${status}"
}
trap cleanup EXIT

cat >"${TEMP_DIR}/compose.yaml" <<YAML
services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_DATABASE: token_gateway
      MYSQL_USER: token_gateway
      MYSQL_PASSWORD: token_gateway
      MYSQL_ROOT_PASSWORD: token_gateway_root
    ports:
      - "${MYSQL_PORT}:3306"
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "127.0.0.1", "-utoken_gateway", "-ptoken_gateway"]
      interval: 3s
      timeout: 3s
      retries: 40
  redis:
    image: redis:7-alpine
    ports:
      - "${REDIS_PORT}:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 3s
      timeout: 3s
      retries: 40
YAML

cat >"${TEMP_DIR}/config.yaml" <<YAML
environment: rc
service:
  name: token-gateway
  version: rc-smoke
http:
  addr: "127.0.0.1:${GATEWAY_PORT}"
  read_header_timeout: 5s
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s
  shutdown_timeout: 5s
  max_header_bytes: 1048576
database:
  enabled: true
  driver: mysql
  dsn: "token_gateway:token_gateway@tcp(127.0.0.1:${MYSQL_PORT})/token_gateway?parseTime=true&multiStatements=true"
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 30m
  migrations_dir: migrations/mysql
redis:
  enabled: true
  addr: "127.0.0.1:${REDIS_PORT}"
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
telemetry:
  log_level: warn
  log_format: text
  metrics_enabled: true
  tracing:
    enabled: false
    exporter: noop
gateway:
  body:
    max_bytes: 4194304
  protocol:
    default_mode: auto
  idempotency:
    ttl: 24h
  seed_snapshot:
    enabled: true
    api_key: "${API_KEY}"
    api_key_id: "key_rc"
    tenant_id: "tenant_rc"
    project_id: "project_rc"
    model: "gpt-4o-mini"
    upstream_model: "gpt-4o-mini"
    provider_type: "openai_compatible"
    channel_id: "channel_rc_openai"
    provider_base_url: "mock://openai"
    route_strategy: "priority"
    route_priority: 1
    route_weight: 100
    channel_timeout: 30s
  billing:
    enabled: true
    currency: USD
    input_micros_per_token: 1
    output_micros_per_token: 2
    estimated_output_tokens: 256
    hold_ttl: 10m
    local_seed_balance_micros: 1000000000
  limits:
    enabled: true
    rpm: 3600
    qps: 60
    tpm: 60000
    concurrency: 100
    window: 1s
    lease_ttl: 30s
    deny_cache_ttl: 1s
    key_prefix: "${PROJECT_NAME}"
control:
  addr: "127.0.0.1:${CONTROL_PORT}"
  admin_token: "${ADMIN_TOKEN}"
  credential_key: "local-control-plane-credential-key"
  snapshot_poll_interval: 1s
  revocation_ttl: 24h
worker:
  enabled: true
  addr: "127.0.0.1:${WORKER_PORT}"
  shutdown_timeout: 5s
  lease_ttl: 10s
  job_timeout: 10s
  provider_task_poll_interval: 2s
  failed_settlement_interval: 5s
  hold_reaper_interval: 5s
  reconciliation_interval: 30s
  callback_interval: 2s
  batch_size: 20
configd:
  addr: "127.0.0.1:${CONFIGD_PORT}"
  shutdown_timeout: 5s
  publish_on_start: false
YAML

wait_for() {
  local name="$1"
  shift
  local deadline=$((SECONDS + 90))
  until "$@" >/dev/null 2>&1; do
    if (( SECONDS > deadline )); then
      fail "timeout waiting for ${name}"
    fi
    sleep 2
  done
}

wait_http() {
  local name="$1"
  local url="$2"
  wait_for "${name}" curl -fsS "${url}"
}

admin_post() {
  local path="$1"
  local payload="$2"
  curl -fsS -X POST \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "${payload}" \
    "http://127.0.0.1:${CONTROL_PORT}${path}" >/dev/null
}

start_process() {
  local name="$1"
  shift
  (cd "${ROOT_DIR}" && "$@" -config "${TEMP_DIR}/config.yaml" >"${LOG_DIR}/${name}.log" 2>&1) &
  PIDS+=("$!")
}

echo "rc_smoke=dependencies_up project=${PROJECT_NAME}"
docker_compose up -d mysql redis >/dev/null

echo "rc_smoke=migrate"
wait_for "mysql migration" bash -c "cd '${ROOT_DIR}' && go run ./cmd/migrate -config '${TEMP_DIR}/config.yaml' -direction up"

echo "rc_smoke=start_processes"
start_process gateway go run ./cmd/gateway
start_process control-api go run ./cmd/control-api
start_process configd go run ./cmd/configd
start_process worker go run ./cmd/worker

wait_http "gateway ready" "http://127.0.0.1:${GATEWAY_PORT}/readyz"
wait_http "control health" "http://127.0.0.1:${CONTROL_PORT}/healthz"
wait_http "configd ready" "http://127.0.0.1:${CONFIGD_PORT}/readyz"
wait_http "worker ready" "http://127.0.0.1:${WORKER_PORT}/readyz"

echo "rc_smoke=seed_control_plane"
admin_post "/admin/tenants" '{"id":"tenant_rc","name":"RC Tenant","enabled":true}'
admin_post "/admin/projects" '{"id":"project_rc","tenant_id":"tenant_rc","name":"RC Project","enabled":true}'
admin_post "/admin/api-keys" '{"id":"key_rc","tenant_id":"tenant_rc","project_id":"project_rc","name":"RC Key","plaintext_key":"'"${API_KEY}"'","allowed_models":["gpt-4o-mini"]}'
admin_post "/admin/models" '{"public_model":"gpt-4o-mini","protocol":"native_openai","capability":"chat","enabled":true}'
admin_post "/admin/channels" '{"id":"channel_rc_openai","provider_type":"openai_compatible","base_url":"mock://openai","enabled":true,"models":[{"public_model":"gpt-4o-mini","upstream_model":"gpt-4o-mini"}]}'
admin_post "/admin/routes" '{"public_model":"gpt-4o-mini","strategy":"priority","enabled":true,"candidates":[{"channel_id":"channel_rc_openai","priority":1,"weight":100}]}'
admin_post "/admin/prices" '{"public_model":"gpt-4o-mini","currency":"USD","input_micros_per_token":1,"output_micros_per_token":2,"estimated_output_tokens":256,"enabled":true}'
admin_post "/admin/limits" '{"tenant_id":"tenant_rc","project_id":"project_rc","api_key_id":"key_rc","public_model":"gpt-4o-mini","provider_type":"openai_compatible","channel_id":"channel_rc_openai","rpm":3600,"qps":60,"tpm":60000,"concurrency":100,"daily_budget_micros":1000000000,"cost_per_minute_micros":1000000,"enabled":true}'

echo "rc_smoke=configd_publish"
curl -fsS -X POST -H "X-Admin-Token: ${ADMIN_TOKEN}" "http://127.0.0.1:${CONFIGD_PORT}/configd/snapshots/publish" >/dev/null
curl -fsS -X POST -H "X-Admin-Token: ${ADMIN_TOKEN}" "http://127.0.0.1:${CONFIGD_PORT}/configd/snapshots/publish" >/dev/null
curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "http://127.0.0.1:${CONFIGD_PORT}/configd/snapshots/diagnostics" | grep -q '"active"'
docker_compose exec -T redis redis-cli GET "${PROJECT_NAME}:snapshot:active" | grep -q '"version":"snap-'

echo "rc_smoke=gateway_snapshot_watch"
wait_for "gateway active snapshot" bash -c "headers=\$(mktemp); body=\$(mktemp); code=\$(curl -sS -D \"\$headers\" -o \"\$body\" -w '%{http_code}' -H 'Authorization: Bearer ${API_KEY}' -H 'Content-Type: application/json' -d '{\"model\":\"gpt-4o-mini\",\"messages\":[{\"role\":\"user\",\"content\":\"rc smoke\"}]}' 'http://127.0.0.1:${GATEWAY_PORT}/v1/chat/completions'); grep -qi '^X-Gateway-Snapshot-Version: snap-' \"\$headers\" && grep -q 'chat.completion' \"\$body\" && test \"\$code\" = 200"

echo "rc_smoke=portal_customer_acceptance"
(cd "${ROOT_DIR}" && go run ./tools/portal-smoke -gateway-url "http://127.0.0.1:${GATEWAY_PORT}" -api-key "${API_KEY}" -model "gpt-4o-mini" -create-derived-key)

echo "rc_smoke=configd_rollback"
curl -fsS -X POST -H "X-Admin-Token: ${ADMIN_TOKEN}" "http://127.0.0.1:${CONFIGD_PORT}/configd/snapshots/rollback" >/dev/null
wait_for "gateway rollback snapshot" bash -c "curl -fsS -H 'Authorization: Bearer ${API_KEY}' -H 'Content-Type: application/json' -d '{\"model\":\"gpt-4o-mini\",\"messages\":[{\"role\":\"user\",\"content\":\"rc rollback\"}]}' 'http://127.0.0.1:${GATEWAY_PORT}/v1/chat/completions' | grep -q 'chat.completion'"

echo "rc_smoke=metrics"
curl -fsS "http://127.0.0.1:${GATEWAY_PORT}/metrics" | grep -q "token_gateway_http_requests_total"
curl -fsS "http://127.0.0.1:${WORKER_PORT}/metrics" | grep -q "token_gateway_worker_job_runs_total"

echo "rc_smoke=passed project=${PROJECT_NAME}"
