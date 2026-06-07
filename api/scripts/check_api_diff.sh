#!/usr/bin/env sh
set -eu

root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
cd "$root"

api/scripts/lint_openapi.sh
api/scripts/generate_ts_client.sh
git diff --exit-code -- web/packages/api-client/src/generated
go test ./tests/contract
