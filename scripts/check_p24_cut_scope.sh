#!/usr/bin/env sh
set -eu

fail() {
  echo "P24 cut-scope check failed: $*" >&2
  exit 1
}

check_no_match() {
  name="$1"
  pattern="$2"
  shift 2
  tmp="/tmp/token-gateway-p24-cut-scope-$$.txt"
  if rg -n "$pattern" "$@" >"$tmp"; then
    cat "$tmp" >&2
    rm -f "$tmp"
    fail "$name"
  fi
  rm -f "$tmp"
}

openapi_files="api/openapi/admin-bff.yaml api/openapi/portal-bff.yaml"
generated_files="web/packages/api-client/src/generated/admin-bff.ts web/packages/api-client/src/generated/portal-bff.ts"
frontend_roots="web/apps/admin/src web/apps/portal/src"

check_no_match \
  "BFF OpenAPI must not expose NewAPI group/ratio/payment/subscription/redemption/invite/deployment routes" \
  '^  /api/(admin|portal)/v1/[^:]*(/|[-_])(user[-_]?groups?|model[-_]?groups?|channel[-_]?groups?|groups?|ratios?|payments?|subscriptions?|redemptions?|invite[-_]?rewards?|invites?|deployments?|model[-_]?deployments?|system[-_]?settings|settings/(system|global|payment|billing))([^A-Za-z0-9_-]|:|$)' \
  $openapi_files

check_no_match \
  "BFF contracts and generated clients must not expose cut-scope field names" \
  '(^|[^A-Za-z0-9_])(user_group|model_group|channel_group|group_ratio|model_ratio|channel_ratio|[A-Za-z0-9_]+_ratio|ratio_[A-Za-z0-9_]+|payment_config|payment_provider|subscription(_[A-Za-z0-9_]+)?|redemption(_[A-Za-z0-9_]+)?|invite_reward|deployment(_[A-Za-z0-9_]+)?)([^A-Za-z0-9_]|$)' \
  $openapi_files $generated_files

check_no_match \
  "frontend route configs must not expose cut-scope routes" \
  '(route|routePrefix|href):[[:space:]]*"[^"]*(user[-_]?groups?|model[-_]?groups?|channel[-_]?groups?|groups?|ratios?|payments?|subscriptions?|redemptions?|invite[-_]?rewards?|invites?|deployments?|model[-_]?deployments?|system[-_]?settings|settings/(system|global|payment|billing))([^A-Za-z0-9_-]|$)' \
  $frontend_roots

check_no_match \
  "frontend API calls must not call cut-scope BFF routes" \
  '"/api/(admin|portal)/v1/[^"]*(user[-_]?groups?|model[-_]?groups?|channel[-_]?groups?|groups?|ratios?|payments?|subscriptions?|redemptions?|invite[-_]?rewards?|invites?|deployments?|model[-_]?deployments?|system[-_]?settings|settings/(system|global|payment|billing))([^A-Za-z0-9_-]|$)' \
  $frontend_roots

if rg --files $frontend_roots | rg '(^|/)(user[-_]?groups?|model[-_]?groups?|channel[-_]?groups?|groups?|ratios?|payments?|subscriptions?|redemptions?|invite[-_]?rewards?|invites?|deployments?|model[-_]?deployments?|system[-_]?settings)(/|\.|$)' >/tmp/token-gateway-p24-cut-scope-files-$$.txt; then
  cat /tmp/token-gateway-p24-cut-scope-files-$$.txt >&2
  rm -f /tmp/token-gateway-p24-cut-scope-files-$$.txt
  fail "frontend feature files must not be created for cut-scope modules"
fi
rm -f /tmp/token-gateway-p24-cut-scope-files-$$.txt

echo "P24 cut-scope check passed"
