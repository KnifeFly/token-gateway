# P24 NewAPI Lean Console Parity

## 阶段目标

P24 的目标是在 P19-P23 已建立的 Portal/Admin Console 基础上，补齐对标 NewAPI 后仍有价值的精简产品能力：渠道管理、模型管理、用户/客户账户管理、令牌管理、使用日志、任务日志、操练场和最小额度运营。

P24 不是 NewAPI clone。用户分组、模型分组、渠道分组、倍率、订阅套餐、兑换码、第三方支付配置、邀请返利、模型部署服务和大而全系统设置不进入本阶段。需要表达范围和运营策略时，使用 token-gateway 已有的 tenant/project/API key/model ACL/category/tag/route policy/price component/snapshot 体系。

## 当前状态

- P19 已建立 `cmd/console`、OpenAPI split、frontend workspace、generated client 和本地开发基线。
- P20 已建立 Portal Web BFF、Portal session、Portal UI、Portal smoke 和 `/api/portal/v1/*`。
- P21 已建立 Admin Web BFF、operator session、RBAC、audit、owner-service write workflow 和 `/api/admin/v1/*`。
- P22 仍待完成完整 Admin UI、Portal/Admin E2E smoke、static asset、安全头和 console 生产发布闭环。
- P23 已完成 Console directory alignment，覆盖 backend/frontend app/shared package 拆分、import boundary 和最终全量验收。
- P24 design 已将 NewAPI 测试环境观察到的模块拆成 keep/cut decision，并定义了 token-gateway 自己的信息架构和页面设计方式。

## 范围决策

### 进入 P24

- Channel Management：provider/channel safe CRUD、credential rotation、channel test、health、model binding、sync preview/apply、route policy hint。
- Model Management：model catalog metadata、category、modalities、capabilities、schema preview、pricing summary、channel coverage、Portal preview。
- Customer Account Management：tenant/project scoped customer account、status、role、credits、API key、session reset、audit。
- API Key Management：create/disable/rotate、allowed models、IP allowlist、expires_at、usage summary。
- Usage Logs：request_id drilldown、tenant/project/API key/model/channel/provider filters、usage/cost/settlement safe detail。
- Task Logs：task_id drilldown、async lifecycle、provider task、callback、settlement status。
- Playground：schema-driven Portal/Admin model test、safe debug、channel test executor reuse。
- Minimal Credit Operations：balance/ledger read model、manual adjustment、failed settlement links、export。

### 不进入 P24

- NewAPI 用户分组、模型分组、渠道分组。
- NewAPI model ratio/group ratio/channel ratio。
- 订阅套餐、套餐升级分组、兑换码、第三方支付配置、邀请返利。
- io.net 或其他模型部署服务。
- 系统设置大杂烩页面、菜单可配置平台、签到、复杂首页配置。
- 任意 request body/header passthrough、系统提示词注入、思考内容转换、任意状态码复写。
- 客户自选 provider/channel。

## 交付物

- `docs/design/ai_gateway_newapi_lean_console_design.md` 作为 P24 设计真相源。
- `docs/plan/35-p24-newapi-lean-console-parity.md` 作为 P24 执行计划。
- `docs/tasks.md` P24 任务拆分和当前状态。
- Admin IA 与 route plan：Workbench、Catalog、Routing、Accounts、Activity、Tools、Settings。
- Portal IA 补强：Dashboard、Models、Playground、API Keys、Usage、Tasks、Credits、Project Settings。
- Admin BFF OpenAPI 更新：channels、models、customer accounts、api keys、usage logs、task logs、playground。
- Portal BFF OpenAPI 更新：models detail/schema、playground、api keys、usage/tasks/credits/ledger。
- Generated TypeScript client 更新和 frontend query hooks。
- Channel management UI：list/detail/drawer/workflows/test/sync preview/audit。
- Model management UI：catalog/detail/schema/pricing/channel coverage/Portal preview。
- Customer account UI：account/detail/credits/API keys/usage/tasks/audit。
- Usage/task log UI：filters、safe detail、request_id/task_id drilldown、export。
- Playground UI：schema-driven payload editor、safe debug、Admin/Portal scope separation。
- Focused tests、contract tests、frontend tests、E2E smoke 和 runbook 更新。

## 核心实现顺序

1. 固化 P24 cut-scope guard：OpenAPI、tasks、UI route plan 明确不出现 group/ratio/payment/subscription/redemption/deployment。
2. 对齐 frontend IA：在 P22/P23 基础上建立 Admin 的 Workbench/Catalog/Routing/Accounts/Activity/Tools 信息架构，不复制 NewAPI 菜单。
3. 扩展 Admin BFF safe DTO 和 OpenAPI：先补 channels/models/customer accounts/api keys/logs/tasks/playground 的 read schema。
4. 实现 Channel Management read path：列表、详情、health、model coverage、cost status、audit safe view。
5. 实现 Channel Management write workflows：create/edit/enable/disable/rotate credential/test/sync preview/apply，全部 reason + idempotency + audit。
6. 实现 Model Management：catalog metadata、category、schema preview、pricing summary、channel coverage、Portal preview 和 sync preview。
7. 实现 Customer Account Management：tenant/project scoped account、status、role、credits、API key、session reset 和 manual adjustment。
8. 收口 API Key Management：Admin 与 Portal 共用 owner service 规则，保证 plaintext key 只显示一次，allowed models 不扩大权限。
9. 实现 Usage Logs 和 Task Logs：read model、filters、safe detail、export、request_id/task_id drilldown。
10. 实现 Playground：按 OpenAPI/schema 生成 payload 表单，执行 safe test，显示 route/channel/usage/debug safe result。
11. 补齐 Portal model detail、playground、ledger/credits、usage/tasks 体验，不开放 Admin 能力。
12. 接入 generated client、frontend tests、contract tests、RBAC tests、audit/redaction tests 和 E2E smoke。
13. 更新 runbook、release gate 和 known limits，确认 P24 non-goals 没有进入 UI 或 API。

## API Scope

P24 具体 path 可以按现有 `api/openapi/admin-bff.yaml` 风格调整，但 browser Admin 只能使用 `/api/admin/v1/*`，不能调用 `/admin/*`。

```text
Admin:
  GET/POST/PATCH channels
  POST channels/{id}/enable
  POST channels/{id}/disable
  POST channels/{id}/rotate-credential
  POST channels/{id}/test
  POST channels/{id}/sync-preview
  POST channels/{id}/sync-apply
  GET  channels/{id}/health-events

  GET/POST/PATCH models
  POST models/{id}/disable
  POST models/{id}/deprecate
  GET  models/{id}/channels
  GET  models/{id}/schema-preview
  POST models/sync-preview

  GET/POST/PATCH customer-accounts
  POST customer-accounts/{id}/disable
  POST customer-accounts/{id}/adjust-credit
  POST customer-accounts/{id}/reset-session

  GET/POST api-keys
  POST api-keys/{id}/disable
  POST api-keys/{id}/rotate

  GET usage-logs
  GET usage-logs/{request_id}
  GET task-logs
  GET task-logs/{task_id}

  POST playground/run
  POST playground/import-preview
  POST playground/export

Portal:
  GET  models
  GET  models/{id}
  GET  models/{id}/schema
  POST playground/run
  GET/POST api-keys
  POST api-keys/{id}/disable
  GET  usage
  GET  tasks
  GET  credits
  GET  ledger
```

## 页面范围

### Admin Routing / Channels

- Summary cards：healthy、degraded、disabled、missing cost、missing model binding、snapshot pending。
- Filters：provider、protocol、capability、health、enabled、cost status、model keyword、tag。
- Detail tabs：Basics、Credentials、Model Coverage、Health、Policy、Audit。
- Dangerous workflows：disable、rotate credential、sync apply 都需要 reason、confirm、idempotency 和 audit。

### Admin Catalog / Models

- Summary cards：active、deprecated、missing price、missing provider cost、no healthy channel、schema missing。
- Filters：category、capability、status、provider coverage、tag、keyword。
- Detail tabs：Overview、Schema、Pricing、Channels、Portal Preview、Audit。
- Pricing 只展示 component price summary，不出现倍率输入。

### Admin Accounts / Customer Accounts

- Filters：status、tenant/project、role、balance range、model access、keyword。
- Detail tabs：Overview、Credits、API Keys、Usage、Tasks、Audit。
- Manual adjustment 必须走 billing owner service，不直接改余额字段。

### Admin Activity

- Usage Logs：request_id、client_request_id、tenant/project、API key、model、channel、provider、status、time range。
- Task Logs：task_id、tenant/project、model、channel、provider、task status、callback status、settlement status。
- Detail 只返回 safe metadata。

### Tools / Playground

- schema-driven form。
- Admin test 与 Portal test 范围隔离。
- Debug 只展示 request_id、route/channel、latency、usage、safe error。
- 导入导出不含 secret。

## 关键设计约束

- P24 不改 `/v1/*`、`/v1/portal/*`、`/admin/*` 的职责。
- Admin browser 仍只调用 `/api/admin/v1/*`。
- Portal browser 仍只调用 `/api/portal/v1/*`。
- Admin mutation 必须校验 operator session、RBAC、CSRF、Idempotency-Key 和 reason。
- Credential、API key、raw prompt、raw response、raw provider error、raw repair payload 不返回前端。
- Channel/model/customer 写操作必须调用 owner service：`configadmin`、snapshot、billing、task/worker。
- P24 不创建全局 service/repository，不把 control/config owner 合并进 `internal/app/admin`。
- UI 不绕过 generated client 直接裸 fetch。
- OpenAPI 改动必须同步 generated client 和 contract tests。
- Frontend shared packages 不放业务 feature 组件。

## 验收标准

- P24 design、plan、tasks 和 design index 一致，明确 keep/cut decision。
- Channel Management 支持 safe CRUD、credential rotation、test、sync preview/apply、health read model 和 audit，credential 不泄漏。
- Model Management 支持 catalog metadata、category、schema、pricing summary、channel coverage 和 Portal preview，UI/API 不出现倍率。
- Customer Account Management 支持 tenant/project scoped account、status、credits、API keys、manual adjustment 和 audit，UI/API 不出现用户分组。
- API Key Management 支持 create/disable/rotate/allowed models/IP/expires，plaintext key 只显示一次。
- Usage Logs 和 Task Logs 支持关键筛选、safe detail、request_id/task_id drilldown 和 export。
- Playground 支持 schema-driven test、safe debug、Admin/Portal scope separation。
- Admin/Portal frontend 只通过 generated client 调 BFF，不调用 `/admin/*`。
- RBAC deny、CSRF deny、audit redaction、credential redaction 和 raw payload denylist 有 focused tests。
- `pnpm generate:api`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build`、Go focused tests、contract tests、E2E smoke 和 `git diff --check` 通过。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 对标 NewAPI 变成复制 NewAPI | P24 design 记录 keep/cut matrix，UI IA 重做，不复用 NewAPI 菜单结构 |
| 分组/倍率绕回实现 | OpenAPI schema、UI 文案、tasks 和 tests 明确禁止 group/ratio 字段 |
| Channel UI 绕过 owner service | Admin service 只做 orchestration，配置写入委托 `configadmin` |
| Channel sync 自动发布未知模型 | 只允许 preview -> approve -> snapshot，默认不 customer visible |
| Playground 变成 raw proxy | schema-driven form，raw JSON 只做 import-preview，不绕过 parser/schema |
| 用户管理扩张成完整 IAM | P24 只做 customer account/project role，不接 SSO/OIDC/group mapping |
| 日志详情泄漏 raw payload | safe DTO + denylist contract tests |
| 支付/订阅/兑换码被顺手加入 | P24 non-goals 明确禁止；后续如需要单独开商业支付阶段 |

## 依赖关系

- 依赖 P19 的 console process、OpenAPI split、frontend workspace 和 generated client。
- 依赖 P20 的 Portal Web BFF、Portal session 和 Portal frontend。
- 依赖 P21 的 Admin Web BFF、operator session、RBAC、audit 和 owner-service workflow。
- 依赖 P22 的 Admin UI、static asset、安全头、E2E smoke 和 production release gate；如果 P22 未完成，P24 作为其功能范围输入。
- 依赖 P23 的目录结构、frontend app/shared package 拆分和 import boundary；如果 P23 未完成，P24 页面实现应避免扩大单文件。

## 设计来源

- [NewAPI Lean Console Parity 设计](../design/ai_gateway_newapi_lean_console_design.md)
- [Portal/Admin Console Monorepo 设计](../design/ai_gateway_console_monorepo_design.md)
- [P22 Console Frontend Production](./33-p22-console-frontend-production.md)
- [P23 Console Directory Structure Alignment](./34-p23-console-directory-structure-alignment.md)
- [路线图](./00-roadmap.md)
- [任务清单](../tasks.md)
