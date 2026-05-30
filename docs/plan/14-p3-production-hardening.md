# P3 生产语义补齐与商用硬化

## 阶段目标

在 M0-M9、P0、P1、P2 已经完成主体实现后，P3 不继续推进完整 Realtime。Realtime 只保留 M8/P2 已有的 disabled contract、session 预留和 WebSocket stub，未启用时稳定返回 501/`feature_not_enabled`。

P3 的目标是把现有商用网关从“主体能力已落地”推进到“生产语义可解释、可验证、可运营”：补齐限流算法语义、账务计费策略、Native Compatible media 兼容、真实路由信号、configd 分发验证和生产级集成验收。

## 交付物

- Redis 限流从当前固定窗口语义补齐到设计要求的 QPS/RPM token bucket、TPM estimated pre-charge、concurrency lease、daily budget 和 cost-per-minute 原子判断。
- Billability policy 明确同步、流式和异步任务在无输出、部分输出、客户端断开、provider 异常、任务取消和任务成功时的计费行为。
- Native Compatible media endpoint 补齐 OpenAI images/audio 的 provider adapter、parser、错误映射和 SDK 兼容测试。
- Unified Media async task 的 provider task adapter 不再只依赖通用 HTTP 假设，明确至少一类真实供应商映射或可扩展 adapter contract。
- RouteSignals 接入真实健康、延迟、成本和 quota 数据源，避免生产策略只依赖静态默认值。
- Configd snapshot publish/watch/rollback/diagnostics 有进程级 smoke 和故障场景验证。
- OpenAPI、runbook、failure drills、load test 和集成测试与当前实现保持一致。

## 核心实现顺序

1. 固化 P3 范围：完整 Realtime 不进入当前路线，任务板只保留 disabled contract 的维护边界。
2. 梳理 RedisEnforcer 的现有固定窗口语义，先补行为测试，再改造 QPS/RPM token bucket 和 TPM 预扣 Lua 逻辑。
3. 为 daily budget、cost-per-minute 和 concurrency lease 增加多规则组合、local deny cache 和 Redis 集成测试。
4. 设计并实现 `BillabilityPolicy`，让 stream finalizer、settlement planner 和 task settlement 共享同一套可审计计费判定。
5. 补齐 Native OpenAI images/audio endpoint 到 provider adapter 的转发、模型改写、usage 解析和错误映射。
6. 抽象 Unified Media provider task adapter contract，为 image/video/audio/music 的 submit、poll、cancel 明确供应商字段映射。
7. 为 routing strategy 接入真实 `RouteSignals` provider，来源包括 provider attempt 统计、成本配置、quota projection 和 emergency disable。
8. 强化 configd 与 gateway 的 snapshot 分发链路，覆盖发布失败、不切 active version、rollback、gateway stale policy 和 configd 重启。
9. 更新 OpenAPI、runbook、ADR 或设计差异说明，确保文档不声明当前未实现能力。
10. 跑通 `go test ./...`、`make lint`、`make build`、Redis integration、compose smoke、关键 curl 和 failure drill。

## 关键设计约束

- 完整 Realtime 不属于 P3；不能新增 WebSocket/WebRTC 会话、provider realtime adapter 或 realtime billing 任务。
- Gateway 热路径仍只读 runtime snapshot、Redis hot data 和内存索引，不回退查询控制面管理表。
- Provider adapter 只负责协议转换、上游调用、错误映射和 usage/task 状态解析，不拥有路由、策略、账务或租户决策。
- Billability policy 必须可测试、可审计，不能在多个 settlement 入口各自硬编码。
- 限流允许 local deny cache 缓存短期拒绝，不允许缓存正向 allow 结果。
- Native compatible endpoint 与 unified media endpoint 共享 URI 时，仍必须保留 header、model registry 和 body schema 消歧。
- OpenAPI 和 runbook 必须反映实际可用能力；未实现能力只能标注为 disabled 或 unsupported。

## 验收标准

- QPS/RPM token bucket、TPM 预扣、concurrency lease、daily budget 和 cost-per-minute 在 Redis Lua 下原子生效，并有单元测试和 `TOKEN_GATEWAY_REDIS_ADDR` 集成测试。
- 流式无有效输出默认不计费，部分输出按策略计费，客户端断开、provider error 和 task cancel 的结算行为都有确定性测试。
- OpenAI SDK 或等价 HTTP 测试能覆盖 native images/audio endpoint 的成功、上游错误和 unsupported model 场景。
- Unified Media provider task adapter 能证明至少一个真实 provider 映射路径，mock 只作为测试辅助。
- health/cost/latency/quota routing strategy 使用真实信号源时有确定性回归测试，信号缺失时有安全 fallback。
- Configd 独立启动、snapshot publish、watch、rollback、diagnostics、gateway stale policy 和 configd failure smoke 可复现。
- `docs/tasks.md`、OpenAPI、runbook 和阶段计划不再把完整 Realtime 列为待开发范围。
- `go test ./...`、`make lint`、`make build`、Redis integration 和 compose smoke 均通过或记录明确外部依赖缺口。

## 2026-05-30 实现记录

- Redis 限流 Lua 已按 bucket/counter/lease 拆分：QPS、RPM、TPM 和 cost-per-minute 使用 token bucket，daily budget 使用固定日计数，concurrency 使用带 TTL 的 lease。
- 统一 `BillabilityPolicy` 已接入同步/流式 settlement planner、task settlement 和 provider task poller，计费原因会写入 settlement plan/ledger reason。
- Native OpenAI images/audio 已在 parser/classifier/openai adapter 层按 `X-Gateway-Protocol`、Content-Type、body schema 做消歧，并支持 JSON 与 multipart 模型改写。
- Unified Media async task 已抽象 `ProviderTaskAdapter` 与 registry，默认通用 HTTP adapter 保留，provider 可注册专属 submit/poll/cancel 映射。
- RouteSignals 已增加 Redis 数据源，路由策略读取 health、latency、cost、quota、disabled、model compatibility，缺失信号按安全默认值回退。
- Configd 分发 smoke 已覆盖 HTTP publish/diagnostics/rollback、data-plane watcher 热加载、rollback 热加载、configd restart 和 gateway hard stale policy。
- OpenAPI 对 images/audio 共享 URI 和 `X-Gateway-Protocol` 消歧的描述与实现一致，本阶段未新增公共 URI；验收命令固化到 [P3 Production Hardening Runbook](../runbook/p3-production-hardening.md)。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 限流算法改造影响现有配置 | 先用兼容字段生成新 Lua 输入，保留旧配置 fallback，并增加迁移测试 |
| 计费策略分散导致账务不可解释 | 引入统一 `BillabilityPolicy`，所有 settlement 入口只消费 policy 结果 |
| Native media 与 Unified media 冲突 | 保持 `X-Gateway-Protocol` 优先级，无法判断时返回 `ambiguous_protocol` |
| 路由信号缺失导致策略误选 | 信号源缺失时退回 priority/weighted，并暴露 metric/alert |
| Configd 独立化引入发布不可用 | Gateway 保留最近可用 snapshot，并用 stale policy 决定 soft warn 或 fail-close |
| Realtime 范围重新膨胀 | 任务板只保留 disabled contract 维护项，完整 Realtime 需要重新产品决策后另立文档 |

## 设计来源

- [路线图](./00-roadmap.md)
- [P0 生产闭环补齐](./11-p0-production-closure.md)
- [P1 设计能力补齐](./12-p1-design-capabilities.md)
- [P2 架构一致性与高级能力](./13-p2-architecture-advanced.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
- [ADR](../design/ai_gateway_ADR.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单](../tasks.md)
