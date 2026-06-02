# P23 Console Directory Structure Alignment

## 阶段目标

P23 的目标是把 P19-P21 已经实现的 Console、Portal Web BFF、Admin Web BFF 和前端 workspace，按最终目标目录树做一次行为保持型结构对齐。

P22 关注 Admin UI、E2E、static asset、安全头和生产发布闭环；P23 专门处理目录、文件拆分和 import 边界，避免在 P22 继续把页面、handler、service、repository 都堆进少数大文件。

本阶段不新增产品能力，不改变对外 API，不改变 `/v1/*`、`/v1/portal/*`、`/api/portal/v1/*`、`/api/admin/v1/*`、`/admin/*` 的行为。它只做可验证的模块拆分、命名收敛和开发体验补强。

## 边界决策

### Admin：重命名 control owner，但不合并进 `internal/app/admin`

`internal/controlplane/admin` 和 `internal/app/admin` 不是同一个 bounded context，P23 不把前者合并进后者。为了彻底降低命名误导，P23 应把 `internal/controlplane/admin` 做 rename-only 调整为 `internal/controlplane/configadmin`。

- `internal/controlplane/configadmin` 继续是 machine control/config owner，负责 tenant、project、API key、model、channel、route、pricing、limit、snapshot 相关配置写入、校验、credential encryption 和 snapshot 输入。
- `internal/app/admin` 继续是 browser Admin Web BFF application，负责 operator session、RBAC、CSRF、Idempotency-Key、reason guard、durable audit、safe DTO、UI read model 和调用 owner service。
- Admin 端新增渠道、新增模型、新增 route、调整价格、调整 limit、发布/回滚 snapshot 等浏览器操作，执行链路必须是 `adminhttp -> app/admin service -> configadmin.Service 或 snapshot/billing/task owner service -> audit`。
- `internal/app/admin/repository` 可以做 dashboard、config safe read model、operations views、operator/session/RBAC/audit store，但不得绕过 `configadmin.Service` 直接写配置表。
- `internal/controlplane/config` 不作为本阶段首选：它过于通用，容易和启动配置、运行配置、`Config` 类型混淆。
- `internal/controlplane/management` 不作为本阶段首选：它范围过宽，容易把 operations、billing repair、worker control 等非配置 owner 也吸进去。
- rename 只改 Go import path、package name、docs 和 tests，不改变 `/admin/*` machine API 路径、OpenAPI contract、DB schema 或 service 方法语义。

### Portal：逐步并入 `internal/app/portal`

`internal/portal` 和 `internal/app/portal` 的关系更接近“legacy public Portal service 与 browser Portal app service 的过渡”。P23 可以推进合并，但必须保留兼容窗口。

- `internal/portal` 当前承载 `/v1/portal/*` programmatic customer API，使用 Bearer API key 鉴权，属于 P8 public Portal API 的历史实现。
- `internal/app/portal` 承载 `/api/portal/v1/*` browser Portal BFF，使用 HttpOnly session、CSRF 和 dashboard/onboarding 聚合。
- P23 允许把 `internal/portal` 的核心 use case 迁移到 `internal/app/portal/service`，但 `internal/portal` 先保留 compatibility shim，直到 `/v1/portal/*` contract tests 和 bootstrap/transport imports 全部切换并验证通过。
- `/v1/portal/*` 的 Bearer API key 语义和 `/api/portal/v1/*` 的 browser session 语义必须保持分离，不得因为目录合并而混用 auth、response DTO 或错误语义。

## 当前状态

- P19 已落地 `cmd/console`、OpenAPI split、frontend workspace、generated client、Makefile/CI 和本地开发 runbook。
- P20 已落地 Portal Web BFF、Portal session、Portal UI、Portal smoke 和 `/api/portal/v1/*`。
- P21 已落地 Admin Web BFF、operator session、RBAC、audit、owner-service write workflow 和 `/api/admin/v1/*`。
- 当前实现仍有部分文件偏集中，例如 `portalwebhttp/handler.go`、`adminhttp/handler.go`、`internal/app/*/service/service.go`、frontend `App.tsx` 和 shared package `index.ts`。
- 当前命名容易让人误以为 `internal/controlplane/admin` 可以被 `internal/app/admin` 吸收；P23 必须先完成 `configadmin` rename 和边界检查，再做大规模文件拆分。

## 交付物

- API 目录补齐：
  - `api/examples/portal`
  - `api/examples/admin`
  - `api/examples/control`
  - `api/scripts/lint_openapi.sh`
  - `api/scripts/generate_ts_client.sh`
  - `api/scripts/check_api_diff.sh`
  - optional generated `api/openapi/bundle.yaml`
- Portal public API 迁移与 app backend 拆分：
  - `internal/app/portal/service/{ports,public_api,web_bff,auth,dashboard,models,credits,usage,api_keys,tasks,onboarding,sessions,helpers}.go`
  - `internal/app/portal/repository/{repository,memory,mysql,mysql_models,mysql_credits,mysql_usage,mysql_api_keys,mysql_tasks,mysql_dashboard,mysql_sessions}.go`
  - `internal/portal` 保留 compatibility shim，转发到 `internal/app/portal` 或保留薄适配层，直到所有 `/v1/portal/*` imports 和 contract tests 完成迁移。
- Admin app backend 拆分：
  - `internal/app/admin/service/{ports,auth,tenants,projects,api_keys,models,channels,routes,pricing,limits,snapshots,emergency,workers,settlements,callbacks,audit,dashboard,sessions,helpers}.go`
  - `internal/app/admin/repository/{repository,memory,mysql,mysql_sessions,mysql_operators,mysql_dashboard,mysql_tenants,mysql_projects,mysql_models,mysql_channels,mysql_routes,mysql_pricing,mysql_limits,mysql_operations,mysql_audit}.go`
  - 保留 `internal/app/admin -> internal/controlplane/configadmin` 的 owner-service 调用方向；不得把 `configadmin` 配置写方法移动到 `app/admin`。
- Control/Admin 命名防混淆：
  - `internal/controlplane/admin` rename 为 `internal/controlplane/configadmin`，package name 使用 `configadmin`。
  - Go imports 使用 `configadmin`，不再使用 `cpadmin` 作为长期别名。
  - import boundary check 明确禁止 `controlplane/configadmin -> app/admin` 反向依赖。
- Transport 层拆分：
  - `internal/transport/consolehttp/{handler,session,csrf,static}.go`
  - `internal/transport/portalhttp/{handler,auth,models,credits,usage,api_keys,tasks,response}.go`
  - `internal/transport/portalwebhttp/{handler,auth_handler,dashboard_handler,models_handler,credits_handler,usage_handler,api_keys_handler,tasks_handler,onboarding_handler,response}.go`
  - `internal/transport/adminhttp/{handler,auth_handler,dashboard_handler,tenants_handler,projects_handler,api_keys_handler,models_handler,channels_handler,routes_handler,pricing_handler,limits_handler,snapshots_handler,operations_handler,audit_handler,response}.go`
- Frontend apps 拆分：
  - `web/apps/portal/src/app/{main,routes,providers}.tsx`
  - `web/apps/portal/src/features/*`
  - `web/apps/portal/src/shared/*`
  - `web/apps/admin/src/app/{main,routes,providers}.tsx`
  - `web/apps/admin/src/features/*`
  - `web/apps/admin/src/shared/*`
- Frontend packages 拆分：
  - `web/packages/api-client/src/{portal-public,portal-bff,admin-bff,control,fetcher,errors}.ts`
  - `web/packages/auth/src/{session,csrf,permissions}.ts`
  - `web/packages/format/src/{money,tokens,date,status}.ts`
  - `web/packages/ui/src/{button,form,table,dialog,chart,toast,empty-state}`
  - optional `web/packages/config` and `web/packages/test-utils`
- 文档和 import 边界同步：设计文档、plan、tasks、runbook、README 或 ARCHITECTURE 只记录实际存在且可维护的目标结构。

## 核心实现顺序

1. 锁定 Admin/Portal owner 决策，生成当前结构与目标结构的差异清单，标记“必须拆”“可延后”“不应该拆”的文件。
2. 先做 rename-only：`internal/controlplane/admin -> internal/controlplane/configadmin`，同步 package name、imports、tests 和 docs，确认 `/admin/*` API 行为不变。
3. 为 `internal/controlplane/configadmin` 与 `internal/app/admin` 增加边界检查策略，先防止后续实现继续混淆。
4. 先拆 transport 层 handler：按 resource 移动 handler 方法，保留 route registration 和 response/error helpers，确保 route 行为不变。
5. 迁移 `internal/portal` 核心 use case 到 `internal/app/portal/service`，保留 `internal/portal` compatibility shim，并让 `/v1/portal/*` contract tests 继续覆盖旧入口。
6. 拆 Portal service/repository：按 public API、web BFF、session、dashboard、read model 移动方法，保留 public method signatures 和 existing tests。
7. 拆 Admin service/repository：按 auth、dashboard、config safe read、pricing、operations、audit、sessions 维度拆分，保证 RBAC/audit/owner-service write flow 不变。
8. 拆 frontend Portal/Admin app：把现有 `App.tsx` 中的 route、page、feature hooks、shared layout 逐步抽出，避免业务组件进入 `web/packages/ui`。
9. 拆 shared packages：api-client、auth、format、ui 按职责分文件，保留现有 public exports，避免破坏 app imports。
10. 补齐 `api/examples` 和 `api/scripts`，将 Makefile/CI 的 OpenAPI generation、lint 和 diff check 指向这些脚本。
11. 补齐 import boundary 检查或轻量脚本，防止 `web/packages/* -> web/apps/*`、Admin browser -> `/admin/*`、Portal app -> Admin app、`controlplane/configadmin -> app/admin` 这类反向依赖。
12. 更新 docs/tasks.md、docs/runbook、ARCHITECTURE 或 README 中的结构说明。
13. 跑全量验证和 focused regression，确认重构没有行为变化。

## 关键设计约束

- P23 是行为保持型 refactor；不新增 API，不改 response schema，不改 auth/session/RBAC/audit 语义。
- 不创建空包。每个新增文件都必须承载实际方法、类型、测试或脚本。
- 不把 `internal/transport` 改回 `internal/http`，当前仓库以 `internal/transport/*http` 为传输层约定。
- 不新增全局 `internal/store` 或全局 `repository`；repository 仍按 bounded context 存放。
- 不移动 core domain ownership：billing、task、controlplane、snapshot、dataplane、provider 的 owner 边界不变。
- 不把 `internal/controlplane/configadmin` 合并进 `internal/app/admin`，也不让 `internal/app/admin/repository` 直接写 control/config owner 表。
- `internal/portal` 迁移必须保留 `/v1/portal/*` Bearer API key 行为；browser Portal session 继续留在 `internal/app/portal`。
- Frontend shared packages 只放跨 app 复用能力，不放业务 feature 组件。
- `cmd/console` 继续只承载 browser BFF 和可选静态资源，不接管 data plane 或 machine control API。

## 验收标准

- P19/P20/P21 已通过的 focused tests 继续通过。
- 所有 route path、OpenAPI operationId、security scheme 和 generated TS client public exports 保持兼容，除非在 plan 中明确记录并同步调用方。
- `/admin/*` machine Control API 行为不变；Admin browser 仍只能调用 `/api/admin/v1/*`。
- `/v1/portal/*` programmatic Portal API 行为不变；Portal browser 仍只能通过 `/api/portal/v1/*` 使用 session BFF。
- `go test ./internal/controlplane/configadmin ./internal/app/admin/... ./internal/transport/adminhttp ./internal/transport/controlhttp` 通过。
- `go test ./internal/portal ./internal/app/portal/... ./internal/transport/portalhttp ./internal/transport/portalwebhttp` 通过。
- `go test ./...`、`go vet ./...`、`go build ./cmd/...` 通过。
- `pnpm generate:api`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build` 通过。
- `make api-check` 或等价 OpenAPI diff check 通过。
- Portal web smoke 和 Admin BFF focused tests 通过。
- `git diff --check` 和 markdown trailing-whitespace scan 通过。
- 目标目录树中承诺的关键文件要么已落地，要么在 P23 完成记录中明确说明延后原因。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 把 `configadmin` 合并进 `app/admin` 导致 browser BFF 接管 machine config owner | 明确禁止合并；Admin mutation 只做权限、审计、DTO 和 workflow orchestration，配置写入继续委托 owner service |
| `configadmin` rename 扩散成行为修改 | 单独做 rename-only 小步骤；只改 import path/package/docs/tests，不改 service 方法语义、HTTP path、OpenAPI 或 DB schema |
| 过早删除 `internal/portal` 破坏 P8 `/v1/portal/*` | 保留 compatibility shim，先迁移内部实现，再通过 contract tests 和 bootstrap imports 验证后删除 |
| 纯移动文件导致行为回归 | 每一步保持小提交/小 diff，移动后立即跑 focused tests |
| 拆分后 import cycle | 先定义 ports 和 helper ownership，再移动实现；避免 service 与 repository 互相引用 |
| 过早创建空文件 | 只有承载真实代码或脚本时才创建文件；目标树不是空文件清单 |
| P22 UI 开发被结构调整阻塞 | 先拆高增长文件：Admin App、Admin handler、Admin service，再拆低风险 shared packages |
| generated client 路径变化破坏前端 | 保留 barrel exports，新增内部目录不改变 app import |
| docs 与实际目录再次漂移 | P23 完成时用 `find`/`tree` 快照更新 runbook 或 ARCHITECTURE |

## 执行时机

P23 按编号位于 P22 之后，但它是结构治理阶段。如果 P22 开始前发现 Admin UI 或 handler/service 继续扩张会导致单文件过大，可以先执行 P23 的 backend/frontend 拆分子集，再继续 P22。

执行时必须保持 P23 的非功能性边界：只拆结构，不新增产品能力。`configadmin` rename、Admin 配置写入边界和 Portal 兼容 shim 是 P23 的前置条件，不是实现过程中的可选项。

## 设计来源

- [路线图](./00-roadmap.md)
- [P19 Console Monorepo Foundation](./30-p19-console-monorepo-foundation.md)
- [P20 Portal Web BFF](./31-p20-portal-web-bff.md)
- [P21 Admin Web BFF](./32-p21-admin-web-bff.md)
- [P22 Console Frontend Production](./33-p22-console-frontend-production.md)
- [Portal/Admin Console Monorepo 设计](../design/ai_gateway_console_monorepo_design.md)
- [任务清单](../tasks.md)
