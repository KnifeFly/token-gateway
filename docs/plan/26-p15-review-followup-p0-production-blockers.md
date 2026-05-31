# P15 Review Follow-up P0 生产阻塞收敛

## 阶段目标

P15 的目标是处理 P12-P14 完成后，新一轮静态 review 暴露出的生产阻塞问题。上一轮已经修掉 stream release ownership、async terminal settlement、async price snapshot、API key HMAC、egressguard 和 README 等基础问题；本阶段继续收敛更深一层的生命周期和身份边界问题。

本阶段优先级高于继续 P11 模型价格目录扩展。任何会影响账务唯一性、并发限流真实性、异步任务最终扣费、callback contract 或 outbound 安全 fail-closed 的问题，都必须在继续新增功能前完成。

## 交付物

- Internal request ID 与 client request ID 完全分离：内部账务、limit、attempt、usage、ledger、failed settlement 和 trace 使用服务端生成的 `internal_request_id`，客户传入的 `X-Request-ID` 只作为 `client_request_id` 透传与观测字段。
- 重复 `X-Request-ID` 回归测试：不同请求携带同一客户 request id 时，必须生成不同 internal request id，并产生独立 hold、usage record、ledger 和 limit lease。
- Redis stream concurrency lease renewal：长 stream 运行超过 `lease_ttl` 后仍占用真实并发配额，直到 `AccountingStream.Close()` 或等价 finalizer 停止 renewal 并释放 lease。
- Task-aware hold reaper：异步任务处于 queued/running/submitted 等未终态时，reaper 不得释放其引用的 active hold。
- Provider webhook 不直接指向 customer callback URL：Replicate 等 provider 不再收到客户 callback URL；所有客户 callback 只能由 gateway callback outbox 统一签名、重试、审计和投递。
- Egress guard 生产 fail-closed：非 local/test 环境中，file URL、callback URL、provider custom base URL 或 provider webhook endpoint 能力启用时，必须启用 egress guard 或显式 allowlist。
- P0 focused tests 和真实依赖回归覆盖 request identity、stream renewal、task hold lifecycle、provider webhook contract 和 egress fail-closed。

## 核心实现顺序

1. 梳理 `RequestState`、HTTP response headers、日志字段、billing repository、limit member、usage attempt、usage record、ledger 和 failed settlement 中所有 `RequestID` 用途，区分内部唯一 ID 与客户透传 ID。
2. 修改 `newState()` 或等价构造逻辑，让 `state.RequestID` 永远由服务端生成；新增或恢复 `state.ClientRequestID` 保存客户 `X-Request-ID`，并在响应中明确返回 `X-Request-ID` 和可选 `X-Client-Request-ID`。
3. 将 balance hold、usage record、ledger、failed settlement、Redis concurrency member、provider/channel attempt lease、trace 关联和 repair key 全部切到 internal request id。
4. 增加重复 `X-Request-ID` 的账务和限流回归测试，覆盖两个不同请求、同一客户 request id、不同 internal request id、两条 usage record、两次独立 settlement。
5. 扩展 limit release 抽象为可 renewal 的 lease，例如 `Renew(ctx)` 或独立 `LimitLease` 接口；为 Redis concurrency lease 增加原子 renew 脚本，更新 zset score 和 key TTL。
6. 在 stream finalizer 或 `AccountingStream` 中启动 renewal ticker，推荐间隔为 `lease_ttl / 3`，close/read error/client disconnect/settlement failure 路径都必须停止 ticker 并幂等 release。
7. 调整 hold reaper repository 查询：释放 expired active hold 前，检查是否被 queued/running/submitted provider task 引用；被引用的 task hold 记录 protected metric 和日志，不释放。
8. 调整 Replicate 或其他 async provider submit 逻辑，不把 `request.Task.CallbackURL` 直接写入 provider webhook 字段；短期优先使用 provider polling，若需要 provider webhook，则指向 gateway internal webhook endpoint。
9. 在 gateway internal webhook 或 polling 完成任务后，统一通过 callback outbox 投递客户 callback，并保证 HMAC、retry、dead-letter、audit 和敏感字段过滤仍生效。
10. 在 config validation 中增加生产 fail-closed 规则：非 local/test 环境若 outbound URL 能力开启而 `gateway.egress.enabled=false`，启动失败；允许部署级 allowlist 作为显式例外。
11. 增加 Redis/MySQL integration 或现有 RC smoke 扩展，验证长 stream 超过 `lease_ttl`、长异步任务超过 hold TTL、provider webhook 配置和 egress 关闭配置的真实行为。

## 关键设计约束

- `X-Request-ID` 是客户观测字段，不是账务幂等键；业务幂等只能由 `Idempotency-Key` 或明确内部幂等状态机表达。
- Internal request id 必须不可由客户控制，且每个 gateway admission 都唯一。
- Stream 并发 lease 的生命周期必须覆盖真实下游占用时间，不能只覆盖 engine 函数返回前的时间。
- Renewal 失败必须可观测；如果 Redis 短暂失败，不得造成 silent over-admission，需要明确选择 fail-open/fail-closed 策略并记录。
- 异步 task hold 的生命周期跟随 task state machine，不再由普通 request hold TTL 盲扫释放。
- 客户 callback contract 只由 gateway 对外承诺；provider callback、webhook、polling 都是内部实现细节。
- Egress guard 在生产中是安全边界，不是可随意关闭的优化项。
- P15 不引入完整 RBAC、对象存储、完整 Realtime、动态插件或新的 public API 面。

## 验收标准

- 两个不同请求携带同一 `X-Request-ID` 时，产生两个不同 internal request id、两个独立 hold、两条 usage record、两条 ledger entry 或对应 failed settlement，不复用 Redis lease member。
- `X-Request-ID` 响应头返回 internal request id；客户传入的 request id 只出现在 `X-Client-Request-ID`、日志和 trace 的 client 字段中。
- `LeaseTTL=1s`、stream 持续 3s、concurrency=1 时，中途第二个请求仍被并发限制拒绝；stream close 后新请求可进入。
- queued/running async task 绑定的 expired hold 不会被 reaper 释放；task terminal 后 settlement 能成功或进入可修复 failed settlement。
- Replicate submit body 或等价 provider request 不包含 customer callback URL；客户只收到 gateway outbox 发出的单次 callback。
- 非 local/test 环境关闭 egress guard 且启用 file URL/callback/provider custom base URL 时，进程启动或配置发布 fail closed。
- Focused tests、Redis/MySQL integration 或 RC smoke 扩展、`go test ./...` 相关包、`go vet ./...` 和 `git diff --check` 通过。

## 风险与处理

| 风险 | 处理 |
|---|---|
| request id 改动触碰账务唯一约束和历史数据 | 新增 internal/client 字段时提供兼容迁移，旧数据按已有 request_id 作为 internal id 解释，新请求走新语义 |
| 客户依赖 `X-Request-ID` 原样回显 | 增加 `X-Client-Request-ID`，README/runbook 明确两种 ID 语义；必要时提供短期兼容 header |
| stream renewal ticker 泄漏 | renewal 与 stream close context 绑定，release 幂等，并在测试中覆盖 read error、client disconnect 和 settlement failure |
| reaper 跳过 task hold 导致长期 active hold | 增加 stuck task 报警、task age metric 和人工/repair workflow，而不是盲目释放可收费任务 hold |
| provider webhook 不传客户 URL 后任务完成变慢 | 短期使用 polling；如接 internal webhook，必须先落库 provider event，再由 gateway callback outbox 投递客户 |
| egress fail-closed 影响企业内网 provider | 只通过显式 allowlist、部署网络策略或私有 provider 配置放行，不允许隐式关闭 guard |

## 设计来源

- [路线图](./00-roadmap.md)
- [M2 账务闭环](./03-m2-billing-loop.md)
- [M3 Streaming + Native Compatible](./04-m3-protocols-streaming.md)
- [M4 Unified Media Async Task](./05-m4-media-tasks.md)
- [P12 Review P0 正确性收敛](./23-p12-review-p0-correctness.md)
- [P13 Review P1 商业账务与安全边界](./24-p13-review-p1-commercial-hardening.md)
- [P14 Review P2 工程交付与安全基线](./25-p14-review-p2-engineering-readiness.md)
- [任务清单](../tasks.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
