# P17 Review Follow-up Worker 与 Callback 稳定性

## 阶段目标

P17 的目标是把后台 worker、provider task poller、failed settlement replay 和 callback dispatcher 从“可运行”推进到“多副本下可解释、可恢复、可限并发”的状态。

本阶段承接 P15/P16 的账务和 callback contract 收敛，不新增客户 API。重点是 job lease renewal、per-job concurrency、单任务错误隔离、callback durable claim 和 delivery 细节，避免 worker 超时、lease 过期或单个坏任务导致重复执行、批量阻塞或重复 callback。

## 交付物

- Worker job lease TTL 与 job timeout 配置约束：默认 lease TTL 大于 timeout，或所有长 job 都有 heartbeat renewal。
- Worker lease heartbeat：job 运行期间定期续约 Redis `SET NX` lease 或等价锁，避免接近 timeout 时另一个 worker 抢到同一 job。
- `MaxConcurrency()` 接口语义落地或删除：如果保留，Runner 必须按 job 的 max concurrency 启动 bounded worker pool/semaphore；如果不保留，接口和配置从代码中移除，避免误导。
- ProviderTaskPoller 单任务错误隔离：一个 provider poll error 只影响当前 task，不能中断整个 batch；DB/Redis 等基础设施错误仍可返回 job-level error。
- CallbackDispatcher durable claim：多 worker 或 lease 过期时，同一 callback delivery 不应被重复投递；claim 失败、超时、重试和 dead-letter 可观测。
- Callback HTTP 细节补齐：投递后 drain/close response body，记录 status、latency、retryable、signature version、delivery id 和 body size。
- Worker/callback failure drills：覆盖 lease 过期、heartbeat 失败、poll 单任务失败、callback 5xx/timeout、dispatcher 崩溃后恢复和多 worker 并发。

## 核心实现顺序

1. 审计 Runner、job interface、lease manager、provider poller、callback dispatcher 和 failed settlement replay 当前配置，列出每个 job 的 timeout、lease TTL、max concurrency、retry 和 batch 行为。
2. 在配置 normalize/validate 阶段强制 `job_lease_ttl >= job_timeout * 3`，或为支持 heartbeat 的 job 明确配置 `heartbeat_interval <= lease_ttl / 3`。
3. 实现 worker lease heartbeat：job 开始后由 Runner 或 lease manager 启动 ticker，周期性延长 Redis lease TTL；job finish、panic recovery、context cancel 时停止 heartbeat 并释放。
4. 定义 heartbeat 失败策略：短暂失败可重试并记录 warning，连续失败或 lease owner mismatch 时停止当前 job 或进入 degraded mode，避免两个 owner 同时执行高价值 job。
5. 落地 `MaxConcurrency()`：Runner 为每个 job 按 max concurrency 启动 bounded workers，或通过 semaphore 并发处理 batch item；callback 默认可并发，provider poller 默认保持 1。
6. 重构 ProviderTaskPoller batch：将 task 处理拆成 `processOne(task)`，每个 task 的 provider error、bad payload、terminal settlement failure、callback enqueue failure 都记录 task-level error 后继续处理后续 task。
7. 为 CallbackDispatcher 增加 durable claim 字段或状态：pending -> processing -> delivered/dead_letter，记录 owner、claimed_at、attempt_count、next_retry_at、last_error。
8. 投递 callback 时保证 request body 只含 gateway contract 字段，HMAC 签名覆盖 body/timestamp/delivery id；收到响应后 drain/close body，避免连接池不可复用。
9. 增加 metrics/log/trace：job lease heartbeat status、owner mismatch、job duration、batch processed/skipped/error、callback claim conflict、delivery result、dead-letter backlog。
10. 编写 focused tests 和 failure drills，模拟多 worker、lease TTL 过期、heartbeat 中断、单任务 provider error、callback timeout/5xx 和 dispatcher crash recovery。

## 关键设计约束

- Worker lease 不是账务幂等的替代品；账务、callback 和 poller 各自仍要有 durable idempotency/claim。
- 高价值 job 不能只依赖本地内存防重；多副本语义必须依赖 Redis lease、数据库 claim 或二者组合。
- Callback 对客户是商业 contract，重复投递必须由 delivery id 和 durable state 控制，并在文档中说明 retry/dedup 语义。
- Provider poller 不能让一个坏 provider task 阻塞同批其他任务。
- `MaxConcurrency()` 要么真实生效，要么移除；不允许保留未使用接口制造错误预期。
- P17 不改变 task/customer callback public API，除非为了增加兼容 header 或 delivery id 且同步 OpenAPI。

## 验收标准

- job 运行时间接近或超过旧 lease TTL 时，第二个 worker 不能获取同一 job lease；heartbeat 停止后 lease 能按预期释放或过期。
- job lease TTL 小于 timeout 的配置在生产环境启动失败，或被 normalize 到安全值并输出明确 warning。
- ProviderTaskPoller batch 中一个 task poll 返回 5xx/timeout/bad JSON 时，后续 task 仍被处理，错误被记录到 task attempt/audit。
- CallbackDispatcher 多 worker 并发时，同一 outbox row 只被一个 owner claim；owner 失效后可重新 claim，并不会无限 processing。
- Callback 投递 drain/close response body，连接复用测试或 fake transport 断言通过。
- `MaxConcurrency()` 对 callback dispatcher 生效，provider poller 保持单任务或配置上限；相关并发测试稳定。
- Failure drills 和 focused tests 通过，worker metrics 能显示 heartbeat、claim、processed/error/dead-letter 状态。

## 风险与处理

| 风险 | 处理 |
|---|---|
| heartbeat 失败后 job 中止造成部分批次未处理 | 每个 batch item 必须可重试，job 中止只影响本轮调度，不破坏 durable state |
| durable claim 迁移影响已有 outbox 行 | 新字段默认 pending/nullable owner，migration 后按旧状态兼容读取 |
| 并发 callback 增加客户压力 | `MaxConcurrency()` 默认保守，并支持租户级或全局速率限制 |
| poller 单任务错误被吞掉 | task-level error 必须进入日志、metrics、last_error 或 attempt audit，不允许静默 continue |
| Runner 复杂度上升 | lease heartbeat、concurrency 和 batch isolation 分成小接口，避免把业务逻辑塞进 Runner |

## 设计来源

- [路线图](./00-roadmap.md)
- [M4 Unified Media Async Task](./05-m4-media-tasks.md)
- [P4 Release Candidate Readiness](./15-p4-release-candidate-readiness.md)
- [P6 Provider Reliability](./17-p6-provider-reliability.md)
- [P13 Review P1 商业账务与安全边界](./24-p13-review-p1-commercial-hardening.md)
- [P15 Review Follow-up P0 生产阻塞收敛](./26-p15-review-followup-p0-production-blockers.md)
- [P16 Review Follow-up 账务与审计一致性](./27-p16-review-followup-accounting-audit.md)
- [任务清单](../tasks.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
