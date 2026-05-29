# M8 Realtime 协议预留

## 阶段目标

不在 MVP 中完整实现 Realtime/WebSocket/WebRTC，但预留客户可见 session API、内部接口和观测边界，避免未来接入实时语音、视频或双向流协议时破坏现有认证、计费、审计和路由模型。

## 交付物

- `POST /v1/realtime/sessions` 和 `GET /v1/realtime/sessions/{session_id}` OpenAPI 合同。
- RealtimeSession domain、RealtimeEngine interface 和 WebSocket handler stub。
- 鉴权、审计、metrics 和 trace 接入点。
- 未启用时返回标准 `501` 或明确的 `feature_not_enabled` 错误。

## 核心实现顺序

1. 定义 session request/response、状态和过期时间。
2. 定义 RealtimeEngine interface，不绑定具体 provider。
3. 建立 HTTP handler 和 WebSocket handler stub。
4. 接入 API key auth、audit、metrics 和 trace。
5. 在 OpenAPI、任务看板和 ADR 中明确 Realtime 暂不进入 MVP。

## 关键设计约束

- M8 只做协议和架构预留，不实现完整双向音频、WebRTC、低延迟 token accounting 或 provider realtime adapter。
- Realtime 接口仍必须使用统一 API key、租户、项目、模型权限和审计机制。
- 未启用时必须显式返回标准错误，不允许静默降级到普通 chat。
- Realtime 后续计费必须能接入 balance hold、usage record、ledger 和 failed settlement repair。

## 验收标准

- OpenAPI 包含 Realtime session 接口。
- 未启用时 session 创建返回标准 501/feature_not_enabled。
- 鉴权失败返回 401，权限不足返回 403。
- session 请求有 request_id、trace_id、metrics 和 audit 记录。
- WebSocket stub 可编译，但不承诺生产可用 realtime 能力。

## 风险与处理

| 风险 | 处理 |
|---|---|
| Realtime 预留扩大成完整实现 | 明确 M8 只做 session 和 stub |
| 双向流绕开现有治理 | 复用 auth、policy、audit、metrics 和 future billing 接口 |
| 客户误认为已支持 realtime | 未启用时返回明确 feature_not_enabled |

## 设计来源

- [实施计划 M8](../design/ai_gateway_implementation_plan.md)
- [系统设计 Realtime 预留](../design/ai_gateway_system_design.md)
- [ADR Realtime 决策](../design/ai_gateway_ADR.md)
- [OpenAPI Realtime Session](../design/ai_gateway_openapi.yaml)
- [任务清单 M8](../tasks.md)
