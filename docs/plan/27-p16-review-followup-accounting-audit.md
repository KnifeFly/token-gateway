# P16 Review Follow-up 账务与审计一致性

## 阶段目标

P16 的目标是在 P15 生产阻塞收敛后，修复不会立即阻断单次请求、但会削弱商业报表、账务解释、故障修复和幂等体验的一致性问题。

本阶段重点不是扩展新协议，而是让 sync/async 两条路径在 attempt、idempotency、fallback、failed settlement、budget 和 reporting 上具备同等审计质量。完成后，运营、财务和排障人员应能回答每一次 provider 调用为什么发生、是否最终计费、失败是否可修复、预算消耗如何解释。

## 交付物

- Async idempotency 并发 race 稳定 replay：并发同 key 同 body 创建任务时，只有一个 task/provider submit 成功，第二个请求返回已有 task 或明确的 pending/409 语义，不暴露 SQL duplicate error。
- Async submit attempts 持久化到 `usage_attempts` 或等价 durable audit 表，覆盖 provider/channel、upstream model、错误、retryable、fallback、final 和 task id。
- Sync dispatcher 最后失败 attempt 的 `final` 标记 durable 化，避免只在内存中修正。
- Failed settlement replay 增加 row claim、processing state、owner、heartbeat 或等价机制，降低多 worker 下重复处理与状态竞态。
- Budget 语义收敛：明确 `daily_budget_micros` 是 admission estimate guard，还是升级为 ledger-based real spend budget；命名、配置、报表和 runbook 必须一致。
- 账务不变量测试覆盖 successful provider result、failed settlement、idempotency replay、重复 client request id、async fallback 和 final failed attempt。
- 报表和 runbook 能解释 estimated admission、actual settlement、provider cost、failed repair backlog 与 reconciliation 差异。

## 核心实现顺序

1. 梳理 async task 创建流程中的 idempotency pre-check、task insert、idempotency binding、provider submit 和 admission hold release 顺序，定位 duplicate key 发生点。
2. 在 repository 层捕获 duplicate idempotency error：事务 rollback 后按 `(tenant_id, api_key_id, endpoint, idempotency_key)` 查已有记录，校验 request hash，并返回已有 task 和 `hit=true`。
3. 若需要更强语义，设计 `started -> bound -> completed` 的 idempotency acquire 状态机；并发第二个请求看到 `started` 时短暂等待、返回 202/pending，或稳定返回 409，不返回 SQL 错误。
4. 定义 async submit attempt durable schema 或复用现有 `usage_attempts` 字段，补齐 `task_id`、`attempt_index`、`provider_type`、`channel_id`、`upstream_model`、`status_code`、`error_code`、`retryable`、`fallback_from`、`final` 和 `created_at`。
5. 给 `TaskBridge`、async provider dispatcher 或等价组件注入 `AttemptRecorder`，确保每个 submit candidate 的成功、失败、fallback 和 final 都在事务边界外可追踪。
6. 修正 sync dispatcher candidate loop：在持久化 attempt 前判断 `isLastCandidate`；最后一个 candidate 即使错误 eligible 也必须 durable 标记 `final=true`，或在 `markFinalAttempt()` 后执行 durable update。
7. 为 failed settlement replay 增加 claim 机制：按状态筛选 pending/failed_retryable，原子更新为 processing，并记录 owner、claimed_at、heartbeat_at、attempt_count 和 next_retry_at。
8. 校准 budget 命名与实现：若保留 Redis admission guard，改名或文档明确为 `daily_admission_budget_micros`；若做真实 spend budget，则以 ledger/settlement 聚合为准并定义延迟与一致性边界。
9. 更新 reporting、runbook、metrics 和 alert，让 async attempt 失败率、final failed attempt、failed settlement backlog、budget estimate vs actual 能被查到。
10. 增加 focused tests、并发测试和真实依赖 integration，覆盖 duplicate idempotency、async fallback attempt audit、final failed attempt durable update、multi-worker failed settlement claim 和 budget 语义。

## 关键设计约束

- Async idempotency 的最终语义必须和扣费语义一致：同 key 同 body 不重复 provider submit，不重复扣费，不泄漏第二个 hold。
- Request hash 不一致必须返回稳定 conflict，不得 replay 已有 task。
- Usage attempt 是商业审计记录，不是内存调试信息；sync 和 async provider 尝试都必须 durable。
- `final` 语义以持久化记录为准，不能只依赖 `state.Attempts` 内存状态。
- Failed settlement replay 可重复执行，但 worker claim 必须降低并发重复处理和 retry_count 竞态。
- Budget 名称必须匹配产品语义；如果只是 admission 估算 guard，不能在客户账单中宣称为真实 spend budget。
- P16 不新增价格体系；如需要复杂 price component，仍回到 P11 范围。

## 验收标准

- 两个同 `Idempotency-Key`、同 body、同租户/项目/API key 的并发 async 请求只创建一个 task 和一个 provider task；两个响应都能稳定指向同一个 task 或一个返回明确 pending/conflict 语义。
- 同 key 不同 body 返回 deterministic conflict，第二个 admission hold 被释放或根本不创建。
- Async submit 第一个 candidate 失败并 fallback 到第二个 candidate 时，两条 durable attempt 均可查询，最终 attempt 的 `final=true` 正确。
- Sync dispatcher 最后一个 candidate 失败时，数据库中的 attempt 记录也标记 final，不只修改内存对象。
- 多个 worker 同时运行 failed settlement replay 时，同一行只被一个 owner claim；owner 失效后可按超时重新 claim。
- Budget runbook 和 metrics 能区分 estimated admission consumption、actual ledger settlement 和 reconciliation 差异。
- 账务不变量测试证明：每个 successful provider result 最终有 usage record/ledger 或 failed settlement；每个 hold 最终 settled/released/protected/repairable。

## 风险与处理

| 风险 | 处理 |
|---|---|
| duplicate idempotency 处理引入阻塞等待 | 先实现 duplicate 后查已有 task 的稳定 replay；状态机等待作为可选增强 |
| async attempt 写入失败影响 task submit | attempt audit 写失败需要明确策略：高价值商业路径优先 fail closed 或写 failed settlement/audit backlog |
| failed settlement claim 状态卡住 | 增加 heartbeat、claim timeout、manual reset 和 alert |
| budget 改名影响配置兼容 | 支持旧字段读取并发 warning，snapshot 构建时规范化到新语义 |
| 报表查询变慢 | 为 task_id/request_id/final/status/created_at 增加必要索引，避免运营查询拖慢热路径 |

## 设计来源

- [路线图](./00-roadmap.md)
- [M2 账务闭环](./03-m2-billing-loop.md)
- [M4 Unified Media Async Task](./05-m4-media-tasks.md)
- [P3 Production Hardening](./14-p3-production-hardening.md)
- [P6 Provider Reliability](./17-p6-provider-reliability.md)
- [P15 Review Follow-up P0 生产阻塞收敛](./26-p15-review-followup-p0-production-blockers.md)
- [任务清单](../tasks.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
