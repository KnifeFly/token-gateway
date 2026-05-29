# P2 架构一致性与高级能力

## 阶段目标

在 P0 生产闭环和 P1 设计能力稳定后，补齐完整架构蓝图中的独立配置面、插件能力、协议分类器增强和 Realtime 后续边界。P2 追求设计一致性和长期演进能力，不把 Realtime 误标为当前已完成能力。

## 交付物

- 独立 `cmd/configd`，承担 config plane 的 snapshot build、validate、publish、watch 和诊断职责。
- Gateway 只消费 active runtime snapshot、Redis hot data 和 provider health 信号，不直接依赖控制面管理表。
- 插件补齐 IP allowlist、Model ACL、RouteOverride 和 Callback plugin。
- CostGuard 能产生明确的 policy 或 route decision，而不是只记录建议。
- 请求分类器支持 model registry hint、body schema inference 和 `ambiguous_protocol`。
- Realtime 完整实现保持独立后续阶段，当前只维护 disabled contract、session 预留和 WebSocket stub 边界。

## 核心实现顺序

1. 梳理现有 snapshot build、validate、publish 和 watch 逻辑，明确哪些职责迁移到 `cmd/configd`。
2. 实现 `cmd/configd` 启动入口，接入配置、DB、Redis、logger、metrics 和 tracing。
3. 让 configd 负责 snapshot 发布和变更通知，gateway 负责 watch、load、validate runtime index 和暴露 staleness。
4. 为 configd 增加启动自检、发布失败处理、回滚入口和 metrics。
5. 补齐 IP allowlist、Model ACL、RouteOverride 和 Callback plugin，并接入现有 plugin phase。
6. 将 CostGuard 的输出升级为可被 policy 或 routing 消费的 decision。
7. 增强 classifier，按 header、endpoint、model registry hint 和 body schema inference 依次消歧。
8. 对 native compatible API 与 unified API 重叠且无法判断的请求返回 `ambiguous_protocol`。
9. 保持 Realtime disabled contract，补充测试确认未启用时返回标准 501/feature_not_enabled。
10. 为未来完整 Realtime 另建阶段入口，范围包括 WebSocket/WebRTC、session memory、双向音视频、provider realtime adapter 和 realtime billing。

## 关键设计约束

- Configd 是配置面服务，不处理客户请求和 provider relay。
- Gateway 热路径不能因为 configd 独立化而回退到控制面表查询。
- Snapshot 发布必须可验证、可回滚，并暴露版本、发布时间和 staleness。
- 插件仍是配置驱动的内置能力，不引入动态代码执行、WASM 或插件市场。
- RouteOverride plugin 只能输出受约束的 route decision，不能绕过 policy、billing 或 model ACL。
- Classifier 不能靠单一 endpoint 猜测协议；无法确定时必须显式返回 `ambiguous_protocol`。
- Realtime 在 P2 仍不声明完整可用，完整能力必须另行规划和验收。

## 验收标准

- `cmd/configd` 可独立启动，能 build、validate、publish runtime snapshot。
- Gateway 能通过 configd 发布的 snapshot 热更新，并在 configd 不可用时按 stale policy 行为运行。
- Snapshot publish、watch、load、rollback 和 staleness metrics 有测试或 smoke 覆盖。
- IP allowlist、Model ACL、RouteOverride 和 Callback plugin 有 phase 顺序、阻断和副作用测试。
- CostGuard 能触发明确 deny、degrade 或 route decision，并被主链路消费。
- Classifier 测试覆盖 header override、endpoint inference、model registry hint、body schema inference 和 `ambiguous_protocol`。
- Realtime 未启用时 session API 和 WebSocket stub 稳定返回标准错误，不进入半实现状态。
- 文档明确完整 Realtime 不属于 P2 的完成定义，只保留后续阶段入口。

## 风险与处理

| 风险 | 处理 |
|---|---|
| configd 独立化引入新单点 | Gateway 保留最近可用 snapshot，并通过 stale policy 控制风险 |
| snapshot 发布失败导致配置漂移 | 发布前 validate，失败不切换 active version，并记录审计和 metrics |
| 插件能力绕过核心治理 | 插件输出统一 decision，由 policy/routing 主链路消费 |
| classifier 误判协议 | 增加 registry hint 和 schema inference，无法确定时返回 `ambiguous_protocol` |
| Realtime 范围失控 | P2 只维护预留和 disabled contract，完整 Realtime 单独立项 |

## 设计来源

- [路线图](./00-roadmap.md)
- [M5 控制面与 Runtime Snapshot](./06-m5-control-plane-snapshot.md)
- [M6 插件与安全治理](./07-m6-plugins-security.md)
- [M8 Realtime 协议预留](./09-m8-realtime-reserved.md)
- [实施计划](../design/ai_gateway_implementation_plan.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
- [ADR](../design/ai_gateway_ADR.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单](../tasks.md)
