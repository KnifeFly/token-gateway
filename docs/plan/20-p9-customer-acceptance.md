# P9 客户接入验收收口

## 阶段目标

P9 的目标是在 P5-P8 产品能力完成后，补齐客户接入前的可重复验收资产。P9 不扩大产品面，不新增 RBAC、复杂发票、对象存储、完整 Realtime、动态插件或多地域能力，只把 Portal、OpenAPI 和 RC smoke 收敛为客户侧可以执行和复核的验收流程。

## 交付物

- 客户侧 Portal smoke CLI：使用 customer Bearer API key 验证 `/v1/portal/*` 最小闭环。
- RC smoke 集成：干净依赖环境中自动执行 Portal customer acceptance。
- OpenAPI import preflight：合同测试验证 YAML 可解析、本地 `$ref` 可解析、operationId 唯一和 Portal 鉴权继承。
- 客户接入验收 runbook：提供本地、staging 和 RC 验证命令。
- 任务看板和路线图同步：P9 作为 P5-P8 后的验收收口阶段。

## 核心实现顺序

1. 确认 P8 已提交且工作区干净，从 P8 提交切出 P9 分支。
2. 新增 `tools/portal-smoke`，只依赖 Go 标准库和客户 API key。
3. 将 Portal smoke 接入 `tests/rc/clean_env_smoke.sh`，复用 RC 环境的 seeded API key 和 snapshot。
4. 新增 OpenAPI import preflight 合同测试，避免导入工具才暴露结构性问题。
5. 更新 `docs/plan`、`docs/tasks.md`、runbook 和设计路线边界。

## 关键设计约束

- Portal smoke 不接受 admin token，不调用 control/configd 管理接口。
- 默认不创建派生 key；需要完整生命周期验收时显式传 `-create-derived-key`。
- 输出不得打印 API key、plaintext key、hash、provider credential 或内部 repair 信息。
- OpenAPI preflight 只验证当前合同的结构完整性，不替代 Apifox 或客户 SDK 的真实导入验收。
- P9 只做验收收口，不把先前声明不做或先不做的能力重新纳入路线。

## 验收标准

- `go test ./tools/portal-smoke ./tests/contract` 通过。
- `bash -n tests/rc/clean_env_smoke.sh` 通过。
- `go test ./...` 通过。
- 对运行中的 gateway 执行 `go run ./tools/portal-smoke -gateway-url <url> -api-key <key>` 能完成模型、schema、credits、usage、API key 列表和 task 查询。
- 在 RC smoke 中执行 `go run ./tools/portal-smoke ... -create-derived-key` 能完成派生 key 创建、列表不泄露 plaintext/hash、禁用派生 key 和 task 查询。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 验收脚本误用 admin token | CLI 只接收 customer API key，runbook 明确禁止 admin token |
| 烟测污染客户环境 | 派生 key 生命周期默认关闭，只在 RC/staging 中显式启用并立即 disable |
| OpenAPI 导入问题延后暴露 | `tests/contract` 中增加本地 `$ref`、operationId 和 Portal security 预检 |
| P9 被误解为新产品阶段 | 文档固定 P9 是验收收口，不扩大路线边界 |

## 执行记录

- 2026-05-30：新增 `tools/portal-smoke`、OpenAPI import preflight、RC Portal customer acceptance 步骤和客户验收 runbook。
- 2026-05-30：P9 保持 P5-P8 后的验收收口定位，不新增 Portal 以外的客户 API 面。
- 2026-05-30：`go test ./...`、`go test ./tools/portal-smoke ./tests/contract`、`bash -n tests/rc/clean_env_smoke.sh` 和 `git diff --check` 通过。

## 设计来源

- [P8 Portal API](./19-p8-portal-api.md)
- [发布候选验收](./15-p4-release-candidate-readiness.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单 P9](../tasks.md)
