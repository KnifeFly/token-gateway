# 商用 AI Gateway ADR

ADR 是 Architecture Decision Record，用来记录关键设计决策、背景、取舍和后果。

## ADR 模板

```markdown
# ADR-000X: 标题

## Status

Proposed / Accepted / Superseded / Deprecated

## Context

背景、问题、约束。

## Decision

做出的设计选择。

## Consequences

正面影响、负面影响、后续工作。

## Alternatives

考虑过但没有选择的方案。
```

---

# ADR-0001: 采用四平面架构

## Status

Accepted

## Context

系统既要服务高并发 AI 调用，又要支持控制面配置、账务结算、异步任务、失败修复。若所有能力放在一个进程中，会导致扩展和故障隔离困难。

## Decision

采用四平面：

```text
Data Plane: cmd/gateway
Control Plane: cmd/control-api
Config Plane: cmd/configd
Worker Plane: cmd/worker
```

## Consequences

优点：职责清晰，可独立扩缩容。缺点：部署和本地开发复杂度增加。

## Alternatives

单体进程：简单，但长期不可维护。

---

# ADR-0002: 统一 URI 但引入 APIClassifier 消歧

## Status

Accepted

## Context

OpenAI-compatible API 和 Unified Media API 会共享 `/v1/images/generations`、`/v1/audio/speech` 等路径。

## Decision

保留统一 URI，通过 `X-Gateway-Protocol`、model registry、body schema 进行消歧。无法判断时返回 `ambiguous_protocol`。

## Consequences

优点：对外 URI 简洁，兼容性好。缺点：classifier 复杂度上升，需要严密测试。

## Alternatives

使用 `/unified/v1/...`：无冲突，但对外协议割裂。

---

# ADR-0003: 数据面只读 Runtime Snapshot

## Status

Accepted

## Context

数据面高并发请求不能实时查控制面数据库，否则性能和稳定性受影响。

## Decision

控制面生成 RuntimeSnapshot，数据面原子加载本地 indexed snapshot。

## Consequences

优点：热路径快，配置可版本化。缺点：存在最终一致性窗口，需要 key revoke blacklist 和 price pinning。

## Alternatives

每次请求查 DB：简单但不可扩展。

---

# ADR-0004: 账务使用 Balance Hold + Settlement + Ledger

## Status

Accepted

## Context

AI provider 调用有成本。不能先调用 provider 再检查余额。

## Decision

请求前预估并创建 balance hold；请求后根据实际 usage settlement；写 ledger；失败写 failed_settlement 并由 worker replay。

## Consequences

优点：避免欠费，支持失败修复。缺点：实现复杂，需要幂等和对账。

## Alternatives

请求后直接扣费：provider 成功但扣费失败时资金风险高。

---

# ADR-0005: MVP 插件 Phase 精简为 9 个

## Status

Accepted

## Context

v0.2 定义了 18 个 phase，评审认为存在过度设计风险。

## Decision

MVP 只启用 9 个 phase：

```text
pre_request
post_auth
pre_prompt
pre_route
post_route
pre_provider
post_provider
pre_settlement
audit
```

扩展 phase 保留但默认不遍历。

## Consequences

优点：降低复杂度和性能损耗。缺点：部分高级插件需要等待后续阶段启用。

## Alternatives

保留 18 个全部启用：灵活但过度设计。

---

# ADR-0006: 异步任务支持 Idempotency-Key

## Status

Accepted

## Context

视频、生图等任务耗时长，客户端重试很常见。如果没有幂等，可能重复创建任务和重复扣费。

## Decision

异步任务和扣费型写操作支持 `Idempotency-Key`。作用域为 `tenant_id + api_key_id + endpoint + key`。

## Consequences

优点：避免重复任务和重复扣费。缺点：需要维护幂等记录和请求体 hash。

## Alternatives

让客户端自己去重：不可靠。

---

# ADR-0007: Realtime 只做协议预留，不进入 MVP

## Status

Accepted

## Context

Realtime/WebSocket/WebRTC 是未来趋势，但会显著增加状态管理、音频流、双向协议、计费复杂度。

## Decision

OpenAPI 预留 `/v1/realtime/sessions`，代码预留 `RealtimeEngine`，MVP 返回 501。

## Consequences

优点：协议上可演进。缺点：短期不支持 realtime 客户。

## Alternatives

MVP 直接实现 realtime：风险过高。

---

# ADR-0008: Redis 限流采用 bucket/counter/lease 三类语义

## Status

Accepted

## Context

生产限流同时需要 QPS/RPM/TPM、预算和并发控制。固定窗口计数无法准确表达突发平滑、token 预扣和请求生命周期内的并发占用。

## Decision

Redis Lua 输入拆为三类操作：QPS、RPM、TPM 和 cost-per-minute 使用 token bucket；daily budget 使用按日 counter；concurrency 使用带 TTL 的 sorted-set lease。请求只缓存短期 deny，不缓存 allow。

## Consequences

优点：多副本下限流语义清晰且原子。缺点：Redis key 类型更多，集成测试必须覆盖 refill、预扣和 release。

## Alternatives

全部使用固定窗口 counter：实现简单，但 TPM 和突发流量语义不符合商用设计。

---

# ADR-0009: 计费由统一 BillabilityPolicy 判定

## Status

Accepted

## Context

同步、流式和异步任务都有 provider success、部分输出、客户端断开、provider error、任务取消等边界。如果各 settlement 入口自行判断，会导致账务不可解释。

## Decision

引入统一 `BillabilityPolicy`。所有 settlement 入口先生成 billability context，再由 policy 输出 billable 与 reason；无有效输出默认不计费，已向客户交付部分输出时按策略计费。

## Consequences

优点：账务原因可审计，replay 和人工排查有同一事实来源。缺点：新增场景时必须补 policy case 和 focused tests。

## Alternatives

在 stream finalizer、billing planner 和 task settlement 分别编码：短期快，但长期账务口径会漂移。

---

# ADR-0010: RouteSignals 使用 Redis hot data，缺失时安全回退

## Status

Accepted

## Context

health/cost/latency/quota 路由策略不能只依赖静态 route policy。信号必须能被多个 gateway 副本共享，同时不能让控制面 DB 进入热路径。

## Decision

RouteSignals 从 Redis hash 读取 channel 级健康、权重、延迟、成本、剩余额度、禁用状态和模型兼容性。缺失信号按 healthy、compatible、weight=1 的安全默认值处理，并继续允许 priority/weighted 策略回退。

## Consequences

优点：策略可读取实时运营信号，且数据面仍只访问 snapshot、Redis hot data 和内存索引。缺点：信号生产者需要独立保障 TTL 和更新频率。

## Alternatives

请求时查询控制面或报表库：数据更完整，但破坏热路径隔离和稳定性。

---

# ADR-0011: 当前路线收敛为 Provider 兼容、可靠性、非存储媒体转发、Portal API 和客户验收

## Status

Accepted

## Context

M0-P4 已经完成主体网关、账务、snapshot、worker、媒体任务和发布候选验收。后续如果继续把控制面安全平台、复杂财务、对象存储、完整 Realtime、生产级 Observability、WASM/动态插件、semantic routing/cache 和多地域多活都放入同一路线，会稀释 provider 兼容和客户接入的优先级。

## Decision

当前后续路线只推进 P5 Provider 协议兼容、P6 Provider 可靠性、P7 非存储媒体转发生态、P8 Portal API 和 P9 客户接入验收收口。

明确当前不做：

```text
控制面 RBAC / 审计平台
复杂财务 / 发票闭环
对象存储
完整 Realtime WebSocket / WebRTC
生产级 Observability 扩展平台
WASM 插件
动态脚本插件
```

明确当前先不做：

```text
semantic routing
semantic cache
跨地域多活
```

`/v1/files/*` 只表达 transient/non-storage input asset，用于请求归一化、幂等校验、大小限制和 provider 转发。Portal 第一版复用 API key 鉴权，只开放模型、schema、credits、usage、API key 自助管理和 task 查询，不暴露 admin/control 配置能力。P9 只补齐 Portal smoke、OpenAPI import preflight 和 RC smoke 集成，不新增产品面。

## Consequences

优点：后续路线聚焦客户接入、上游稳定性、自助查询和可重复验收，避免网关产品边界扩大成对象存储、审计平台或多地域平台。缺点：需要在文档和 OpenAPI 中持续标注 non-storage、not planned 和先不做边界。

## Alternatives

继续把所有能力纳入 P5 之后路线：覆盖面更广，但会让优先级和验收标准失焦。
