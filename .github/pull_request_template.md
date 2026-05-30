## Scope

- Phase:
- Summary:
- Public API changes:
- Migration changes:

## Validation

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `go build ./cmd/...`
- [ ] `go test ./tools/portal-smoke ./tools/release-handoff ./tests/contract`
- [ ] `bash -n tests/rc/clean_env_smoke.sh`
- [ ] `tests/failure/release_gate.sh`
- [ ] `go run ./tools/release-handoff -run-checks`

## Customer Acceptance

- [ ] `tests/rc/clean_env_smoke.sh` was run in disposable RC/staging, or intentionally deferred
- [ ] `rc_smoke=portal_customer_acceptance`
- [ ] `portal_smoke=passed`
- [ ] OpenAPI import preflight passed

## Release Notes

- Release commit:
- Image tag:
- Latest migration:
- Snapshot version:
- Rollback tested:
- Reconciliation result:
- Known risks:

## Boundaries

- [ ] No provider/API keys, raw prompts, raw responses, credentials, or plaintext derived keys are included
- [ ] No new RBAC, invoice, object storage, full Realtime, dynamic plugin, semantic cache, or multi-region scope was added unless explicitly planned
