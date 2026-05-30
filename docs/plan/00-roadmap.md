# 商用 AI Gateway 路线图

## 阶段目标

把最终设计包整理成可执行路线图。推进顺序遵循“先数据面、后控制面；先账务闭环、后商业扩展；先内置插件、后动态扩展；Realtime 只预留 disabled contract，不进入当前完整实现路线”的原则。

## 版本节奏

| 版本 | 阶段 | 目标 |
|---|---|---|
| v0.1 internal alpha | M0 + M1 | 基础工程和最小数据面可运行 |
| v0.2 billing alpha | M2 | 完成余额预占、结算、ledger 和失败修复 |
| v0.3 protocol beta | M3 | 支持 OpenAI、Claude、Gemini 主协议和 stream |
| v0.4 media beta | M4 | 支持统一媒体任务、文件、回调和 provider polling |
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

## 交付物

- `docs/plan/01-m0-foundation.md` 到 `docs/plan/10-m9-commercial-ops.md` 作为阶段规划。
- `docs/plan/11-p0-production-closure.md` 到 `docs/plan/13-p2-architecture-advanced.md` 作为设计差距补齐规划。
- `docs/plan/14-p3-production-hardening.md` 作为生产语义补齐与商用硬化规划。
- `docs/plan/15-p4-release-candidate-readiness.md` 作为发布候选与商用上线验收规划。
- `docs/tasks.md` 作为任务看板和执行入口。
- 阶段文档只沉淀执行化摘要，设计真相以 `docs/design` 中不带版本号的最终版为准。

## 核心实现顺序

1. M0 建立 Go 工程、配置、错误、日志、HTTP server、DB/Redis、metrics、tracing、migration 和 CI。
2. M1 跑通 `/v1/chat/completions` 的认证、解析、路由、provider relay 和基础观测。
3. M2 建立 balance hold、usage attempt、usage record、ledger、settlement 和 failed replay 闭环。
4. M3 扩展 OpenAI Responses、Embeddings、Claude Messages、Gemini GenerateContent 和 stream accounting。
5. M4 扩展统一媒体任务、文件资产、异步 provider task、polling/webhook 和 callback。
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

## 验收标准

- 每个阶段都有明确目标、交付物、实现顺序、设计约束、验收标准和风险处理。
- `docs/tasks.md` 能直接指导第一轮 P0 开发。
- M0-M9 的先后关系与最终设计包一致。
- P0-P4 能直接指导设计差距补齐、商用硬化和发布候选验收，且每个阶段都有可验证的完成标准。
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

## 设计来源

- [设计包索引](../design/ai_gateway_design_pack_README.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
- [实施计划](../design/ai_gateway_implementation_plan.md)
- [任务清单](../tasks.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
