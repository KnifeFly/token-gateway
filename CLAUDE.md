# token-gateway Agent Guide

This file is the persistent project context for Claude Code and other coding agents.
Keep it short, operational, and aligned with `docs/plan/`, `docs/tasks.md`, and the design sources in `docs/design/`.

## Project Intent

`token-gateway` is a commercial AI Gateway, not a thin OpenAI proxy.
It must support:

- Native-compatible APIs for OpenAI, Claude, Gemini, embeddings, images, and audio.
- Unified media and Agent APIs for image, video, audio, music, files, async tasks, callback, credits, and usage.
- Multi-tenant routing, provider fallback, API resale, token/credit billing, ledger, reconciliation, security governance, and observability.

The product target is: one stable customer-facing API surface, many upstream providers, auditable commercial accounting.

## Documentation Map

Before implementing architecture or domain behavior, use the docs in this order:

1. `docs/plan/00-roadmap.md` for the M0-M8 roadmap, release rhythm, and phase-level execution order.
2. The current phase doc in `docs/plan/01-m0-foundation.md` through `docs/plan/09-m8-commercial-ops.md` for executable scope, constraints, acceptance criteria, and risks.
3. `docs/tasks.md` for the task board, current execution entry point, status, dependencies, and handoff notes.
4. `docs/design/ai_gateway_design_pack_v2_README.md` for the design-pack index and non-negotiable principles.
5. `docs/design/ai_gateway_system_design_v2.md` for product scope, protocols, lifecycle, security, and acceptance criteria.
6. `docs/design/ai_gateway_architecture_design_v2.md` for planes, layering, package ownership, runtime snapshot, routing, billing, and stream rules.
7. `docs/design/ai_gateway_code_blueprint_v2.md` for package layout, interfaces, structs, and code-level architecture references.
8. `docs/design/ai_gateway_openapi_v2.yaml` for API contract updates.

`docs/plan/` and `docs/tasks.md` are the execution entry points. `docs/design/` remains the source of truth for system, architecture, code blueprint, and OpenAPI design. Do not copy long design or plan text into code; keep implementation comments focused on local behavior.

## Architecture Rules

- Public URI names describe capabilities, not vendors. Do not add vendor-shaped public routes unless compatibility requires it.
- `model` is the routing entry point, not a plain string. It resolves through public model, aliases, capability, schema, provider mapping, route policy, price rule, limit rule, and plugin binding.
- Data Plane must read runtime snapshot/indexes on the hot path. Do not query admin/config tables from request handling.
- Provider adapters only translate protocol, call upstream, map errors, parse usage, and submit/poll provider tasks. They must not own routing, policy, billing, or tenant decisions.
- Keep package ownership clear: `domain` has entities and invariants; `app` coordinates use cases; `infra` implements repositories and external services; `provider` handles upstream protocol; `transport` handles HTTP decode/write; `dataplane` is the hot path.
- `internal/bootstrap` wires dependencies only. Do not put business logic there.
- Use a single request state through the GatewayEngine hot path. The main flow should remain readable enough to act as system documentation.
- Plugins are configuration-driven built-ins first. Do not introduce dynamic code execution or plugin marketplaces in the MVP.

# Code Style

### Go Idioms

Follow [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments):

- **Error flow**: Early return, indent error handling—not else blocks
  ```go
  if err != nil {
      return err
  }

  // normal code
  ```
- **Naming**: initialisms (`URL`, `HTTP`, `ID`—not `Url`, `Http`, `Id`)
- **Receiver names**: 1–2 letters reflecting type (`c` for `Client`, `h` for `Handler`)
- **Error strings**: Lowercase, no trailing punctuation (`"something failed"` not `"Something failed."`)
- **Doc comments**: Full sentences starting with the name being documented
  ```go
  // Handler serves HTTP requests for the file server.
  type Handler struct { ... }
  ```
- **Empty slices**: `var t []string` (nil slice), not `t := []string{}` (non-nil zero-length)

## Commercial Invariants

- Provider success followed by local settlement failure must be repairable.
- Every billable request must produce usage records and ledger entries.
- Settlement must be idempotent and replayable; failed settlement must go to repair workflow.
- Balance hold, usage attempt, usage record, ledger entry, and final settlement must stay consistent.
- Stream accounting happens at stream close. If tokens have already been sent downstream, do not transparently retry another provider.
- If a client disconnects after receiving partial output, billability is decided by explicit policy and must be recorded.
- Multi-replica QPS, TPM, concurrency, and channel limits cannot rely on local memory as the source of truth.

## Security And Observability

- Never log API keys, provider keys, raw prompts, raw responses, credentials, or decrypted secret values.
- Do not place sensitive values in metrics labels or trace attributes.
- Logs must be structured and include request_id, trace_id, tenant_id, and project_id when available.
- Redact error messages before returning or logging them.
- Every request needs request_id and trace_id.
- Every provider attempt needs observable latency, status, error class, retry/fallback outcome, and cost/usage information when available.
- Snapshot version and staleness must be exposed through metrics; diagnostic response headers may be gated by config.

## Implementation Sequence

Prefer the staged path from `docs/plan/00-roadmap.md`:

1. M0: Go module, directories, config, error package, logger, HTTP server, healthz, readyz, metrics, OpenAPI, migrations, Makefile.
2. M1: minimal Data Plane for `/v1/chat/completions`, API key auth, model parse, runtime snapshot, priority/weighted routing, OpenAI-compatible relay, provider error mapping, basic metrics/tracing.
3. M2: billing closure with balance hold, usage attempt, settlement, ledger, failed settlement replay.
4. M3: OpenAI Responses, Claude Messages, Gemini GenerateContent, embeddings, and stream support.
5. M4: unified async media tasks, files, provider polling/webhook, callback.
6. M5: control plane configuration, runtime snapshot build/validate/publish/watch, rollback, and hot reload.
7. M6: plugin chain, security governance, PII redaction, PromptGuard, ResponseGuard, and audit.
8. M7: production metrics, tracing, dashboards, alerts, load tests, and failure drills.
9. M8: commercial operations, usage/cost/profit reports, reconciliation, billing export, and model marketplace support.

When a task is broad, narrow it to the next milestone unless the user explicitly asks for a later capability.

## Expected Layout

The intended Go layout is:

```text
cmd/gateway
cmd/control-api
cmd/worker
cmd/configd
configs
migrations
internal/bootstrap
internal/transport
internal/app
internal/domain
internal/dataplane
internal/controlplane
internal/provider
internal/billing
internal/task
internal/infra
internal/worker
pkg/apperr
pkg/money
pkg/redaction
pkg/tokenusage
```

Create only the directories and packages needed for the current milestone. Avoid empty package sprawl.

## Coding Defaults

- Use Go 1.26+ unless the repo pins a newer version.
- Prefer standard `net/http` plus a small router such as chi unless the repo has already chosen another framework.
- Prefer `slog` or an established repo logger for structured logs.
- Keep domain packages free of HTTP, SQL, Redis, provider SDKs, and metrics clients.
- Make interfaces small and owned by the package that consumes them.
- Keep external protocol compatibility at transport/adapter boundaries; do not let provider-specific request shapes leak through the core engine.
- Use explicit money/usage types. Avoid floats for money.
- Prefer deterministic clocks and ID generators in testable components.

## Commands

The repo is currently design-first. As implementation lands, keep these commands working:

```bash
go test ./...
make test
make lint
make build
make run-gateway
go run ./cmd/gateway -config configs/local.yaml
```

For M0 acceptance, the gateway should expose:

```text
GET /healthz
GET /readyz
GET /metrics
```

If a command is missing because the scaffold has not reached that stage yet, add the smallest useful target instead of inventing a parallel workflow.

## Definition Of Done

For non-trivial changes, verify the relevant slice before finishing:

- Unit tests for touched core packages.
- Integration or e2e coverage for request lifecycle, provider relay, stream close, billing, task, or snapshot behavior when affected.
- Error codes mapped to the external protocol.
- Metrics, trace spans, structured logs, audit events, and redaction updated when the request path changes.
- `docs/design/ai_gateway_openapi_v2.yaml` updated when public API behavior changes.
- `docs/plan/` or `docs/tasks.md` updated when phase scope, execution order, task status, or handoff state changes.
- `docs/design/` updated when system, architecture, blueprint, or API design truth changes; otherwise explain intentional implementation differences.
- `go test ./...`, `make test`, `make lint`, or the nearest available focused command run and reported.

## Review Checklist

Before accepting a change, ask:

- Can we answer who called, which model was used, why this provider/channel was chosen, and where the trace is?
- Can we explain estimated cost, actual billed amount, provider cost, and repair state after failure?
- Did the hot path avoid live admin-table lookups?
- Did any provider adapter accidentally take ownership of routing, policy, billing, or tenant decisions?
- Did any sensitive data enter logs, metrics, traces, audit, or errors?
- Did stream, retry, fallback, and settlement behavior stay explicit?

## Repo Etiquette

- Keep `CLAUDE.md` as the root source of agent guidance. `AGENTS.md` may remain a short pointer to this file.
- Do not overwrite user changes. Inspect `git status --short` before broad edits.
- Prefer focused diffs that follow the milestone currently being implemented.
- Use `docs/tasks.md` as the live task board and handoff entry point.
- Use `docs/plan/` for milestone execution changes and `docs/design/` for architecture/design truth.
- When adding new architecture, update the design docs or explain why the implementation intentionally differs.

## Commit Rules

- Use atomic commits grouped by logical intent.
- Use Conventional Commit format: `feat(scope)`, `fix(scope)`,
  `refactor(scope)`, `docs`, `test(scope)`, or `chore(scope)`.

## Handoff Rule

Before any context compaction, long-task handoff, pause, or stop, update
[`docs/tasks.md`](./docs/tasks.md).

The update must record completed work, in-progress work, blocked work, next
recommended steps, and verification results. If no progress occurred, record
"no status change" and the current verification state.
