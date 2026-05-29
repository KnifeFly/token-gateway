# 商用 AI Gateway 执行任务清单

本文是最终设计包的唯一执行看板，综合了 v0.2 的细粒度商用任务和 v0.3 的架构修订。实现时只维护本文件的任务状态，不再在 `docs/design` 下维护第二份 task list。

## 当前状态

- 已完成：v2/v3 设计包归一为最终版文档；M0 Go 工程骨架、配置、错误、日志、HTTP server、metrics、DB/Redis client、migration、Makefile、compose 和 CI 已落地；M1 `/v1/chat/completions` 非流式数据面已跑通 APIClassifier、BodyStore/OpenAI Chat Parser、API key auth、seed RuntimeSnapshot、priority/weighted routing、OpenAI-compatible provider adapter、ProviderDispatcher、provider attempt metrics/trace 和 settlement mock；M2 账务闭环已落地 balance account/hold、usage attempt、usage record、ledger、failed settlement replay、Redis limit 和 reconciliation；M3 已扩展 OpenAI stream/Responses/Embeddings、Claude Messages、Gemini GenerateContent/streamGenerateContent、ProviderStream/AccountingStream、SSE writer、StreamFinalizer、downstream disconnect 分类和 `ambiguous_protocol` 映射；M4 已落地 Unified Media async task、Task/File domain、Idempotency-Key、task/file migration、TaskBridge、provider task poller、callback outbox、task settlement 和 base64/url/stream file service。
- 未开始：M5 control plane + runtime snapshot 和后续控制面能力。
- 阻塞：无。
- 下一步建议：进入 M5，先实现 admin auth 和 tenant/project/api key CRUD，再推进 model/schema/channel/credential/route/price/limit CRUD 与 snapshot builder/publisher/watcher。
- 最近验证：M4 开发后 `go test ./...`、`make lint`、`make build`、`make race`、`git diff --check` 均通过；本轮未重跑 compose smoke。

## 使用规则

- 任务 ID 采用 `M{milestone}/E{epic}-T{number}/P{priority}` 格式。
- 第一轮优先完成 M0-M3 的 P0 任务，形成最小商用内核。
- M4-M9 只拆到可执行粒度，避免早期过度展开控制面、插件和运营后台。
- 每个任务完成时必须按影响范围更新代码、测试、迁移、OpenAPI、ADR、metrics、trace、日志、审计、配置、计划和故障说明。

## M0 基础工程与文档归一

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M0/E0-T01/P0 | P0 | 初始化 Go module 和最小目录 | repo root, `cmd/`, `internal/`, `pkg/` | `go test ./...` 可运行 |
| [x] | M0/E0-T02/P0 | P0 | 建立 Makefile | `Makefile` | test、lint、race、build、run 目标可用 |
| [x] | M0/E0-T03/P0 | P0 | 建立配置加载 | `internal/bootstrap/config.go` | default、yaml、env、normalize、validate 支持 |
| [x] | M0/E0-T04/P0 | P0 | 建立统一错误包 | `pkg/apperr` | code、status、type、retryable、safe 可表达 |
| [x] | M0/E0-T05/P0 | P0 | 建立结构化日志 | `internal/infra/log` | request_id、trace_id 可输出 |
| [x] | M0/E0-T06/P0 | P0 | 建立 HTTP server 基础 | `internal/transport/httpserver` | `/healthz`、`/readyz`、`/metrics` 可访问 |
| [x] | M0/E0-T07/P0 | P0 | 初始化 Prometheus 和 OTel | `internal/infra/telemetry` | metrics 和 trace exporter 可配置 |
| [x] | M0/E0-T08/P0 | P0 | 初始化 DB、Redis 和 migration | `internal/infra/db`, `internal/infra/redis`, `migrations/` | ping、close、migrate up/down 可用 |
| [x] | M0/E0-T09/P1 | P1 | 建立 CI | `.github/workflows` | PR 触发 test、vet/lint、race |
| [x] | M0/E0-T10/P1 | P1 | 固化文档入口 | `docs/design`, `docs/plan`, `docs/tasks.md` | 无旧 v2/v3 设计入口，OpenAPI 可导入 |

## M1 最小非流式数据面

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M1/E1-T01/P0 | P0 | 定义 RequestState | `internal/dataplane/engine/state.go` | 覆盖 protocol、principal、snapshot、route、billing、observe |
| [x] | M1/E1-T02/P0 | P0 | 实现 APIClassifier | `internal/dataplane/classifier` | OpenAI chat 可分类，冲突路径有测试骨架 |
| [x] | M1/E1-T03/P0 | P0 | 实现 BodyStore 和 OpenAI Chat Parser | `internal/dataplane/parser` | model、messages、stream flag、usage estimate 可解析 |
| [x] | M1/E1-T04/P0 | P0 | 实现 API key extractor 和 hash 校验 | `internal/dataplane/auth` | Bearer/x-api-key 支持，明文 key 不落日志 |
| [x] | M1/E1-T05/P0 | P0 | 定义最小 RuntimeSnapshot 和 IndexedSnapshot | `internal/controlplane/snapshot`, `internal/dataplane/snapshot` | api key、model、channel、route 索引可用 |
| [x] | M1/E1-T06/P0 | P0 | 实现 RoutePlanner 和 priority selector | `internal/dataplane/router` | 可选中 mock channel，无路由返回标准错误 |
| [x] | M1/E1-T07/P0 | P0 | 实现 Provider relay types 和 registry | `internal/provider/relay`, `internal/provider` | adapter 能注册和按 capability 获取 |
| [x] | M1/E1-T08/P0 | P0 | 实现 OpenAI-compatible adapter | `internal/provider/openai` | 非流式 chat 可调用 mock/真实上游 |
| [x] | M1/E1-T09/P0 | P0 | 实现 ProviderDispatcher 和 attempt 记录 | `internal/dataplane/dispatch` | provider 5xx、429、401 分类正确 |
| [x] | M1/E1-T10/P0 | P0 | 实现 GatewayEngine.Handle 非流式主链路 | `internal/dataplane/engine` | curl `/v1/chat/completions` 成功 |
| [x] | M1/E1-T11/P1 | P1 | 接入基础观测 | `internal/dataplane/observe` | access log、provider attempt metrics、route span 可见 |

## M2 账务闭环

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M2/E2-T01/P0 | P0 | 建立账务表 migration | `migrations/` | balances、holds、attempts、records、ledger、failed_settlements 可迁移 |
| [x] | M2/E2-T02/P0 | P0 | 实现金额和价格类型 | `pkg/money`, `internal/domain/pricing` | 金额使用 micros，不用 float |
| [x] | M2/E2-T03/P0 | P0 | 实现 PriceQuoter / PriceEstimator | `internal/dataplane/admission` | input/output token 可报价 |
| [x] | M2/E2-T04/P0 | P0 | 实现 Balance service 和 hold | `internal/billing` | 余额不足不调用 provider |
| [x] | M2/E2-T05/P0 | P0 | 实现 AdmissionController.Reserve | `internal/dataplane/admission` | request_id 幂等创建 hold |
| [x] | M2/E2-T06/P0 | P0 | 实现 usage attempt writer | `internal/billing` | 每次 provider attempt 有记录 |
| [x] | M2/E2-T07/P0 | P0 | 实现 Settlement planner/executor | `internal/billing` | provider 成功后扣费并 release hold |
| [x] | M2/E2-T08/P0 | P0 | 实现 Ledger service | `internal/billing` | ledger entry 幂等且可对账 |
| [x] | M2/E2-T09/P0 | P0 | 实现 failed settlement replay worker | `internal/worker/jobs` | 结算失败可重放且不重复扣费 |
| [x] | M2/E2-T10/P1 | P1 | 实现 Redis token bucket 和 concurrency lease | `internal/dataplane/limit` | 多副本 QPS/TPM/concurrency 准确 |
| [x] | M2/E2-T11/P1 | P1 | 实现初版 reconciliation query | `internal/billing/reconciliation.go` | 可发现 ledger 与 balance 差异 |

## M3 Streaming + Native Compatible

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M3/E3-T01/P0 | P0 | 实现 SSE writer | `internal/transport/httpserver/sse_writer.go` | OpenAI stream 可输出 |
| [x] | M3/E3-T02/P0 | P0 | 定义 ProviderStream / AccountingStream | `internal/provider/relay`, `internal/dataplane/stream` | close-time settlement 入口唯一 |
| [x] | M3/E3-T03/P0 | P0 | 实现 StreamFinalizer | `internal/dataplane/stream/finalizer.go` | stream close 后完成 settlement |
| [x] | M3/E3-T04/P0 | P0 | 分类 client disconnect | `internal/dataplane/stream` | 不误罚 provider health |
| [x] | M3/E3-T05/P0 | P0 | 实现 Claude Messages parser/adapter | `internal/provider/claude` | `/v1/messages` 可用 |
| [x] | M3/E3-T06/P0 | P0 | 实现 Gemini GenerateContent parser/adapter | `internal/provider/gemini` | `/v1beta/...` 可用 |
| [x] | M3/E3-T07/P1 | P1 | 实现 OpenAI Responses / Embeddings | `internal/provider/openai` | OpenAI SDK 可调用 |
| [x] | M3/E3-T08/P1 | P1 | 补齐协议消歧测试 | `internal/dataplane/classifier`, `internal/dataplane/engine` | `X-Gateway-Protocol`、model registry、body schema 和 `ambiguous_protocol` 覆盖 |

## M4 Unified Media Async Task

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M4/E4-T01/P0 | P0 | 实现 Task domain 和状态机 | `internal/task` | queued、running、succeeded、failed、canceled 有效 |
| [x] | M4/E4-T02/P0 | P0 | 实现 IdempotencyStore | `internal/task/idempotency.go` | 同 key 同 body 返回同 task，同 key 不同 body 返回 409 |
| [x] | M4/E4-T03/P0 | P0 | 实现 Unified media parser | `internal/dataplane/parser/unified_media_parser.go` | image、video、audio、music 和 `model_params` 可解析 |
| [x] | M4/E4-T04/P0 | P0 | 实现 File service | `internal/task/file_service.go` | base64、url、stream upload 可用 |
| [x] | M4/E4-T05/P0 | P0 | 实现 TaskBridge | `internal/task/bridge.go`, `internal/dataplane/engine` | 创建 internal task 并返回 task object |
| [x] | M4/E4-T06/P0 | P0 | 实现 ProviderTaskDispatcher | `internal/task/provider_dispatcher.go` | external_task_id 落库 |
| [x] | M4/E4-T07/P0 | P0 | 实现 provider task poller | `internal/worker/jobs/provider_task_poller.go` | task 状态推进 |
| [x] | M4/E4-T08/P0 | P0 | 实现 callback outbox/dispatcher | `internal/worker/jobs/callback_dispatcher.go` | callback 失败可重试 |
| [x] | M4/E4-T09/P1 | P1 | 实现 task settlement | `internal/task/settlement.go` | 任务成功后最终扣费 |

## M5 Control Plane + Snapshot

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | M5/E5-T01/P0 | P0 | 实现 admin auth | `internal/transport/controlhttp` | admin token 或 RBAC 可用 |
| [ ] | M5/E5-T02/P0 | P0 | 实现 tenant/project/api key CRUD | `internal/controlplane/admin` | key create、disable、list 可用 |
| [ ] | M5/E5-T03/P0 | P0 | 实现 model/schema/alias CRUD | `internal/controlplane/admin` | 新增模型无需重启 gateway |
| [ ] | M5/E5-T04/P0 | P0 | 实现 provider/channel/credential CRUD | `internal/controlplane/admin` | credential 加密，不进 snapshot 明文 |
| [ ] | M5/E5-T05/P0 | P0 | 实现 route/price/limit CRUD | `internal/controlplane/admin` | route、price、limit 可配置 |
| [ ] | M5/E5-T06/P0 | P0 | 实现 snapshot builder/validator | `internal/controlplane/snapshot` | 坏配置拒绝发布 |
| [ ] | M5/E5-T07/P0 | P0 | 实现 snapshot publisher/watcher | `internal/controlplane/snapshot`, `internal/dataplane/snapshot` | gateway 原子切换 snapshot |
| [ ] | M5/E5-T08/P0 | P0 | 实现 request-level pinning | `internal/dataplane/engine` | 已开始请求 pin 住 snapshot、price、route |
| [ ] | M5/E5-T09/P0 | P0 | 实现 API key revocation blacklist | `internal/infra/redis/revocation.go` | revoke 在目标 SLA 内生效 |
| [ ] | M5/E5-T10/P1 | P1 | 实现 rollback 和 staleness metrics | `internal/controlplane/snapshot` | snapshot version/staleness 可观测 |

## M6 Plugins + Security

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | M6/E6-T01/P0 | P0 | 定义 MVP 9 phase enum | `internal/dataplane/plugin/phase.go` | phase 与 ADR 一致 |
| [ ] | M6/E6-T02/P0 | P0 | 实现 PluginManager | `internal/dataplane/plugin/manager.go` | 无绑定 phase O(1) skip |
| [ ] | M6/E6-T03/P0 | P0 | 实现 PluginBinding resolver | `internal/dataplane/plugin/binding_resolver.go` | scope specificity、priority、name 排序正确 |
| [ ] | M6/E6-T04/P0 | P0 | 实现 RequestSizePlugin | `plugin/builtin/request_size.go` | 超限拒绝 |
| [ ] | M6/E6-T05/P0 | P0 | 实现 PromptTokenLimitPlugin | `plugin/builtin/prompt_token_limit.go` | prompt token 超限拒绝 |
| [ ] | M6/E6-T06/P0 | P0 | 实现 PIIRedactionPlugin | `plugin/builtin/pii_redaction.go` | 日志和审计脱敏 |
| [ ] | M6/E6-T07/P0 | P0 | 实现 PromptGuardPlugin | `plugin/builtin/prompt_guard.go` | 命中返回 `policy_denied` |
| [ ] | M6/E6-T08/P1 | P1 | 实现 ResponseGuardPlugin / CostGuardPlugin | `plugin/builtin` | 支持 deny、degrade、audit |
| [ ] | M6/E6-T09/P1 | P1 | 实现 AuditLogPlugin / LLMMetricPlugin | `plugin/builtin` | 审计和指标不含敏感明文 |

## M7 Observability + Performance

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | M7/E7-T01/P0 | P0 | 固化 metrics 命名和 label 规范 | `internal/infra/telemetry/metrics.go` | provider、billing、task、snapshot 指标齐全 |
| [ ] | M7/E7-T02/P0 | P0 | 补齐 OpenTelemetry spans | `internal/dataplane/observe` | 每个关键阶段 span 可见 |
| [ ] | M7/E7-T03/P0 | P0 | 实现统一 redactor | `pkg/redaction` | api key、provider key、prompt、response 脱敏 |
| [ ] | M7/E7-T04/P0 | P0 | 编写 load test | `tools/loadtest` | QPS、stream concurrency、Redis 延迟报告 |
| [ ] | M7/E7-T05/P0 | P0 | 编写 failure drills | `tests/failure` | provider、billing、redis、db、snapshot 场景 |
| [ ] | M7/E7-T06/P1 | P1 | 建立 dashboard 和 alert rules | `deployments/observability` | failed settlement、snapshot stale、provider 429/5xx 有告警 |

## M8 Realtime Reserved Extension

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | M8/E8-T01/P2 | P2 | 定义 RealtimeSession domain | `internal/dataplane/realtime/session.go` | session 类型、状态、过期时间可表达 |
| [ ] | M8/E8-T02/P2 | P2 | 实现 create/get session API | `internal/transport/realtimehttp` | 未启用时返回 501/feature_not_enabled |
| [ ] | M8/E8-T03/P2 | P2 | 定义 RealtimeEngine interface | `internal/dataplane/realtime/engine.go` | 不绑定具体 provider |
| [ ] | M8/E8-T04/P2 | P2 | 实现 WebSocket handler stub | `internal/transport/realtimehttp` | 可编译，有鉴权、审计、metrics 接入点 |

## M9 Commercial Operations

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | M9/E9-T01/P1 | P1 | 实现客户余额和用量报表 | `internal/controlplane/admin` | 客户可查余额、用量和扣费流水 |
| [ ] | M9/E9-T02/P1 | P1 | 实现 provider cost 和利润报表 | `internal/billing/reporting` | 运营可查渠道成本和模型利润 |
| [ ] | M9/E9-T03/P1 | P1 | 实现 reconciliation report | `internal/billing/reconciliation.go` | 每日对账能发现差异 |
| [ ] | M9/E9-T04/P1 | P1 | 实现 manual adjustment | `internal/billing` | 人工调账幂等且有强审计 |
| [ ] | M9/E9-T05/P2 | P2 | 实现模型市场配置 | control API | 租户可见模型可配置 |
| [ ] | M9/E9-T06/P2 | P2 | 实现 Agent metadata 报表 | reporting 或 analytics | workflow、scene、shot 维度可分析 |
| [ ] | M9/E9-T07/P2 | P2 | 建立 backup/restore runbook | `docs/runbook` | 恢复演练通过 |

## 阶段验收总览

| 阶段 | 验收标准 |
|---|---|
| M0 | `go test ./...`、`make lint`、healthz、readyz、metrics、OpenAPI、ADR 和文档入口通过 |
| M1 | `/v1/chat/completions` non-stream 可认证、路由、调用 provider、返回标准响应并记录基础观测 |
| M2 | provider 成功后的本地结算失败可修复，ledger 与 balance 可对账 |
| M3 | OpenAI stream、Claude、Gemini 和 stream close-time accounting 可用 |
| M4 | 统一媒体任务可创建、幂等查询、轮询、回调和最终结算 |
| M5 | 新增模型、渠道、价格、路由和限流无需重启 gateway |
| M6 | 插件可按 scope 绑定并执行 deny、redact、audit 和 metrics 行为 |
| M7 | dashboard、alert、压测和 failure drills 可支撑灰度商用 |
| M8 | Realtime session API 和 WebSocket stub 可编译，未启用时明确返回 501 |
| M9 | 客户、运营和财务能围绕余额、用量、成本、利润、对账和灾备开展运营 |
