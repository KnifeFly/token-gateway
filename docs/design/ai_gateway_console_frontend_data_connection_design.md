# Console Frontend Data Connection and Routing Hardening 设计

## 背景

P22 已完成 Console 前端生产化壳层，P24 已补齐 Admin/Portal 所需的 lean NewAPI parity 后端能力、BFF OpenAPI、generated client 和若干前端页面入口。最新 review 的核心问题不再是缺少后端 API，而是前端生产页面仍有明显断层：

- Admin 多数 P24 面板已经有 feature API helper，但生产面板仍保留 sample data，用户看到的不是 BFF 实时数据。
- Portal `App.tsx` 聚合了认证、数据加载、导航、页面状态和业务事件，后续迭代容易形成高风险单点。
- Admin 已能从 URL 解析初始 route，但缺少浏览器 back/forward 同步；Portal route metadata 已存在，但页面切换仍主要依赖内存状态。
- `web/packages/ui` 与 `web/packages/auth` 仍是最小公共层，列表、分页、确认、toast、loading、复制等生产交互在各页面内重复实现或缺失。

P25 的设计目标是把 P24 已交付的后端能力真正接到 Admin/Portal 前端生产路径，并在不扩张产品范围的前提下补齐路由、列表和最小共享 UI 基础设施。

## 目标

1. Admin P24 面板从 sample data 切换到 `/api/admin/v1/*` BFF 数据，并复用 generated client 类型。
2. Admin 写操作统一满足 CSRF、reason、idempotency 和 audit 约束，不允许绕过 shared mutation guard。
3. Portal 和 Admin 支持可分享 URL、刷新恢复当前页面，以及浏览器 back/forward。
4. Portal App 拆出 shell、session、feature data hooks 和 route state，降低单文件状态耦合。
5. 建立足够支撑 P25 的共享 UI primitives，避免每个面板重复实现表格、分页、确认、toast 和加载态。
6. 保持 P24 cut-scope：只连通已进入范围的渠道、模型、客户账户、令牌、日志、任务、操练场和最小额度运营。
7. Portal/Admin 视觉风格对齐 SiliconFlow Cloud 控制台的轻量云产品客户端样式，但不复制对方 logo、品牌素材或文案。

## 非目标

- 不新增用户分组、模型分组、渠道分组、倍率、订阅、兑换码、支付、邀请返利、模型部署或复杂系统设置。
- 不改动数据面 `/v1/*`、兼容 Portal `/v1/portal/*` 或 control `/admin/*` 的既有职责。
- 不让浏览器 Admin 直接调用 `/admin/*`，也不把 control admin token 暴露给浏览器。
- 不在 P25 引入全局状态库、完整 React Router 迁移、Storybook、图表体系、暗色主题或脱离当前业务页面的大规模视觉重设计。
- 不把对象存储、完整 Realtime、复杂财务/发票闭环纳入本阶段。
- 不复制 SiliconFlow 的商标、logo、插画、活动 banner 文案、模型描述文案或其他品牌资产；只提炼控制台布局、密度、颜色和交互语言。

## 范围

### Admin 数据连通

以下生产面板必须切换到真实 BFF 数据：

| 面板 | 现有 API helper | 数据来源 | P25 要求 |
|---|---|---|---|
| Channel Management | `channelsApi.ts` | `/api/admin/v1/channels` | 列表、筛选、详情、test、credential rotate、sync preview/apply、enable/disable 走 BFF |
| Model Management | `modelsApi.ts` | `/api/admin/v1/models` | catalog、schema preview、channel coverage、sync preview、disable/deprecate 走 BFF |
| Customer Accounts | `accountsApi.ts` | `/api/admin/v1/customer-accounts` | 账户列表、状态、API key summary、session reset、manual adjustment 走 BFF |
| API Key Management | `apiKeysApi.ts` | `/api/admin/v1/api-keys` | create/update/enable/disable/rotate、一次性明文展示和权限边界走 BFF |
| Activity Logs | `activityApi.ts` | `/api/admin/v1/usage-logs`, `/task-logs` | usage/task filters、详情 safe metadata 和 cursor/limit 走 BFF |
| Credit Operations | `creditsApi.ts` | `/api/admin/v1/customer-accounts/*` | credit report、active holds、ledger/export 和 adjustment 联动客户账户 |
| Tools Playground | `toolsApi.ts` | `/api/admin/v1/playground/*` | run、import preview、export 走 BFF，结果脱敏展示 |

Danger Operations 已经走真实 API，P25 只做交互一致性检查，不把它当成 sample-data 迁移对象。

### Portal 拆分与路由

Portal 已有 Dashboard、Models、Playground、API Keys、Usage、Tasks、Credits、Settings 等生产页面。P25 不重新设计这些能力，而是拆出：

- `AppShell`：导航、标题、session 状态、logout。
- `usePortalSession`：登录恢复、CSRF/session 状态和错误处理。
- `usePortalRoute`：根据 route metadata 维护当前页面、URL 写入、刷新恢复和 popstate。
- feature data hooks：按 dashboard/models/keys/usage/tasks/credits 拆分加载、刷新和错误状态。
- list state：usage、tasks、ledger 等列表使用 cursor/limit 或稳定的本地分页状态。

拆分后，`App.tsx` 只保留入口装配，不继续持有所有 feature 的业务状态。

### 共享 UI primitives

P25 只建设当前面板必须使用的最小集合：

- `DataTable`：列定义、空态、加载态、错误态、行点击和 action slot。
- `Pagination`：cursor/limit 或 page/limit 两种最小模式。
- `FilterBar`：搜索、select、date range 和 reset slot。
- `ConfirmDialog`：危险操作、reason 输入和确认 loading。
- `Drawer` 或 `Modal`：详情、创建、编辑和 schema preview。
- `Toast`：成功、失败、复制、提交中反馈。
- `Skeleton`：列表和详情加载占位。
- `CopyButton`：API key、request_id、task_id、curl 等复制反馈。

这些组件放在 `web/packages/ui`，但不要求一次性抽象成完整设计系统。只暴露被 Admin/Portal 实际使用的 API。

### SiliconFlow Cloud 风格对齐

参考页面：`https://cloud.siliconflow.cn/me/models`。当前页面在登录态下呈现的是轻量云产品控制台：左侧固定导航、顶部轻公告条、主内容区高密度卡片网格、紫色 active 状态和 Ant Design/Tailwind 风格控件。P25 需要把 Portal/Admin 调整到同类客户端风格。

可复用的视觉语言：

| 维度 | SiliconFlow 观察 | Portal/Admin P25 要求 |
|---|---|---|
| 布局 | 约 200px 左侧轻侧栏，主内容从 x=220 左右开始，内容区 12px gap | Portal/Admin shell 使用统一轻色侧栏，不继续 Portal 深色侧栏与 Admin 蓝色侧栏分裂 |
| 品牌主色 | 紫色主色约 `#6c28f6`/`#6e29f6`，active 背景约 `rgba(110, 41, 246, 0.1)` | shared theme token 使用紫色 primary，active/hover/focus 全部从 token 派生 |
| 背景 | 页面整体白底，内容卡片为 `#f8fafc`，边框很轻 | 减少厚重 panel shadow，控制台页面以白底、slate 文本和浅灰卡片为主 |
| 圆角与边框 | 侧栏 active/card/button 多为 8px，输入框约 6px，标签约 4px | 统一 8px card/button radius、6px input radius、4px tag radius |
| 字体 | system UI，导航 14px，正文 12-14px，标题克制 | 去掉过大的页面标题和营销化 hero，采用密集、可扫描的运营控制台层级 |
| 卡片 | 模型卡片约 381x162，padding 16，grid gap 12，无重阴影，hover 紫色边框 | Portal 模型、Admin 渠道/模型/账户摘要使用 compact card/table，不做大卡片堆叠 |
| 标签 | 12px 文本、22px 高，蓝/紫浅底，信息密集 | 模型能力、状态、权限、scope 统一用 compact tags |
| 筛选 | 40px 高搜索/筛选控件，12px 水平间距 | `FilterBar`、search、select、date range 使用 40px 控件和紧凑行高 |
| 辅助入口 | 文档/工单/商务合作放侧栏底部，账号入口右上 | Portal/Admin 可保留帮助/文档入口，但不复制活动与商业文案 |

当前实现差距：

- `web/apps/portal/src/styles.css` 使用深色 `#152033` 侧栏和绿色 `#46c2a2` brand mark，与参考页轻量白底/紫色主色不一致。
- `web/apps/admin/src/styles.css` 使用 260px rail、蓝色 `#246bfe` brand mark 和较重 panel shadow，与参考页 200px 轻侧栏、紫色 active 和低阴影不一致。
- 两端 shell、nav、button、tag、panel、card 和 filter 控件未共享统一 token，后续很容易继续分裂。

P25 的实现边界是建立 shared console theme tokens，并在真实数据连通过程中把生产页面切换到该风格。视觉对齐应优先覆盖 shell、sidebar、topbar、filter、table/card、tag、button、modal/drawer 和 toast，不要求一次性重写所有边缘页面。

## 数据与安全约束

- 前端类型以 generated client schema 为源头。局部 `types.ts` 可以做 alias 或 view model，但不能重新定义与 OpenAPI 漂移的 DTO。
- Admin mutation 必须经过 shared request wrapper 或 feature API 的等价封装，确保 CSRF、`X-Reason`、`Idempotency-Key` 和审计上下文存在。
- Read-only 请求可以直接使用 generated client，但错误格式要统一落到页面可读的 error state。
- 一次性明文 secret 只允许在 create/rotate 成功响应后短暂展示，不进入列表状态、localStorage、日志或导出。
- 所有导出、import preview、playground payload 和 safe metadata 必须继续遵守 P24 denylist。
- RBAC 先以按钮/入口禁用与后端 403 双保险实现，不在 P25 新增前端权限模型。

## 路由设计

Admin 和 Portal 都采用轻量 browser history helper，避免为 P25 引入完整 routing 迁移：

1. 启动时从 `window.location.pathname` 匹配 route metadata。
2. 用户点击导航时调用 `history.pushState`，同时更新 active route。
3. 监听 `popstate`，根据当前 pathname 恢复 active route。
4. 未匹配路径回退到默认首页，并保留明确 fallback。
5. 仅在应用内部维护 route id，不在业务组件中散落 pathname 字符串。

## 状态模型

每个 feature panel 管理自己的 query state：

- `filters`：搜索、状态、类型、时间范围等。
- `pagination`：cursor、limit、hasNext、nextCursor。
- `data`：BFF 返回的 safe DTO 或 view model。
- `loading`：初始加载、刷新、mutation 分开表达。
- `error`：可读错误信息，保留 request_id 时显示复制入口。

跨 feature 共享的状态只保留 session、active route、toast 和必要的全局 reload signal。

## 验收标准

- Admin production panel 不再引用 `sampleChannels`、`sampleModels`、`sampleAccounts`、`sampleAPIKeys`、`sampleUsageLogs`、`sampleTaskLogs`、`sampleReport`、`sampleExport` 或 `sampleResult`。
- Admin 渠道、模型、客户账户、API keys、活动日志、额度运营和操练场可以在本地 console smoke 中看到 BFF 返回的数据。
- Admin create/update/disable/rotate/test/adjust/export/import/run 等 mutation 全部带 CSRF、reason 或 idempotency 约束，并有后端 focused tests 或 smoke 证据。
- Portal 和 Admin 刷新页面、直接访问子 route、浏览器 back/forward 均能恢复正确 view。
- Portal `App.tsx` 不再承担全部 feature data loading 和业务状态，新增 feature hooks 有 focused tests。
- `web/packages/ui` 的新增组件被至少两个 feature 使用，且不引入未使用的大型抽象。
- Portal/Admin 使用同一套 SiliconFlow-like console theme tokens：轻色 200px 侧栏、紫色 primary、8px card/button radius、低阴影、compact card/table/tag/filter 视觉密度；不包含 SiliconFlow logo、品牌资产或活动文案。
- `make p24-cut-scope-check`、`make boundary-check`、`make api-check`、前端 lint/typecheck/test/build、focused Go tests 和 console smoke 通过。

## 风险与处理

| 风险 | 影响 | 处理 |
|---|---|---|
| BFF 返回 fixture 或空数据导致 UI 看起来像未连通 | smoke 不能证明真实链路 | 在 smoke 中检查 network/BFF response，并在空态中显示来源明确的 empty state |
| feature API helper 绕过 shared mutation wrapper | CSRF/reason/idempotency 不一致 | 先完成 mutation client contract，再迁移各面板 |
| Portal 拆分引入行为回归 | 登录、刷新、任务/账本页受影响 | 先加 route/session focused tests，再逐 feature 拆分 |
| UI primitives 过度抽象 | P25 周期膨胀 | 只抽取被两个以上页面复用的交互，未复用逻辑先留在 feature 内 |
| 视觉对齐变成品牌复制 | 法务和产品定位风险 | 只复用布局密度、token 和交互模式，不复制 SiliconFlow logo、文案、活动 banner 或模型描述 |
| 后端分页能力不完整 | 列表体验不一致 | 优先使用现有 cursor/limit；缺失处用明确的 limit/empty state，必要时只补最小 BFF 字段 |
