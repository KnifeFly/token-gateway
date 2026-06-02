# Release Handoff Runbook

## Scope

P10 closes the PR and release handoff loop after P9 customer acceptance. It does not add new product capabilities. The handoff document records current git state, latest migration, validation commands, customer acceptance evidence, release fields, and rollback command.

## Generate Handoff

Print a handoff document without running checks:

```bash
make release-handoff
```

Write a handoff document with local verification evidence:

```bash
go run ./tools/release-handoff -run-checks -output /tmp/token-gateway-release-handoff.md
```

The `-run-checks` mode runs:

- `go test ./...`
- `go vet ./...`
- `go build ./cmd/...`
- `go test ./tools/portal-smoke ./tools/portal-web-smoke ./tools/p24-console-smoke ./tools/release-handoff ./tests/contract`
- `make p24-cut-scope-check`
- `bash -n tests/rc/clean_env_smoke.sh`
- `tests/failure/release_gate.sh`

Full RC/staging smoke is intentionally not run inside the handoff tool because it needs Docker or staging dependencies and disposable credentials.

## PR Checklist

Use `.github/pull_request_template.md` for every release PR. The PR should include:

- scope and public API/migration impact
- validation command results
- customer acceptance evidence
- release commit, image tag, migration version, snapshot version
- rollback and reconciliation status
- explicit confirmation that no out-of-scope capability was added

## Customer Acceptance Evidence

For local syntax and contract:

```bash
go test ./tools/portal-smoke ./tools/portal-web-smoke ./tools/p24-console-smoke ./tools/release-handoff ./tests/contract
make p24-cut-scope-check
bash -n tests/rc/clean_env_smoke.sh
```

For disposable RC or staging:

```bash
tests/rc/clean_env_smoke.sh
go run ./tools/p24-console-smoke \
  -console-url "${CONSOLE_URL}" \
  -api-key "${API_KEY}" \
  -admin-email "${ADMIN_EMAIL}" \
  -admin-password "${ADMIN_PASSWORD}" \
  -model "${MODEL}"
```

Required output markers:

- `rc_smoke=portal_customer_acceptance`
- `portal_smoke=passed`
- `p24_console_smoke=passed`
- `rc_smoke=passed`

## Release Evidence Fields

Fill these fields from the generated handoff and staging run:

| Field | Source |
|---|---|
| Release commit | `git rev-parse --short HEAD` or handoff output |
| Branch / PR | GitHub PR URL |
| Image tag | build pipeline |
| Migration latest | handoff output or `migrations/mysql` latest `*.up.sql` |
| Snapshot version | configd diagnostics |
| Redis key prefix | staging config |
| Release gate result | `release_gate=passed` |
| Portal smoke result | `portal_smoke=passed` |
| P24 console smoke result | `p24_console_smoke=passed` |
| P24 cut-scope result | `P24 cut-scope check passed` |
| Rollback tested | configd rollback result |
| Reconciliation result | reporting/reconciliation output |
| Known risks | owner-authored |

## Rollback

Prefer snapshot rollback before code or image rollback:

```bash
curl -fsS -X POST -H "X-Admin-Token: ${ADMIN_TOKEN}" \
  "${CONFIGD_URL}/configd/snapshots/rollback"
```

After rollback, verify:

- gateway responses show the previous `X-Gateway-Snapshot-Version`
- Redis active snapshot key matches configd diagnostics
- provider attempt error rate returns to baseline
- failed settlement backlog does not increase
- Portal smoke still passes for the disposable customer key
