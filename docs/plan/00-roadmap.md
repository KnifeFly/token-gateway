# 商用 AI Gateway 路线图

## 阶段目标

把最终设计包整理成可执行路线图。推进顺序遵循“先数据面、后控制面；先账务闭环、后商业扩展；先内置插件、后动态扩展；Realtime 只预留 disabled contract，不进入当前完整实现路线”的原则。P5 之后只推进当前明确要做的剩余能力，并把不做或先不做的能力从路线中剥离。P12-P14 已根据 2026-05-31 review 结果作为第一轮可靠性收敛阶段；P15-P18 根据后续 review 暴露的生产生命周期、身份边界、worker/callback 和文件资产边界问题作为第二轮收敛阶段。P19-P22 在核心网关和可靠性收敛完成后，解除“只做后端 API”的阶段性限制，新增完整 Portal/Admin 前后端、browser BFF、frontend monorepo、RBAC、audit 和 console 生产化路线。P23 单独承载 Console 目录结构对齐和模块拆分，避免把结构治理混进 P22 的生产化前端任务。P24 承载 NewAPI 对标后的精简 Console 产品能力，但明确裁掉用户/模型/渠道分组、倍率、订阅、兑换码、支付、模型部署和复杂系统设置。

## 版本节奏

| 版本 | 阶段 | 目标 |
|---|---|---|
| v0.1 internal alpha | M0 + M1 | 基础工程和最小数据面可运行 |
| v0.2 billing alpha | M2 | 完成余额预占、结算、ledger 和失败修复 |
| v0.3 protocol beta | M3 | 支持 OpenAI、Claude、Gemini 主协议和 stream |
| v0.4 media beta | M4 | 支持统一媒体任务、非存储输入资产、回调和 provider polling |
| v0.5 control plane beta | M5 | 支持控制面配置和 runtime snapshot 发布 |
| v0.6 security beta | M6 | 支持插件化安全、审计和脱敏 |
| v0.7 production candidate | M7 | 完成生产观测、压测和故障演练 |
| v0.8 realtime reserved | M8 | 预留 Realtime session 和 WebSocket 架构 |
| v1.0 commercial launch | M9 | 支持正式商业运营、对账、报表和模型市场 |
| v1.1 production closure | P0 | 补齐 worker、异步任务、公开 API、snapshot 安全和紧急禁用生产闭环 |
| v1.2 design capabilities | P1 | 补齐多维限流、策略路由、显式 policy 和模型目录能力 |
| v1.3 architecture advanced | P2 | 补齐独立 configd、插件能力、分类器增强，并固化 Realtime disabled contract 边界 |
| v1.4 production hardening | P3 | 补齐限流算法语义、账务计费策略、native media、真实路由信号和生产集成验收 |
| v1.5 release candidate | P4 | 完成干净依赖环境、真实 provider、后台 job、OpenAPI 合同、运维演练和灰度上线验收 |
| v1.6 provider protocol compatibility | P5 | 补齐 OpenAI、Claude、Gemini SDK 兼容、stream、tool calling、usage/error 映射和 contract tests |
| v1.7 provider reliability | P6 | 补齐 provider/channel 健康探测、熔断、retry budget、fallback 限制和 failure drills |
| v1.8 media forwarding providers | P7 | 固化非存储媒体输入语义，扩展真实媒体 provider submit/poll/cancel/result URL 映射 |
| v1.9 portal API | P8 | 提供开发者自助 portal API，覆盖模型、schema、credits、usage、API key 和 task 查询 |
| v1.10 customer acceptance | P9 | 收口客户接入验收，补齐 Portal smoke、OpenAPI import preflight 和 RC 验收证据 |
| v1.11 release handoff | P10 | 收口发布交接，补齐 release handoff、PR 模板和发布证据清单 |
| v1.12 model pricing catalog | P11 | 建立模型分类、复杂价格体系、渠道成本和模型目录运营能力 |
| v1.13 review p0 correctness | P12 | 按 review P0 修复 stream lease、async idempotency、terminal task settlement 和 zero-price/no-hold 账务正确性 |
| v1.14 review p1 commercial hardening | P13 | 按 review P1 补齐 async price pin、预算语义、egress 安全、classifier 顺序和 async fallback |
| v1.15 review p2 engineering readiness | P14 | 按 review P2 补齐 API key HMAC、管理面安全基线、stream close 稳定性、SSE 解析、trusted proxy 和 README |
| v1.16 review follow-up p0 production blockers | P15 | 收敛 internal/client request ID、stream lease renewal、task-aware hold reaper、gateway callback contract 和 egress fail-closed |
| v1.17 review follow-up accounting audit | P16 | 收敛 async idempotency race、async attempt audit、final failed attempt、failed settlement claim 和 budget 语义 |
| v1.18 review follow-up worker callback stability | P17 | 收敛 worker lease heartbeat、per-job concurrency、poller 错误隔离、callback durable claim 和 delivery 稳定性 |
| v1.19 review follow-up file asset boundary | P18 | 收敛 transient input asset metadata registry、expired quota、cleanup job、stream disabled contract 和文件能力文档边界 |
| v1.20 console monorepo foundation | P19 | 建立 `cmd/console`、OpenAPI 拆分、frontend workspace、generated client、CI 和本地开发基线 |
| v1.21 portal web bff | P20 | 建立 Portal Web BFF、Portal session、Portal UI、dashboard/onboarding 和 Portal smoke |
| v1.22 admin web bff | P21 | 建立 Admin Web BFF、operator session、RBAC、audit、safe read/write workflow 和 operations views |
| v1.23 console frontend production | P22 | 完成 Admin UI、Portal/Admin 前端收口、静态资源部署、安全头、E2E smoke 和 console 发布交接 |
| v1.24 console directory alignment | P23 | 对齐 Portal/Admin Console 目标目录树，rename `internal/controlplane/admin` 到 `internal/controlplane/configadmin`，拆分 handler/service/repository/frontend packages，补齐 api scripts/examples 和 import 边界检查 |
| v1.25 newapi lean console parity | P24 | 基于 NewAPI 参考补齐渠道、模型、用户/客户账户、令牌、日志、任务、操练场和最小额度运营，明确不做分组、倍率、支付和复杂设置 |

## 交付物

- `docs/plan/01-m0-foundation.md` 到 `docs/plan/10-m9-commercial-ops.md` 作为阶段规划。
- `docs/plan/11-p0-production-closure.md` 到 `docs/plan/13-p2-architecture-advanced.md` 作为设计差距补齐规划。
- `docs/plan/14-p3-production-hardening.md` 作为生产语义补齐与商用硬化规划。
- `docs/plan/15-p4-release-candidate-readiness.md` 作为发布候选与商用上线验收规划。
- `docs/plan/16-p5-provider-protocol-compatibility.md` 到 `docs/plan/21-p10-release-handoff.md` 作为剩余产品能力、客户验收和发布交接收口规划。
- `docs/plan/22-p11-model-pricing-catalog.md` 作为模型分类、复杂价格体系、渠道成本和模型目录增强规划。
- `docs/plan/23-p12-review-p0-correctness.md` 到 `docs/plan/25-p14-review-p2-engineering-readiness.md` 作为 2026-05-31 第一轮 review 后的可靠性收敛规划。
- `docs/plan/26-p15-review-followup-p0-production-blockers.md` 到 `docs/plan/29-p18-review-followup-file-asset-boundary.md` 作为后续 review 暴露的生产生命周期、账务审计、worker/callback 和文件资产边界收敛规划。
- `docs/plan/30-p19-console-monorepo-foundation.md` 到 `docs/plan/33-p22-console-frontend-production.md` 作为 Portal/Admin full console、BFF、frontend monorepo 和生产化收敛规划。
- `docs/plan/34-p23-console-directory-structure-alignment.md` 作为 Console 目标目录树对齐、模块拆分和结构治理规划。
- `docs/plan/35-p24-newapi-lean-console-parity.md` 作为 NewAPI 精简对标、渠道/模型/用户/令牌/日志/操练场补齐和裁剪边界规划。
- `docs/tasks.md` 作为任务看板和执行入口。
- 阶段文档只沉淀执行化摘要，设计真相以 `docs/design` 中不带版本号的最终版为准。

## 核心实现顺序

1. M0 建立 Go 工程、配置、错误、日志、HTTP server、DB/Redis、metrics、tracing、migration 和 CI。
2. M1 跑通 `/v1/chat/completions` 的认证、解析、路由、provider relay 和基础观测。
3. M2 建立 balance hold、usage attempt、usage record、ledger、settlement 和 failed replay 闭环。
4. M3 扩展 OpenAI Responses、Embeddings、Claude Messages、Gemini GenerateContent 和 stream accounting。
5. M4 扩展统一媒体任务、非存储输入资产、异步 provider task、polling/webhook 和 callback。
6. M5 建立控制面配置、snapshot build/validate/publish/watch 和热加载。
7. M6 建立插件链、安全治理、PII 脱敏、PromptGuard、ResponseGuard 和 audit。
8. M7 补齐 production metrics、tracing、dashboard、alert、load test 和 failure drills。
9. M8 预留 Realtime session、RealtimeEngine interface 和 WebSocket handler stub。
10. M9 补齐 usage/cost/profit report、账单导出、对账、灾备演练和运营后台能力。
11. P0 补齐生产闭环缺口：worker 进程、真实异步 provider task、公开 API、snapshot stale policy 和 emergency disable。
12. P1 补齐设计能力：多维限流/预算、策略路由、显式 policy stage 和 model catalog/schema/alias。
13. P2 补齐架构一致性和高级能力：独立 configd、剩余插件、协议分类器增强，并固化 Realtime disabled contract。
14. P3 补齐生产语义和商用硬化：限流算法语义、账务计费策略、native media、真实路由信号、configd 分发验证和生产集成验收。
15. P4 完成发布候选验收：干净 compose/staging、真实 provider、后台运营 job、OpenAPI 管理面、SLO/告警和灰度回滚演练。
16. P5 补齐 provider 协议兼容：SDK wire shape、stream、tool calling、多模态、usage/error 映射和合同测试。
17. P6 补齐 provider 可靠性治理：健康信号、熔断、retry budget、fallback 限制、半开恢复和故障演练。
18. P7 补齐非存储媒体转发生态：输入资产只做转发归一化，扩展真实媒体 provider task 生命周期映射。
19. P8 补齐 portal API：模型、schema、credits、usage、API key 自助管理和 task 查询。
20. P9 收口客户接入验收：Portal smoke、OpenAPI import preflight、RC smoke 集成和客户运行手册。
21. P10 收口发布交接：release handoff 文档生成、PR 模板、发布字段和回滚证据清单。
22. P11 补齐模型分类、分类价格模板、组件化客户售价、组件化渠道成本、模型目录展示字段和渠道测试/同步 preview。
23. P12 收敛 review P0 正确性：stream concurrency lease、async idempotency hold、terminal task settlement、zero-price/no-hold settlement 和真实依赖回归测试。
24. P13 收敛 review P1 商业账务与安全边界：async price pin、rate/spend budget 拆分、文件/媒体输入资产语义、egressguard、classifier 顺序和 async fallback。
25. P14 收敛 review P2 工程交付与安全基线：API key HMAC、管理面静态 token 安全基线、stream settlement timeout、SSE parser、nil metrics、trusted proxy 和 README。
26. P15 收敛 follow-up review 生产阻塞：internal/client request ID 分离、stream Redis lease renewal、task-aware hold reaper、provider webhook 不直达 customer callback 和 egress fail-closed。
27. P16 收敛 follow-up review 账务审计一致性：async idempotency 并发 replay、async submit attempt durable audit、final failed attempt durable 标记、failed settlement row claim 和 budget 语义。
28. P17 收敛 follow-up review worker/callback 稳定性：worker lease heartbeat、job concurrency、poller 单任务错误隔离、callback durable claim、response body drain 和 failure drills。
29. P18 收敛 follow-up review 文件资产边界：坚持 transient metadata registry，修正 expired quota、cleanup job、stream disabled contract 和文件能力文档边界。
30. P19 建立 console monorepo foundation：`cmd/console`、`api/openapi/*`、`web/*` workspace、generated client、CI 和本地开发代理。
31. P20 建立 Portal Web BFF：迁移 `internal/app/portal`、新增 `/api/portal/v1/*`、Portal session、Portal UI 和 Portal smoke。
32. P21 建立 Admin Web BFF：新增 `internal/app/admin`、operator session、RBAC、audit、Admin read model 和 owner service 写 workflow。
33. P22 完成 console frontend production：Admin UI、Portal/Admin UI 收口、static asset strategy、安全头、E2E smoke、deployment/rollback runbook。
34. P23 完成 console directory alignment：rename control config owner 为 `internal/controlplane/configadmin`，按目标树拆分 API scripts/examples、transport handler、app service/repository、frontend apps/packages，并保持行为不变。
35. P24 完成 NewAPI lean console parity：补齐渠道、模型、用户/客户账户、令牌、日志、任务、操练场和最小额度运营，明确禁止分组、倍率、支付、订阅、兑换码、模型部署和复杂系统设置进入本阶段。

## 关键设计约束

- 对外 URI 表达能力，不表达供应商。
- `model` 是路由入口，必须映射到模型、能力、schema、provider mapping、route policy、price rule、limit rule 和 plugin binding。
- 数据面热路径只读 runtime snapshot 和索引，不实时查控制面管理表。
- Provider adapter 只做协议翻译、上游调用、错误映射、usage 解析和任务提交/轮询。
- Native Compatible API 与 Unified Media API 共享 URI 时必须通过 `X-Gateway-Protocol`、model registry 和 request schema 消歧，无法判断返回 `ambiguous_protocol`。
- 计费必须有预占、结算、ledger、失败修复和对账。
- 流式响应必须在 stream close 时做最终 accounting。
- 异步媒体任务和扣费型写操作必须支持 `Idempotency-Key`，避免重复任务和重复扣费。
- API key、provider key、原始 prompt、原始 response 默认不得进入日志、metrics label 和 trace attribute。
- P0-P4 是 M0-M9 之后的设计差距补齐、商用硬化和发布候选验收阶段，不重新定义已完成阶段的历史范围。
- P0 必须先补齐生产运行闭环；P1 才扩展设计能力；P2 再追齐架构一致性和非 MVP 能力。
- 完整 Realtime 不进入当前路线；M8/P2 只维护 disabled contract、session 预留和 WebSocket stub。
- P3 优先处理已实现能力的生产语义缺口，不新增大范围产品面。
- P4 不再扩大协议面，优先把已有能力放到干净依赖环境和真实上游中验收，并形成可重复 release gate。
- P5-P11 只覆盖当前明确要做的剩余能力、验收、发布交接和模型价格目录增强；复杂财务/发票闭环、对象存储、完整 Realtime、生产级 Observability 扩展、WASM/动态插件不进入当前路线。
- P12-P18 是 review-driven remediation 阶段，用于可靠性收敛；它们不重新定义旧 P0-P2 历史范围，也不引入新的 public API 或大型产品面。
- P12-P14 表示第一轮 review remediation；P15-P18 表示后续 review 暴露的生产 lifecycle、身份边界、worker/callback 和文件资产边界收敛。
- 当 P15 生产阻塞问题未通过验收时，暂停继续 P11 这类功能扩展，先保证账务、限流、stream、异步任务、callback 和 outbound 安全不变量成立。
- P19-P22 是完整 Portal/Admin console 路线；它们新增 browser BFF 和 frontend monorepo，但不改变 `/v1/*`、`/v1/portal/*` 和 `/admin/*` 的既有职责。
- `/api/portal/v1/*` 与 `/api/admin/v1/*` 只能由 `cmd/console` 承载；`/admin/*` 继续是 machine Control API，不作为 Admin SPA 浏览器接口。
- Admin Web 必须使用 operator session、RBAC、CSRF、Idempotency-Key、durable audit 和 redacted safe DTO；不得把 control admin token 放入浏览器。
- Portal Web API key login 只用于交换 HttpOnly session；不得把 customer API key 长期保存到 browser storage。
- Portal/Admin 后端按 `internal/app/portal`、`internal/app/admin` 拆分 service/repository；control/config owner 在 P23 rename 为 `internal/controlplane/configadmin`；不要新增全局 service/repository 目录。
- Frontend monorepo 使用 `web/apps/portal`、`web/apps/admin` 和 `web/packages/*`，API 类型由 OpenAPI 生成或接受 contract check。
- P23 是行为保持型结构治理阶段，不新增产品能力、不改 API 行为、不改变 core domain ownership。`configadmin` 是 rename-only 去歧义，不代表 control owner 合并进 `app/admin`。若 P22 开始前 Admin UI 或 handler/service 单文件继续膨胀，可先执行 P23 的结构拆分子集。
- P24 是 NewAPI 精简对标阶段，只保留 token-gateway 需要的 Console 能力；用户分组、模型分组、渠道分组、倍率、订阅套餐、兑换码、第三方支付配置、邀请返利、模型部署服务和大而全系统设置都不进入 P24。
- 价格展示币种和存储精度分离：展示使用 USD、CNY 等真实币种和人可读单位，存储使用 currency + micros 整数。
- 模型 category 驱动可配置价格单位、默认展示方式和模型目录筛选；category 不等于 provider type、route strategy 或 New API group。
- 客户售价和 provider 成本可以同构，但必须分表、分用途、分权限；settlement 只能使用客户售价。
- Semantic routing/cache 和多地域 active-active 先不做；如重新进入范围，需要另立路线和任务板。
- 文件能力按非存储输入资产处理，gateway 不承诺媒体对象持久化、下载、生命周期或存储 SLA。
- P8 Portal 第一版复用 API key 鉴权，只做客户自助查询和受限 key 管理；P20 Portal Web 只能扩展 browser self-service，不暴露 admin/control 配置能力。
- 客户传入的 `X-Request-ID` 只能作为 client request id；内部账务、limit、attempt、usage、ledger 和 failed settlement 必须使用服务端生成的 internal request id。
- Provider webhook 不能直接暴露 customer callback URL；客户 callback 只能通过 gateway outbox 按 gateway contract 投递。
- Worker lease、stream lease 和 task hold 都必须绑定真实生命周期，不能只依赖短 TTL 或本地内存语义。

## 验收标准

- 每个阶段都有明确目标、交付物、实现顺序、设计约束、验收标准和风险处理。
- `docs/tasks.md` 能直接指导 P5-P24 后续开发、验收、发布交接、模型价格目录增强、review remediation、console full-stack 实现、目录结构治理和 NewAPI 精简对标。
- M0-M9 的先后关系与最终设计包一致。
- P0-P24 能直接指导设计差距补齐、商用硬化、发布候选验收、剩余产品能力建设、客户验收、发布交接、模型价格目录增强、review remediation、Portal/Admin console 建设、目录结构治理和 NewAPI 精简对标，且每个阶段都有可验证的完成标准。
- 所有 public API 变更都回到 OpenAPI 合同维护。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 一开始铺太多空包 | 每阶段只创建当前需要的目录和包 |
| 控制面提前膨胀 | M0-M2 优先数据面和账务闭环 |
| 计费后补导致返工 | M2 前不扩大商业售卖范围 |
| 插件做成动态执行平台 | M6 前只做配置驱动的内置插件 |
| Realtime 重新膨胀成完整实现 | 只保留 disabled contract，完整 Realtime 需要重新产品决策后另立路线 |
| 设计和实现漂移 | 阶段完成时同步 OpenAPI、计划和设计差异说明 |
| M0-M9 完成后误判为完整设计完成 | P0-P3 单独记录设计差距补齐、商用硬化范围和验收标准 |
| 后续补齐阶段一次性过大 | 按 P0 生产闭环、P1 设计能力、P2 架构一致性拆分执行 |
| 文档声明能力超过实现 | 每个 P 阶段完成时回查 OpenAPI、任务看板、测试和运行文档 |
| 商用硬化变成零散修补 | P3 按限流、账务、native media、路由信号、configd 验证和生产验收六条主线推进 |
| 发布候选只停留在本地单进程验证 | P4 必须使用干净 Docker volume 或 staging 依赖运行四进程、迁移、snapshot、worker 和真实 provider 验收 |
| Provider 兼容性被抽象层掩盖 | P5 使用 SDK/HTTP contract tests 固定 wire shape、stream event、usage 和错误映射 |
| Provider 故障扩大成全局不可用 | P6 使用熔断、retry budget、fallback 限制和 emergency disable 组合治理 |
| 媒体能力误变成对象存储产品 | P7 统一声明 transient/non-storage，只透传 provider result URL 和必要 metadata |
| Portal 演化成第二套 admin API | P8 只开放 customer self-service，不提供 channel、route、price、limit、snapshot 或 emergency 配置 |
| 客户验收只停留在人工 curl | P9 固化 Portal smoke CLI、OpenAPI import preflight 和 RC smoke 集成 |
| 发布交接依赖口头同步 | P10 固化 release handoff 工具、PR 模板、验证命令和回滚证据字段 |
| 模型价格体系继续停留在 input/output token | P11 引入 category 驱动的组件化价格模板、客户售价和 provider 成本分离 |
| 第一轮 Review 发现的正确性问题被功能开发淹没 | P12-P14 作为第一轮可靠性收敛阶段，优先级高于当时的功能扩展 |
| Review P0/P1/P2 与既有 P0/P1/P2 阶段混淆 | 使用 P12-P14、P15-P18 等后续编号承载 review remediation，并在标题中保留 review priority 或 follow-up 语义 |
| 后续 review 暴露生命周期与身份边界问题 | 使用 P15-P18 作为第二轮 review follow-up，P15 未验收前暂停继续 P11 功能扩展 |
| 客户 request id 污染账务幂等和限流 | internal/client request id 分离，所有账务和 Redis lease 使用 internal request id |
| 长 stream、长 task、worker job 超过 TTL 后状态失真 | 为 stream lease 和 worker lease 增加 renewal，为 task hold 增加 task-aware reaper |
| Provider webhook 绕过 gateway callback contract | 禁止 provider 直接调用 customer callback URL，统一通过 gateway internal webhook/polling 和 callback outbox |
| 文件 metadata registry 被误解为对象存储 | P18 固化 transient/non-storage 语义，expired quota 与 cleanup job 对齐该边界 |
| Portal/Admin console 继续按单文件 service/repository 扩张 | P19-P22 使用 `internal/app/{portal,admin}/service` 和 `repository` 子目录，按 use case/read model 拆文件 |
| Admin Web 污染 `/admin/*` machine API | Browser Admin 固定走 `/api/admin/v1/*`，`/admin/*` 只给内部 automation/control |
| Customer API key 长期暴露在浏览器 | Portal API key login 只换取 HttpOnly session，禁止 localStorage/sessionStorage 保存 key |
| OpenAPI 与前端类型漂移 | API contract split、generated client 和 CI diff check |
| Console 静态资源影响 gateway 热路径 | `cmd/console` 独立部署，静态资源优先 CDN/Nginx，gateway 不服务 Portal/Admin SPA |
| 目录结构治理变成大范围行为重写 | P23 只做行为保持型拆分，每一步跑 focused tests，API/schema/session/RBAC/audit 语义不变 |
| 对标 NewAPI 变成复制 NewAPI | P24 使用 lean parity 设计，只保留渠道、模型、用户/客户账户、令牌、日志、任务、操练场和最小额度运营，并用任务板禁止分组、倍率、支付和复杂设置进入 |

## 设计来源

- [设计包索引](../design/ai_gateway_design_pack_README.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
- [Portal/Admin Console Monorepo 设计](../design/ai_gateway_console_monorepo_design.md)
- [NewAPI Lean Console Parity 设计](../design/ai_gateway_newapi_lean_console_design.md)
- [实施计划](../design/ai_gateway_implementation_plan.md)
- [任务清单](../tasks.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
