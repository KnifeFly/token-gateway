# P0 生产闭环缺口补齐

## 阶段目标

补齐 M0-M9 之后仍影响生产上线的闭环缺口。重点不是扩展新能力，而是让已有设计中的 worker、异步任务、公开 API、runtime snapshot 安全策略和紧急禁用能力可运行、可验证、可恢复。

## 交付物

- `cmd/worker` 成为可运行进程，接入配置、DB、Redis、billing、task、logger、metrics 和 tracing。
- Worker runner 支持 graceful shutdown、并发控制、lease、防重复执行、retry/backoff、panic recovery、结构化日志和 metrics。
- `ProviderTaskPoller`、`FailedSettlementReplayer` 和 `CallbackDispatcher` 由 worker 进程实际调度。
- 真实异步 provider task adapter，支持 submit、poll 和 cancel，mock 仅保留在测试路径。
- async media task 从提交、轮询、状态回写、callback 到 settlement 完成闭环。
- `/v1/models`、`/v1/models/{model}/schema`、`/v1/credits` 和 `/v1/moderations` 与 OpenAPI 合同对齐。
- Runtime snapshot soft stale、hard stale 和 fail-close 策略。
- Provider/channel emergency disable 热路径，支持无需重启 gateway 生效。

## 核心实现顺序

1. 梳理现有 task、billing、callback 和 snapshot 代码，确认可复用的 service、repository 和配置入口。
2. 实现 worker 进程依赖装配，复用现有 gateway/control-api 的配置、日志、DB、Redis、metrics 和 trace 初始化方式。
3. 实现通用 job runner，统一 lease、并发、重试、panic recovery、shutdown 和 metrics。
4. 将 provider task polling、failed settlement replay 和 callback dispatch 接入 runner。
5. 抽象并实现生产 provider task adapter，完成 submit、poll、cancel 和 provider error mapping。
6. 让 async media task 从 API 创建任务后进入 worker 轮询、结果归一化、结算和 callback outbox。
7. 实现并注册公开模型、模型 schema、credits 和 moderation HTTP handler。
8. 为 runtime snapshot 增加 soft stale 告警、hard stale fail-close 和诊断指标。
9. 实现 emergency disable 热路径，并让路由或 dispatch 阶段拒绝被禁用的 provider/channel。

## 关键设计约束

- Worker 不拥有业务规则，只负责调度后台任务和执行已有 use case。
- Provider adapter 只做上游任务提交、轮询、取消、错误映射和 usage/result 解析，不拥有路由、计费、租户或权限决策。
- Async task 必须先创建 internal task，再提交 provider task，避免外部任务失联。
- Callback 失败不能阻塞任务完成，必须进入 outbox 重试。
- Snapshot hard stale 后不得静默继续服务；必须 fail-close 或按明确降级策略拒绝请求。
- Emergency disable 必须绕过慢速 snapshot 发布链路，优先生效于 provider/channel 选择和请求分发。
- 公开 API 行为必须与 `docs/design/ai_gateway_openapi.yaml` 同步维护。

## 验收标准

- `cmd/worker` 可启动、可优雅退出，并暴露 worker/job 级 metrics。
- 同一任务在多 worker 副本下不会被重复执行。
- Provider task submit 成功后 external task ID 落库，poll 完成后任务状态和结果资产可查询。
- Task 成功、失败、取消都能进入明确状态，并触发结算或失败修复流程。
- Callback 失败后进入重试队列，重试结果可追踪。
- Failed settlement replay 能由 worker 自动执行，且 replay 幂等。
- `/v1/models`、`/v1/models/{model}/schema`、`/v1/credits` 和 `/v1/moderations` 有 HTTP 测试覆盖。
- Snapshot soft stale 产生可观测告警，hard stale 按策略拒绝热路径请求。
- Emergency disable 后，被禁用 provider/channel 不再被新请求选中。

## 风险与处理

| 风险 | 处理 |
|---|---|
| worker 重复执行导致重复扣费或重复 callback | 使用 lease、幂等键、状态机和 settlement 幂等保护 |
| provider task 已提交但本地状态未记录 | 先创建 internal task，再提交 provider task，并立即保存 external task ID |
| callback 失败影响主任务状态 | callback outbox 独立重试，任务完成状态不回滚 |
| snapshot 过期仍继续接流量 | hard stale fail-close，并暴露 staleness metrics |
| emergency disable 生效太慢 | 使用 Redis hot deny 或等价热路径机制，不等待完整 snapshot 发布 |

## 设计来源

- [路线图](./00-roadmap.md)
- [M4 Unified Media Async Task](./05-m4-media-tasks.md)
- [M5 控制面与 Runtime Snapshot](./06-m5-control-plane-snapshot.md)
- [M7 生产观测与稳定性](./08-m7-observability-stability.md)
- [M9 商业化运营能力](./10-m9-commercial-ops.md)
- [实施计划](../design/ai_gateway_implementation_plan.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单](../tasks.md)
