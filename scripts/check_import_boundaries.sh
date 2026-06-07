#!/usr/bin/env sh
set -eu

fail() {
  echo "import boundary check failed: $*" >&2
  exit 1
}

if rg -n '"github.com/KnifeFly/token-gateway/internal/portal"|internal/portal' cmd internal tests tools >/tmp/token-gateway-boundary-internal-portal.txt; then
  cat /tmp/token-gateway-boundary-internal-portal.txt >&2
  fail "runtime code must not import or reference internal/portal"
fi

if rg -n '"github.com/KnifeFly/token-gateway/internal/controlplane/admin"|internal/controlplane/admin' cmd internal tests tools >/tmp/token-gateway-boundary-control-admin.txt; then
  cat /tmp/token-gateway-boundary-control-admin.txt >&2
  fail "runtime code must use internal/controlplane/configadmin"
fi

if rg -n 'internal/app/admin' internal/controlplane/configadmin >/tmp/token-gateway-boundary-configadmin-app.txt; then
  cat /tmp/token-gateway-boundary-configadmin-app.txt >&2
  fail "configadmin must not depend on browser Admin app code"
fi

if rg -n 'internal/app/admin' internal/app/portal >/tmp/token-gateway-boundary-portal-admin.txt; then
  cat /tmp/token-gateway-boundary-portal-admin.txt >&2
  fail "Portal app must not depend on Admin app"
fi

if rg -n 'internal/app/portal' internal/app/admin >/tmp/token-gateway-boundary-admin-portal.txt; then
  cat /tmp/token-gateway-boundary-admin-portal.txt >&2
  fail "Admin app must not depend on Portal app"
fi

if rg -n '"/admin/|'\''/admin/' web/apps/admin/src >/tmp/token-gateway-boundary-admin-ui.txt; then
  cat /tmp/token-gateway-boundary-admin-ui.txt >&2
  fail "Admin browser app must use /api/admin/v1/*, not machine /admin/*"
fi

if rg -n 'web/apps|\\.\\./\\.\\./apps|\\.\\./apps' web/packages >/tmp/token-gateway-boundary-web-packages.txt; then
  cat /tmp/token-gateway-boundary-web-packages.txt >&2
  fail "shared web packages must not depend on web apps"
fi

echo "import boundary check passed"
