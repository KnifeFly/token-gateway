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
