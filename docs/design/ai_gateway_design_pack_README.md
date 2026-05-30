# 商用 AI Gateway 最终设计包

本设计包综合 `v0.2` 的完整商业化设计、`v0.3` 的评审修订和 P5-P9 剩余能力与验收路线，作为后续实现、计划和任务拆分的唯一设计来源。

## 文档清单

| 文件 | 用途 |
|---|---|
| `ai_gateway_system_design.md` | 产品定位、协议策略、请求生命周期、商业边界、可观测性、性能和灾备要求 |
| `ai_gateway_architecture_design.md` | 四平面架构、Go 分层、package 职责、GatewayEngine、插件、路由、限流、账务、任务和 snapshot 架构 |
| `ai_gateway_code_blueprint.md` | Go module、最小骨架、核心接口、结构体字段、数据库优先级和测试文件要求 |
| `ai_gateway_implementation_plan.md` | M0-M9 阶段计划、交付物、验收标准和每阶段 Definition of Done |
| `ai_gateway_ADR.md` | 架构决策记录模板和首批已接受 ADR |
| `ai_gateway_openapi.yaml` | 对外 OpenAPI 合同，可导入 Apifox |
| `../tasks.md` | 唯一任务看板和执行状态入口 |

## 合并原则

- 以 v0.3 的协议消歧、snapshot pinning、限流一致性、降级策略、异步幂等、ADR、性能预算和灾备要求为最终架构基线。
- 保留 v0.2 的商用底线：账务必须有预占、结算、ledger、失败修复和对账；数据面热路径不得查询控制面管理表；provider adapter 不拥有路由、策略、计费或租户决策。
- `docs/tasks.md` 是唯一任务看板，不在 `docs/design` 下维护第二份 task list。
- `docs/plan/` 是阶段执行摘要；设计真相以本目录最终版文档为准。

## 落地顺序

```text
M0 基础工程与文档归一
M1 最小非流式数据面
M2 账务闭环
M3 Streaming + Native Compatible
M4 Unified Media Async Task
M5 Control Plane + Snapshot
M6 Plugins + Security
M7 Observability + Performance
M8 Realtime Reserved Extension
M9 Commercial Operations
P5 Provider Protocol Compatibility
P6 Provider Reliability
P7 Media Forwarding Providers
P8 Portal API
P9 Customer Acceptance Closure
```

第一版不要急着铺满所有 provider 和高级商业功能。先把请求生命周期、账务闭环、幂等、失败补偿、snapshot、限流和基础可观测性写稳。

## 当前范围边界

- 当前不做：控制面 RBAC/审计平台、复杂财务/发票闭环、对象存储、完整 Realtime、生产级 Observability 扩展、WASM/动态插件。
- 当前先不做：semantic routing/cache、多地域 active-active。
- 文件能力按非存储输入资产处理，只用于请求归一化、幂等校验和 provider 转发，不承诺持久化、下载、生命周期或存储 SLA。
- Portal 第一版只做 API：模型/Schema、credits、usage、API key 自助管理和 task 查询；不做 UI，也不暴露 admin/control 配置能力。
- P9 只做客户接入验收收口：Portal smoke、OpenAPI import preflight 和 RC smoke 集成，不扩大产品边界。
