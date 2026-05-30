# 商用 AI Gateway 路线图

## 阶段目标

把最终设计包整理成可执行路线图。推进顺序遵循“先数据面、后控制面；先账务闭环、后商业扩展；先内置插件、后动态扩展；Realtime 先预留、后完整实现”的原则。

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
| v1.3 architecture advanced | P2 | 补齐独立 configd、插件能力、分类器增强和完整 Realtime 后续边界 |
| v1.4 realtime full | P3 | 单独实现完整 Realtime 会话、双向音视频、provider adapter 和 realtime billing |

## 交付物

- `docs/plan/01-m0-foundation.md` 到 `docs/plan/10-m9-commercial-ops.md` 作为阶段规划。
- `docs/plan/11-p0-production-closure.md` 到 `docs/plan/13-p2-architecture-advanced.md` 作为设计差距补齐规划。
- `docs/plan/14-p3-realtime-full.md` 作为完整 Realtime 的后续独立阶段入口。
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
13. P2 补齐架构一致性和高级能力：独立 configd、剩余插件、协议分类器增强和完整 Realtime 后续阶段。
14. P3 单独实现完整 Realtime：WebSocket/WebRTC、session memory、双向音视频、provider realtime adapter 和 realtime billing。

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
- P0-P2 是 M0-M9 之后的设计差距补齐阶段，不重新定义已完成阶段的历史范围。
- P0 必须先补齐生产运行闭环；P1 才扩展设计能力；P2 再追齐完整架构蓝图和非 MVP 能力。
- 完整 Realtime 不属于 P2，必须在 P3 单独验收。

## 验收标准

- 每个阶段都有明确目标、交付物、实现顺序、设计约束、验收标准和风险处理。
- `docs/tasks.md` 能直接指导第一轮 P0 开发。
- M0-M9 的先后关系与最终设计包一致。
- P0-P2 能直接指导设计差距补齐开发，且每个阶段都有可验证的完成标准。
- 所有 public API 变更都回到 OpenAPI 合同维护。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 一开始铺太多空包 | 每阶段只创建当前需要的目录和包 |
| 控制面提前膨胀 | M0-M2 优先数据面和账务闭环 |
| 计费后补导致返工 | M2 前不扩大商业售卖范围 |
| 插件做成动态执行平台 | M6 前只做配置驱动的内置插件 |
| Realtime 过早变成完整实现 | M8 只预留 session 和 stub，未启用时返回 501 |
| 设计和实现漂移 | 阶段完成时同步 OpenAPI、计划和设计差异说明 |
| M0-M9 完成后误判为完整设计完成 | P0-P2 单独记录设计差距补齐范围和验收标准 |
| 后续补齐阶段一次性过大 | 按 P0 生产闭环、P1 设计能力、P2 架构一致性拆分执行 |
| 文档声明能力超过实现 | 每个 P 阶段完成时回查 OpenAPI、任务看板、测试和运行文档 |

## 设计来源

- [设计包索引](../design/ai_gateway_design_pack_README.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
- [实施计划](../design/ai_gateway_implementation_plan.md)
- [任务清单](../tasks.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
