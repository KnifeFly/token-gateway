# P3 Realtime Full

## 阶段目标

在 P2 明确 Realtime 只保留 disabled contract 和扩展边界之后，单独规划完整 Realtime 能力。P3 目标是把双向低延迟会话、provider realtime adapter、session memory、音视频流和 realtime billing 做成可运营能力，而不是混入普通 HTTP data plane。

## 交付物

- WebSocket 和 WebRTC 双入口，统一绑定同一个 Realtime session lifecycle。
- Realtime provider adapter，支持上游会话创建、双向事件转发、错误映射和关闭清理。
- Session memory 和会话级上下文策略，明确租户、项目、模型、费用和过期边界。
- 双向音频、视频和文本事件协议，支持 backpressure、心跳、断线和重连策略。
- Realtime billing，在会话关闭、异常断开和 provider usage 回传时完成可审计结算。
- Realtime observability，覆盖 session count、duration、event rate、provider latency、close reason 和 billing repair。

## 核心实现顺序

1. 扩展 RealtimeEngine interface，区分 session create、event relay、close 和 usage finalize。
2. 实现 WebSocket 会话通道，并保持 disabled contract 的兼容错误格式。
3. 增加 provider realtime adapter 抽象和至少一个 mock adapter。
4. 增加 session memory store、TTL、租户隔离和恢复策略。
5. 引入音频/视频事件 schema、backpressure 和最大会话资源限制。
6. 接入 realtime billing hold、usage aggregation、close-time settlement 和 failed repair。
7. 增加 realtime metrics、trace、audit 和 failure drills。
8. 更新 OpenAPI/AsyncAPI 或补充协议文档，明确客户端事件与服务端事件合同。

## 关键设计约束

- P3 不能破坏 P2 的 disabled contract；未启用时仍返回 501/feature_not_enabled。
- Realtime 不能绕过 API key auth、model ACL、billing hold、limits 和 audit。
- Provider adapter 只做 realtime 协议转换，不拥有租户、账务或路由策略。
- Stream close、client disconnect 和 provider close 都必须有明确 billability policy。
- Session memory 默认按租户和项目隔离，不把原始音视频内容写入普通日志或 metrics label。

## 验收标准

- Realtime session 能创建、连接、收发事件、关闭并记录完整审计。
- 至少一个 mock provider realtime adapter 通过端到端测试。
- 断线、provider error、超时和会话过期均有稳定错误码和清理行为。
- 会话费用可预占、结算、失败修复和报表查询。
- metrics/tracing 能回答 session 时长、provider 选择、关闭原因和 usage 来源。
- 未启用配置下的 session API 和 WebSocket 仍稳定返回 P2 约定的 disabled contract。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 双向流状态污染普通 HTTP 链路 | Realtime 独立 engine 和 adapter，复用 auth/policy/billing 接口而不是复用 HTTP dispatch |
| 断线后费用不可解释 | 所有 close reason 显式进入 usage aggregation 和 settlement planner |
| 音视频内容泄露到观测系统 | metrics/log/trace 只记录元数据和安全摘要 |
| provider realtime 语义差异大 | adapter 层统一内部事件，不让 provider-specific event 泄漏到核心会话状态 |

## 设计来源

- [M8 Realtime 协议预留](./09-m8-realtime-reserved.md)
- [P2 架构一致性与高级能力](./13-p2-architecture-advanced.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单](../tasks.md)
