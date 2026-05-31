#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "drill=worker_lease_heartbeat"
(cd "${ROOT_DIR}" && go test ./internal/worker -run 'TestRunner(RenewsLeaseWhileJobRuns|RunStartsMaxConcurrencySlots)' -count=1)

echo "drill=provider_task_poller_error_isolation"
(cd "${ROOT_DIR}" && go test ./internal/worker/jobs -run TestProviderTaskPollerIsolatesSingleTaskPollError -count=1)

echo "drill=callback_durable_claim_and_delivery"
(cd "${ROOT_DIR}" && go test ./internal/task -run TestCallbackClaimAssignsOwnerAndReclaimsStaleRows -count=1)
(cd "${ROOT_DIR}" && go test ./internal/worker/jobs -run 'TestCallbackDispatcher(DeadLettersAtRetryCeiling|DrainsAndClosesResponseBody)' -count=1)

echo "worker_callback_drills=passed"
