# P6 Provider 可靠性治理

## 阶段目标

P6 的目标是把 provider/channel 从“可路由、可 fallback”推进到“故障可隔离、恢复可控、重试有预算”。本阶段聚焦健康信号、熔断、重试预算、fallback 限制和故障演练，不扩大 portal、Realtime、semantic routing/cache 或多地域能力。

## 交付物

- Provider/channel 健康信号模型：统一记录成功率、错误率、429/5xx、超时、延迟、stream 中断和手动禁用状态。
- Circuit breaker：支持 closed、open、half_open 状态，按 provider/channel/model capability 维度熔断和恢复。
- Retry budget：限制同一请求、同一租户和同一 provider 的重试次数、重试时间和可重试错误类型。
- Fallback 规则：明确哪些错误可换 channel/provider，哪些错误必须直接返回，stream 已输出后不得透明 fallback。
- Failure drills：覆盖上游 429/5xx、超时、慢响应、坏 JSON、stream 中断、熔断恢复和手动 emergency disable。

## 核心实现顺序

1. 梳理现有 RouteSignals、provider attempt、emergency disable 和 metrics 字段，定义缺失的健康输入和状态输出。
2. 引入 provider/channel 熔断状态机，并确保 router 在 route selection 前读取状态。
3. 在 dispatcher 层加入 retry budget 和 retry eligibility，避免 provider adapter 自行决定重试。
4. 固化 fallback 边界：请求体可重放、未向下游输出、错误类型可重试且预算未耗尽时才允许 fallback。
5. 把熔断、半开探测、恢复和手动禁用事件写入结构化日志、metrics 和 provider attempt 记录。
6. 增加 failure drills 和 focused tests，覆盖多副本共享状态和 Redis 不可用时的安全退化。

## 关键设计约束

- Provider 可靠性治理不能绕过 route policy、model ACL、limit、billing hold 或 snapshot pinning。
- Stream 已向客户端输出 token 后不能切换 provider；只能按 stream accounting 和 billability policy 收尾。
- Retry/fallback 必须有 request 级可追踪记录，不能造成重复扣费、重复 task 或重复 callback。
- Redis 信号缺失时允许安全退回 priority/weighted，但不能把已 emergency disabled 的 channel 重新选中。
- 本阶段不实现 semantic routing/cache、多地域 active-active 或完整生产 Observability 平台。

## 验收标准

- provider/channel 熔断状态可通过测试稳定进入 open、half_open、closed。
- 可重试和不可重试错误分类清晰，429、5xx、timeout、401/403、schema error 和 policy_denied 行为有测试。
- fallback 后的 provider attempt 链路包含原 channel、目标 channel、错误类别、预算消耗和最终结果。
- stream 部分输出后发生错误不会透明 fallback，settlement/finalizer 行为可验证。
- failure drills 能复现上游故障、熔断、恢复和 emergency disable 的组合场景。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 熔断过于激进导致可用 provider 被排除 | 使用窗口阈值、最小样本量和半开探测 |
| 重试导致成本和延迟失控 | 引入 retry budget，并把预算消耗写入 attempt |
| Fallback 造成重复扣费 | fallback 只发生在 settlement 前，request_id 和 attempt_id 保持幂等 |
| Redis 不可用导致状态漂移 | 明确本地短期状态和 snapshot route policy 的 fallback 优先级 |

## 设计来源

- [路线图](./00-roadmap.md)
- [P5 Provider 协议兼容](./16-p5-provider-protocol-compatibility.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [任务清单](../tasks.md)
