# P25 Console Frontend Data Connection and Routing Hardening

## 阶段目标

P25 接在 P22 Console Frontend Production 和 P24 NewAPI Lean Console Parity 之后，目标是把已经交付的 Admin/Portal BFF 与 generated client 真正接入生产前端页面，同时补齐路由、列表状态、最小共享 UI primitives 和 SiliconFlow-like 控制台视觉风格。P25 不再扩大 NewAPI 对标范围，而是把当前 review 指出的 sample data、App 过度集中、route 不完整、前端基础设施缺口和视觉风格不一致收敛为可验收的工程任务。

## 当前状态

- Admin 后端 BFF、OpenAPI 和 feature API helper 已覆盖渠道、模型、客户账户、API keys、活动日志、额度运营和操练场。
- Admin `DangerOperationsPanel` 已经使用真实 API，但多数组件仍保留 sample data 作为生产展示。
- Portal BFF 与 Portal 页面功能可用，但 `App.tsx` 承担了过多 session、route、数据加载和业务状态。
- Portal/Admin 均已有 route metadata，Admin 支持初始 path 解析，Portal 尚未形成完整 URL 状态；两端都需要 back/forward 行为收口。
- `web/packages/ui` 与 `web/packages/auth` 仍是最小公共层，P25 需要补齐当前页面真实连通所需的表格、分页、确认、toast、loading 和复制能力。
- SiliconFlow Cloud `/me/models` 登录态页面呈现轻量云控制台风格：约 200px 白底侧栏、紫色 active、白底内容区、浅灰 compact cards、8px 圆角、低阴影、40px 搜索/筛选控件和高密度 tags；当前 Portal 是深色侧栏/绿色主色，Admin 是蓝色主色/260px rail，两端需要统一。

## 范围决策

### 进入 P25

- Admin P24 面板移除生产 sample data，接入 `/api/admin/v1/*` BFF。
- Admin mutation 统一 CSRF、reason、idempotency 和 audit 约束。
- Portal `App.tsx` 拆分为 shell、session、route 和 feature data hooks。
- Admin/Portal browser history、direct link、refresh 和 popstate 行为收口。
- 最小共享 UI primitives：DataTable、Pagination、FilterBar、ConfirmDialog、Drawer/Modal、Toast、Skeleton、CopyButton。
- SiliconFlow-like console theme tokens：轻色 shell、200px sidebar、紫色 primary、compact cards/tables/tags/filter controls、8px card/button radius 和低阴影。
- Portal usage、tasks、credits ledger 等列表补齐分页、加载、错误和空态。
- P25 smoke/runbook/release evidence 覆盖 Admin/Portal 真实数据路径。

### 不进入 P25

- 用户分组、模型分组、渠道分组、倍率、订阅、兑换码、支付、邀请返利、模型部署和复杂系统设置。
- charts dashboard、暗色主题、Storybook、完整设计系统和脱离 P25 真实页面的大规模视觉重设计。
- SiliconFlow logo、品牌素材、活动 banner 文案、模型描述文案或商标复制。
- 完整 React Router 迁移或全局状态库引入。
- 对象存储、完整 Realtime、复杂财务/发票闭环。
- 浏览器 Admin 直连 `/admin/*` 或使用 control admin token。

## 交付物

- `docs/design/ai_gateway_console_frontend_data_connection_design.md`
- `docs/plan/36-p25-console-frontend-data-connection-hardening.md`
- `docs/tasks.md` 的 P25 执行拆分和验收矩阵
- Admin/Portal 前端代码、shared UI/auth/client 代码、focused tests 和 smoke 证据
- shared console theme tokens 与 Portal/Admin shell/page 视觉对齐
- 必要时的 OpenAPI/generated client 微调，但不改变 P24 产品边界

## 实施顺序

1. 先收口 Admin API client mutation contract，避免后续面板迁移重复实现 CSRF/reason/idempotency。
2. 先迁移读多写少的 Admin 面板，再迁移高风险 mutation 面板。
3. 在真实数据接入过程中抽取 UI primitives，不先做空泛组件库。
4. 以 shared theme tokens 统一 Portal/Admin shell、sidebar、cards、tables、tags 和 filters，视觉对齐随真实页面迁移一起完成。
5. Admin/Portal route helper 与 popstate 收口并行推进，但 Portal `App.tsx` 拆分按 feature 分批完成。
6. 最后做 cut-scope、boundary、API drift、frontend build/test 和 console smoke。

## 任务拆分

| ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|
| P25/E35-T01/P0 | P0 | 固化 Admin mutation client contract | `web/apps/admin/src/shared/api`, `web/apps/admin/src/features/*/*Api.ts`, tests | 所有 Admin 写操作经过 shared wrapper 或等价 feature wrapper，CSRF、reason、idempotency 和错误归一可测试 |
| P25/E35-T02/P0 | P0 | Channel Management 接入真实 BFF | `web/apps/admin/src/features/channels` | 列表、筛选、详情、test、rotate、sync preview/apply、enable/disable 使用 BFF，生产组件无 `sampleChannels` |
| P25/E35-T03/P0 | P0 | Model Management 接入真实 BFF | `web/apps/admin/src/features/models` | catalog、schema preview、coverage、sync preview、disable/deprecate 使用 BFF，生产组件无 `sampleModels` |
| P25/E35-T04/P0 | P0 | Customer Accounts 与 Credit Report 接入真实 BFF | `web/apps/admin/src/features/accounts`, `web/apps/admin/src/features/credits` | 账户、API key summary、session reset、manual adjustment、credit report、ledger/export 联动真实 BFF，生产组件无账户和额度 sample data |
| P25/E35-T05/P0 | P0 | API Key Management 接入真实 BFF | `web/apps/admin/src/features/apiKeys` | create/update/enable/disable/rotate、一次性明文展示和权限边界走 BFF，生产组件无 `sampleAPIKeys` |
| P25/E35-T06/P1 | P1 | Activity Logs 接入真实 BFF | `web/apps/admin/src/features/activity` | usage/task filters、cursor/limit、详情 safe metadata 和复制入口可用，生产组件无日志 sample data |
| P25/E35-T07/P1 | P1 | Tools Playground 接入真实 BFF | `web/apps/admin/src/features/tools` | run、import preview、export 使用 BFF，payload/result 脱敏，生产组件无 `sampleResult` |
| P25/E35-T08/P1 | P1 | 抽取最小 shared UI primitives | `web/packages/ui`, Admin/Portal features | DataTable、Pagination、FilterBar、ConfirmDialog、Drawer/Modal、Toast、Skeleton、CopyButton 被真实页面复用并有 focused tests |
| P25/E35-T09/P1 | P1 | Portal/Admin SiliconFlow-like 视觉风格对齐 | `web/packages/ui`, `web/apps/admin/src/styles.css`, `web/apps/portal/src/styles.css`, feature pages | 两端共享 console theme tokens，使用轻色 200px sidebar、紫色 primary、compact cards/tables/tags/filter controls、8px card/button radius 和低阴影；不复制 SiliconFlow logo、品牌素材或文案 |
| P25/E35-T10/P1 | P1 | Admin/Portal route 与 popstate 收口 | `web/apps/admin/src/app`, `web/apps/portal/src/app` | 直接访问子路由、刷新、浏览器 back/forward 均恢复正确 view，不散落 pathname 字符串 |
| P25/E35-T11/P1 | P1 | Portal App 拆分与列表分页硬化 | `web/apps/portal/src/app`, `web/apps/portal/src/features` | `App.tsx` 只做入口装配，session/route/feature hooks 拆分；usage/tasks/ledger 有分页、loading、error、empty state |
| P25/E35-T12/P1 | P1 | P25 cut-scope、API drift 和 regression tests | `tests`, `api/openapi`, `web` | `make p24-cut-scope-check` 覆盖 P25 前端路径；`make api-check`、frontend lint/typecheck/test/build 通过 |
| P25/E35-T13/P1 | P1 | P25 smoke、runbook 和 release evidence | `tools`, `docs/runbook`, release handoff | Admin 真实数据 workflow、Portal route/list workflow 和 Portal/Admin 视觉对齐截图 smoke 可重复执行，handoff 记录 known limits 和验证命令 |

## 验收命令

P25 docs-only 规划落库只要求：

```bash
git diff --check
rg -n "[[:blank:]]$" docs/design/ai_gateway_console_frontend_data_connection_design.md docs/plan/36-p25-console-frontend-data-connection-hardening.md docs/tasks.md
```

P25 实现完成时至少要求：

```bash
make p24-cut-scope-check
make boundary-check
make api-check
pnpm lint
pnpm typecheck
pnpm test
pnpm build
go test ./internal/app/admin/... ./internal/transport/adminhttp ./internal/app/portal/... ./internal/transport/portalwebhttp ./tests/contract
go run ./tools/p24-console-smoke -console-url http://127.0.0.1:9515 -api-key tg-local-dev-key -admin-email admin@example.com -admin-password admin-local -model gpt-4o-mini -create-derived-key
```

如实现过程中新增 P25 专用 smoke，应在最终验收中替换或补充 `p24-console-smoke`，但仍保留 P24 cut-scope guardrail。

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| 面板一次性迁移过多导致回归难定位 | 按任务逐面板接入，每个面板保留 focused test 和 browser smoke |
| mutation helper 分散导致安全头不一致 | T01 优先完成 shared contract，后续面板不再自行拼 headers |
| 后端真实数据为空时误判为未连通 | 空态显示筛选条件、数据来源和刷新入口，smoke 检查 network/BFF 响应 |
| Portal 拆分影响登录恢复 | 先拆 session hook 并加登录恢复测试，再迁移 feature data |
| 共享 UI primitives 膨胀 | 只抽取 P25 页面实际复用组件，未复用交互留在 feature 内 |
| 视觉对齐误伤品牌边界 | 只使用布局密度、颜色 token、圆角、间距和组件行为作为参考，不复制 SiliconFlow logo、活动文案、模型描述或其他品牌资产 |

## 依赖关系

- 依赖 P21 Admin Web BFF 的 session、RBAC、audit 和 mutation guard。
- 依赖 P22 Console Frontend Production 的 app shell、session restore、static asset 和 security headers。
- 依赖 P24 Admin/Portal lean parity 的 BFF endpoints、OpenAPI/generated client 和 cut-scope regression。
- 不依赖对象存储、Realtime、复杂财务/发票和 NewAPI 裁剪外模块。

## 设计来源

- `docs/design/ai_gateway_console_frontend_data_connection_design.md`
- `docs/plan/34-p22-console-frontend-production.md`
- `docs/plan/35-p24-newapi-lean-console-parity.md`
- `docs/design/ai_gateway_console_monorepo_design.md`
- `docs/tasks.md`
