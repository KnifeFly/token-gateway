#!/usr/bin/env sh
set -eu

root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
cd "$root"

for spec in \
  api/openapi/gateway-public.yaml \
  api/openapi/portal-public.yaml \
  api/openapi/portal-bff.yaml \
  api/openapi/admin-bff.yaml \
  api/openapi/control.yaml
do
  test -f "$spec" || {
    echo "missing OpenAPI spec: $spec" >&2
    exit 1
  }
  pnpm exec openapi-typescript "$spec" -o /tmp/token-gateway-openapi-lint.d.ts >/dev/null
done

echo "openapi lint passed"
