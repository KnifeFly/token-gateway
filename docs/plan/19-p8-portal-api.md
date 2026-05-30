# P8 Portal 接口

## 阶段目标

P8 的目标是为开发者自助门户提供第一版 customer-facing API。Portal 只覆盖模型/Schema、credits、用量报表、API key 自助管理和任务查询，复用现有 API key 鉴权，不引入 RBAC、管理员审计平台、渠道配置或复杂财务能力。

## 交付物

- `/v1/portal/models` 和 `/v1/portal/models/{model}/schema`：返回当前 API key 可见模型和动态参数 schema。
- `/v1/portal/credits`：返回当前 tenant/project 的余额、已用额度和币种。
- `/v1/portal/usage`：返回当前 tenant/project 的用量、扣费流水摘要和分页时间范围。
- `/v1/portal/api-keys`：允许当前 tenant/project 内创建、列表和禁用派生 API key，不能扩大模型权限或额度。
- `/v1/portal/tasks` 和 `/v1/portal/tasks/{task_id}`：查询当前 tenant/project 下异步任务列表和详情。
- Portal OpenAPI、handler/service 规划、权限边界测试和 customer-facing 错误响应。

## 核心实现顺序

1. 在 OpenAPI 增加 Portal tag、paths、schema 和示例，明确所有 portal 接口使用 Bearer API key。
2. 设计 Portal service，复用 snapshot/auth/reporting/task/admin repository 的只读或受限写能力。
3. 实现模型、schema、credits 和 usage 查询，所有返回结果按当前 principal 的 tenant/project/api key scope 过滤。
4. 实现 API key 自助管理：首个 key 仍由 admin 创建，portal 创建的派生 key 只能继承当前 key 的 tenant/project 和模型权限子集。
5. 实现 task 列表和详情查询，禁止跨 tenant/project 访问，隐藏 provider secret、内部错误和敏感 metadata。
6. 增加 handler tests、权限边界测试、OpenAPI contract tests 和最小 portal smoke。

## 关键设计约束

- Portal 不是 admin/control API，不允许配置 provider channel、route、price、limit、plugin、snapshot 或 emergency action。
- Portal 第一版不引入 RBAC；权限来自当前 API key 的 tenant、project、allowed_models 和 enabled/revoked 状态。
- Portal 创建的 API key 不能扩大当前 key 权限，不能跨 project，不能查看 plaintext key 历史值。
- Usage/credits 只返回客户可理解的余额、用量和扣费摘要，不暴露内部 ledger 修复细节或 provider 成本。
- 本阶段不做开发者 UI，只做 API 合同和后端接口规划。

## 验收标准

- OpenAPI 中 `/v1/portal/*` paths、schema、security 和错误响应完整。
- Portal 模型和 schema 结果与 `/v1/models` 当前可见范围一致，但路径为 portal 专用。
- Portal API key 创建、列表、禁用有权限边界测试，不能跨 tenant/project 或扩大 allowed_models。
- Portal usage/credits/tasks 查询只返回当前 tenant/project 范围数据。
- 客户端错误统一返回 gateway 标准 ErrorResponse，不泄露 provider key、admin token、原始 prompt 或内部 SQL 错误。

## 风险与处理

| 风险 | 处理 |
|---|---|
| Portal 变成第二套 admin API | 明确只开放 customer self-service，不暴露配置面写接口 |
| API key 自助管理扩大权限 | 派生 key 权限必须是当前 key 权限子集 |
| Usage 报表暴露内部账务细节 | Portal response 使用客户视角 schema，不返回 provider cost 或 repair internals |
| 与已有 `/v1/models`、`/v1/credits` 重叠 | 保留现有公开 API，Portal 路径作为前端聚合和权限边界更清晰的版本 |

## 设计来源

- [路线图](./00-roadmap.md)
- [P7 非存储媒体转发生态](./18-p7-media-forwarding-providers.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单](../tasks.md)
