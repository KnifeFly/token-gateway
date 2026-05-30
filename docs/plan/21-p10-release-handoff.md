# P10 发布交接收口

## 阶段目标

P10 的目标是在 P9 客户接入验收之后，把当前代码分支收敛到可发 PR、可做 staging release、可回滚和可审计的交接状态。P10 不新增产品能力，不改变 public API，不引入控制面 RBAC、复杂发票、对象存储、完整 Realtime、动态插件、semantic cache 或多地域能力。

## 交付物

- Release handoff CLI：生成当前分支、commit、migration、验证命令、客户验收和回滚字段的 Markdown 交接文档。
- Release handoff check：可选执行本地发布检查并把结果写入交接文档。
- GitHub PR 模板：固定 scope、validation、customer acceptance、release notes 和边界确认。
- Release handoff runbook：说明如何生成证据、填 PR、跑 staging/RC 和处理回滚。
- 任务看板和设计边界同步：P10 作为 P9 之后的发布/PR 收口阶段。

## 核心实现顺序

1. 从 P9 提交切出 P10 分支，确认工作区干净。
2. 新增 `tools/release-handoff`，默认只生成交接文档，`-run-checks` 时执行本地验证集合。
3. 在 Makefile 增加 `release-handoff` 和 `release-handoff-check` 入口。
4. 新增 `.github/pull_request_template.md`，把 P5-P10 验证、客户验收、发布字段和范围边界固定下来。
5. 更新 release handoff runbook、staging rollout runbook、roadmap、tasks 和设计边界。

## 关键设计约束

- Handoff 工具不得输出 API key、provider key、raw prompt、raw response、password、secret 或 plaintext derived key。
- `-run-checks` 只跑本地可重复命令；完整 RC smoke、真实 provider 和 staging 验收仍由 runbook 显式执行。
- 交接文档是发布证据模板，不替代 code review、CI、staging 验收或 owner 批准。
- P10 只做 release/PR 收口，不扩大 P5-P9 已定义产品范围。

## 验收标准

- `go test ./tools/release-handoff` 通过。
- `go run ./tools/release-handoff` 能生成 Markdown 交接文档。
- `go run ./tools/release-handoff -run-checks` 能执行本地 release verification 并输出结果。
- PR 模板包含 validation、customer acceptance、release notes 和范围边界。
- `go test ./...`、`go vet ./...`、`go build ./cmd/...`、`bash -n tests/rc/clean_env_smoke.sh` 和 `git diff --check` 通过。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 发布交接只靠人工口头同步 | 固化 `tools/release-handoff` 和 PR 模板 |
| 本地检查误替代 staging | runbook 明确完整 RC/staging smoke 仍需单独执行 |
| 交接输出泄露密钥 | 工具对常见 token、API key、password、secret 输出做 redaction |
| P10 演变成新产品阶段 | 计划和 ADR 明确 P10 只做 release/PR 收口 |

## 执行记录

- 2026-05-30：新增 `tools/release-handoff`、Makefile 入口、PR 模板和 release handoff runbook。
- 2026-05-30：P10 定位为 P9 后发布交接收口，不新增 public API 或产品能力。
- 2026-05-30：`go run ./tools/release-handoff -run-checks -output /tmp/token-gateway-p10-release-handoff.md` 通过，覆盖 `go test ./...`、`go vet ./...`、`go build ./cmd/...`、Portal/OpenAPI contract、RC smoke 语法和 release gate。

## 设计来源

- [P9 客户接入验收收口](./20-p9-customer-acceptance.md)
- [发布候选验收](./15-p4-release-candidate-readiness.md)
- [Staging Rollout Runbook](../runbook/staging-rollout.md)
- [任务清单 P10](../tasks.md)
