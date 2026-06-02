#!/usr/bin/env sh
set -eu

root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
cd "$root"

pnpm --filter @token-gateway/api-client generate
