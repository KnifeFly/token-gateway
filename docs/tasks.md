# 商用 AI Gateway 执行任务清单

本文是最终设计包的唯一执行看板，综合了 v0.2 的细粒度商用任务和 v0.3 的架构修订。实现时只维护本文件的任务状态，不再在 `docs/design` 下维护第二份 task list。

## 当前状态

- 已完成：v2/v3 设计包归一为最终版文档；M0 Go 工程骨架、配置、错误、日志、HTTP server、metrics、DB/Redis client、migration、Makefile、compose 和 CI 已落地；M1 `/v1/chat/completions` 非流式数据面已跑通；M2 账务闭环已落地 balance account/hold、usage attempt、usage record、ledger、failed settlement replay、Redis limit 和 reconciliation；M3 已扩展 OpenAI stream/Responses/Embeddings、Claude Messages、Gemini GenerateContent/streamGenerateContent、ProviderStream/AccountingStream、SSE writer、StreamFinalizer 和 downstream disconnect 分类；M4 已落地 Unified Media async task、Task/File domain、Idempotency-Key、TaskBridge、provider task poller、callback outbox、task settlement 和 file service；M5 已落地 control API、credential encryption、snapshot build/validate/publish/watch/rollback、request pinned price、Redis revocation 和 snapshot metrics/header；M6 已落地 snapshot 驱动 plugin chain、9 个 MVP phase、内置安全/审计/指标插件和 `policy_denied` 映射；M7 已落地 metrics/tracing/redaction/load test/failure drills/dashboard/alert rules/性能预算；M8 已落地 Realtime reserved extension，未启用时稳定返回 501/`feature_not_enabled`，完整 Realtime 不进入当前路线；M9 已落地商用运营报表、对账、人工调账、模型市场配置、Agent metadata report、OpenAPI 管理接口和 backup/restore runbook；P0 已补齐 worker、异步任务、公开 API、snapshot stale policy、emergency disable 和 focused tests；P1 已补齐多维 limit rule runtime index、Redis Lua 多维限流、local deny cache、routing strategy registry、RouteSignals、显式 policy decision stage、model catalog/schema/alias/provider mapping 和 focused tests；P2 已补齐独立 configd、snapshot publish/rollback/diagnostics、IP allowlist、Model ACL、RouteOverride、Callback、CostGuard decision、classifier registry hint/body schema inference、`ambiguous_protocol` 测试和 Realtime disabled contract 边界；P3 已补齐 Redis token bucket/TPM 预扣、统一 billability policy、Native OpenAI images/audio adapter、Unified Media provider adapter contract、Redis RouteSignals、configd 分发 smoke 和生产验收文档。
- 进行中：P4 发布候选与商用上线验收；P4/E14-T01 干净依赖环境 RC smoke、P4/E14-T02 worker 运营 job、P4/E14-T03 真实 provider release channel、P4/E14-T04 configd 生产分发、P4/E14-T05 OpenAPI 管理面合同、P4/E14-T06 发布级观测与安全验收已完成。
- 阻塞：无。
- 下一步建议：继续 P4/E14-T07，固化 staging 灰度上线 runbook，然后做最终全量验证和 push。
- 最近验证：2026-05-30 `bash tests/rc/clean_env_smoke.sh` 使用独立 Docker compose project/volume 和自动避让端口跑通 MySQL、Redis、migration、gateway、control-api、configd、worker、health/ready、Redis active snapshot key、snapshot publish/watch/rollback、gateway chat 和 metrics，输出 `rc_smoke=passed`；`tests/failure/release_gate.sh` 通过；`go test ./internal/controlplane/snapshot ./internal/dataplane/snapshot ./internal/snapshotdist ./internal/bootstrap` 通过；`go test ./internal/transport/controlhttp` 通过；`go test ./internal/billing ./internal/worker ./internal/worker/jobs ./internal/bootstrap` 通过；`go test ./internal/provider/replicate ./internal/task ./internal/bootstrap` 通过；此前 `go test ./...`、`make lint`、`make build`、Redis 集成、failure drills 和 load test 均通过。

## 使用规则

- M0-M9 任务 ID 采用 `M{milestone}/E{epic}-T{number}/P{priority}` 格式。
- P0-P4 设计差距补齐、商用硬化和发布验收任务 ID 采用 `P{phase}/E{epic}-T{number}/P{priority}` 格式。
- 第一轮优先完成 M0-M3 的 P0 任务，形成最小商用内核。
- M4-M9 只拆到可执行粒度，避免早期过度展开控制面、插件和运营后台。
- P0-P4 是 M0-M9 之后的设计差距补齐、商用硬化和发布候选验收阶段，执行顺序固定为 P0、P1、P2、P3、P4。
- 完整 Realtime 不进入当前路线；M8/P2 只维护 disabled contract、session 预留和 WebSocket stub。
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
| [x] | M5/E5-T01/P0 | P0 | 实现 admin auth | `internal/transport/controlhttp` | admin token 或 RBAC 可用 |
| [x] | M5/E5-T02/P0 | P0 | 实现 tenant/project/api key CRUD | `internal/controlplane/admin` | key create、disable、list 可用 |
| [x] | M5/E5-T03/P0 | P0 | 实现 model/schema/alias CRUD | `internal/controlplane/admin` | 新增模型无需重启 gateway |
| [x] | M5/E5-T04/P0 | P0 | 实现 provider/channel/credential CRUD | `internal/controlplane/admin` | credential 加密，不进 snapshot 明文 |
| [x] | M5/E5-T05/P0 | P0 | 实现 route/price/limit CRUD | `internal/controlplane/admin` | route、price、limit 可配置 |
| [x] | M5/E5-T06/P0 | P0 | 实现 snapshot builder/validator | `internal/controlplane/snapshot` | 坏配置拒绝发布 |
| [x] | M5/E5-T07/P0 | P0 | 实现 snapshot publisher/watcher | `internal/controlplane/snapshot`, `internal/dataplane/snapshot` | gateway 原子切换 snapshot |
| [x] | M5/E5-T08/P0 | P0 | 实现 request-level pinning | `internal/dataplane/engine` | 已开始请求 pin 住 snapshot、price、route |
| [x] | M5/E5-T09/P0 | P0 | 实现 API key revocation blacklist | `internal/infra/redis/revocation.go` | revoke 在目标 SLA 内生效 |
| [x] | M5/E5-T10/P1 | P1 | 实现 rollback 和 staleness metrics | `internal/controlplane/snapshot` | snapshot version/staleness 可观测 |

## M6 Plugins + Security

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M6/E6-T01/P0 | P0 | 定义 MVP 9 phase enum | `internal/dataplane/plugin/phase.go` | phase 与 ADR 一致 |
| [x] | M6/E6-T02/P0 | P0 | 实现 PluginManager | `internal/dataplane/plugin/manager.go` | 无绑定 phase O(1) skip |
| [x] | M6/E6-T03/P0 | P0 | 实现 PluginBinding resolver | `internal/dataplane/plugin/binding_resolver.go` | scope specificity、priority、name 排序正确 |
| [x] | M6/E6-T04/P0 | P0 | 实现 RequestSizePlugin | `plugin/builtin/request_size.go` | 超限拒绝 |
| [x] | M6/E6-T05/P0 | P0 | 实现 PromptTokenLimitPlugin | `plugin/builtin/prompt_token_limit.go` | prompt token 超限拒绝 |
| [x] | M6/E6-T06/P0 | P0 | 实现 PIIRedactionPlugin | `plugin/builtin/pii_redaction.go` | 日志和审计脱敏 |
| [x] | M6/E6-T07/P0 | P0 | 实现 PromptGuardPlugin | `plugin/builtin/prompt_guard.go` | 命中返回 `policy_denied` |
| [x] | M6/E6-T08/P1 | P1 | 实现 ResponseGuardPlugin / CostGuardPlugin | `plugin/builtin` | 支持 deny、degrade、audit |
| [x] | M6/E6-T09/P1 | P1 | 实现 AuditLogPlugin / LLMMetricPlugin | `plugin/builtin` | 审计和指标不含敏感明文 |

## M7 Observability + Performance

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M7/E7-T01/P0 | P0 | 固化 metrics 命名和 label 规范 | `internal/infra/telemetry/metrics.go` | provider、billing、task、snapshot 指标齐全 |
| [x] | M7/E7-T02/P0 | P0 | 补齐 OpenTelemetry spans | `internal/dataplane/observe` | 每个关键阶段 span 可见 |
| [x] | M7/E7-T03/P0 | P0 | 实现统一 redactor | `pkg/redaction` | api key、provider key、prompt、response 脱敏 |
| [x] | M7/E7-T04/P0 | P0 | 编写 load test | `tools/loadtest` | QPS、stream concurrency、Redis 延迟报告 |
| [x] | M7/E7-T05/P0 | P0 | 编写 failure drills | `tests/failure` | provider、billing、redis、db、snapshot 场景 |
| [x] | M7/E7-T06/P1 | P1 | 建立 dashboard 和 alert rules | `deployments/observability` | failed settlement、snapshot stale、provider 429/5xx 有告警 |

## M8 Realtime Reserved Extension

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M8/E8-T01/P2 | P2 | 定义 RealtimeSession domain | `internal/dataplane/realtime/session.go` | session 类型、状态、过期时间可表达 |
| [x] | M8/E8-T02/P2 | P2 | 实现 create/get session API | `internal/transport/realtimehttp` | 未启用时返回 501/feature_not_enabled |
| [x] | M8/E8-T03/P2 | P2 | 定义 RealtimeEngine interface | `internal/dataplane/realtime/engine.go` | 不绑定具体 provider |
| [x] | M8/E8-T04/P2 | P2 | 实现 WebSocket handler stub | `internal/transport/realtimehttp` | 可编译，有鉴权、审计、metrics 接入点 |

## M9 Commercial Operations

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M9/E9-T01/P1 | P1 | 实现客户余额和用量报表 | `internal/controlplane/admin` | 客户可查余额、用量和扣费流水 |
| [x] | M9/E9-T02/P1 | P1 | 实现 provider cost 和利润报表 | `internal/billing/reporting` | 运营可查渠道成本和模型利润 |
| [x] | M9/E9-T03/P1 | P1 | 实现 reconciliation report | `internal/billing/reconciliation.go` | 每日对账能发现差异 |
| [x] | M9/E9-T04/P1 | P1 | 实现 manual adjustment | `internal/billing` | 人工调账幂等且有强审计 |
| [x] | M9/E9-T05/P2 | P2 | 实现模型市场配置 | control API | 租户可见模型可配置 |
| [x] | M9/E9-T06/P2 | P2 | 实现 Agent metadata 报表 | reporting 或 analytics | workflow、scene、shot 维度可分析 |
| [x] | M9/E9-T07/P2 | P2 | 建立 backup/restore runbook | `docs/runbook` | 恢复演练通过 |

## P0 Production Closure

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P0/E10-T01/P0 | P0 | 实现可运行 worker 进程入口 | `cmd/worker`, `internal/bootstrap` | worker 可启动、优雅退出，并接入 config、DB、Redis、logger、metrics、tracing |
| [x] | P0/E10-T02/P0 | P0 | 实现通用 job runner | `internal/worker` | 支持 lease、并发控制、防重复执行、retry/backoff、panic recovery 和 shutdown |
| [x] | P0/E10-T03/P0 | P0 | 接入 provider task polling | `internal/worker/jobs`, `internal/task` | 多 worker 下同一 provider task 不重复推进，状态可从 queued/running 到终态 |
| [x] | P0/E10-T04/P0 | P0 | 接入 failed settlement replay | `internal/worker/jobs`, `internal/billing` | 失败结算可自动重放，且 replay 幂等、不重复扣费 |
| [x] | P0/E10-T05/P0 | P0 | 接入 callback dispatcher | `internal/worker/jobs`, `internal/task` | callback outbox 可重试，失败原因和最终状态可追踪 |
| [x] | P0/E10-T06/P0 | P0 | 实现真实异步 provider task adapter | `internal/provider`, `internal/task` | submit、poll、cancel 可调用真实 provider；mock 仅用于测试 |
| [x] | P0/E10-T07/P0 | P0 | 打通 async media task 生产闭环 | `internal/dataplane/engine`, `internal/task`, `internal/billing` | task 从 API 创建到 provider 轮询、结果回写、callback、settlement 全链路通过 |
| [x] | P0/E10-T08/P0 | P0 | 补齐 `/v1/models` 和模型 schema API | `internal/transport/httpserver`, model catalog 读模型 | `/v1/models`、`/v1/models/{model}/schema` 与 OpenAPI 对齐并有 HTTP 测试 |
| [x] | P0/E10-T09/P0 | P0 | 补齐 `/v1/credits` API | `internal/transport/httpserver`, `internal/billing` | 客户可查询余额/credits，响应不泄露内部账务实现 |
| [x] | P0/E10-T10/P0 | P0 | 补齐 `/v1/moderations` API | `internal/transport/httpserver`, `internal/dataplane` | moderation API 与 OpenAPI 对齐，鉴权、审计、错误映射可用 |
| [x] | P0/E10-T11/P0 | P0 | 实现 snapshot stale policy | `internal/dataplane/snapshot`, `internal/dataplane/engine` | soft stale 告警，hard stale fail-close 或按明确策略拒绝请求 |
| [x] | P0/E10-T12/P0 | P0 | 实现 emergency provider/channel disable | `internal/infra/redis`, `internal/dataplane/router`, `internal/dataplane/dispatch` | 禁用后无需重启 gateway，新请求不再选中对应 provider/channel |
| [x] | P0/E10-T13/P1 | P1 | 补齐 P0 进程级和集成测试 | `tests/`, focused package tests | worker、async task、公开 API、snapshot stale、emergency disable 测试通过 |

## P1 Design Capabilities

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P1/E11-T01/P0 | P0 | 设计并实现 limit rule 运行时索引 | `internal/controlplane/snapshot`, `internal/dataplane/limit` | tenant、project、api key、model、provider、channel 六个维度可组合读取 |
| [x] | P1/E11-T02/P0 | P0 | 实现 Redis Lua 多维限流 | `internal/dataplane/limit`, `internal/infra/redis` | RPM/QPS、TPM、concurrency、daily budget、cost-per-minute 原子判断 |
| [x] | P1/E11-T03/P0 | P0 | 实现 local deny cache | `internal/dataplane/limit` | 拒绝结果可短期缓存，命中时不访问 Redis，过期后恢复 Redis 判断 |
| [x] | P1/E11-T04/P0 | P0 | 抽象 routing strategy registry | `internal/dataplane/router` | priority/weighted 作为默认策略保留兼容 |
| [x] | P1/E11-T05/P0 | P0 | 引入 `RouteSignals` | `internal/dataplane/router`, `internal/dataplane/observe` | 路由可统一读取健康、延迟、成本、额度、模型兼容性和禁用状态 |
| [x] | P1/E11-T06/P0 | P0 | 实现 health/cost/latency/quota 策略 | `internal/dataplane/router` | health weighted、least cost、least latency、quota aware 均有确定性测试 |
| [x] | P1/E11-T07/P0 | P0 | 在主链路加入显式 policy stage | `internal/dataplane/engine`, `internal/dataplane/policy` | GatewayEngine 清晰表达 auth、classify、policy、route、dispatch、settlement |
| [x] | P1/E11-T08/P0 | P0 | 实现 policy decision 输出 | `internal/dataplane/policy` | 支持 allow、deny、degrade、route override，并被主链路消费 |
| [x] | P1/E11-T09/P0 | P0 | 建立 model catalog 统一事实源 | `internal/controlplane/admin`, `internal/controlplane/snapshot` | 模型列表、schema、alias、ACL、provider mapping 由同一数据源发布 |
| [x] | P1/E11-T10/P1 | P1 | 将公开模型 API 切到 model catalog | `internal/transport/httpserver` | `/v1/models` 和 `/v1/models/{model}/schema` 返回不再来自散落配置 |
| [x] | P1/E11-T11/P1 | P1 | 补齐 P1 行为测试 | focused package tests, Redis integration tests | Redis、routing、policy、model catalog 测试通过 |

## P2 Architecture Advanced

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P2/E12-T01/P1 | P1 | 实现独立 `cmd/configd` | `cmd/configd`, `internal/bootstrap` | configd 可独立启动并接入 DB、Redis、logger、metrics、tracing |
| [x] | P2/E12-T02/P1 | P1 | 迁移 snapshot build/validate/publish 职责 | `internal/controlplane/snapshot`, `cmd/configd` | configd 负责发布 active snapshot，坏配置不切换版本 |
| [x] | P2/E12-T03/P1 | P1 | 实现 snapshot watch/rollback/diagnostics | `cmd/configd`, `internal/dataplane/snapshot` | gateway 可热更新，rollback 和 staleness 可观测 |
| [x] | P2/E12-T04/P1 | P1 | 补齐 IP allowlist plugin | `internal/dataplane/plugin/builtin` | 命中允许/拒绝路径有测试，敏感 IP 不进入不安全 label |
| [x] | P2/E12-T05/P1 | P1 | 补齐 Model ACL plugin | `internal/dataplane/plugin/builtin` | 模型权限可通过插件链阻断，且不绕过核心 policy |
| [x] | P2/E12-T06/P1 | P1 | 补齐 RouteOverride plugin | `internal/dataplane/plugin/builtin`, `internal/dataplane/router` | 插件可输出受约束 route decision，不绕过 billing 或 ACL |
| [x] | P2/E12-T07/P1 | P1 | 补齐 Callback plugin | `internal/dataplane/plugin/builtin`, `internal/task` | callback 行为可按 scope 配置并进入 outbox |
| [x] | P2/E12-T08/P1 | P1 | 升级 CostGuard decision | `internal/dataplane/plugin/builtin`, `internal/dataplane/policy` | CostGuard 可触发 deny、degrade 或 route decision |
| [x] | P2/E12-T09/P1 | P1 | 增强 classifier registry hint | `internal/dataplane/classifier` | 协议判断可读取 model registry hint |
| [x] | P2/E12-T10/P1 | P1 | 增强 classifier body schema inference | `internal/dataplane/classifier`, `internal/dataplane/parser` | native/unified 重叠请求可通过 schema 消歧 |
| [x] | P2/E12-T11/P1 | P1 | 补齐 `ambiguous_protocol` 场景测试 | `internal/dataplane/classifier`, `internal/dataplane/engine` | 无法判断协议时稳定返回 `ambiguous_protocol` |
| [x] | P2/E12-T12/P2 | P2 | 固化 Realtime disabled contract | `internal/dataplane/realtime`, `internal/transport/realtimehttp` | 未启用时 session API 和 WebSocket stub 稳定返回 501/feature_not_enabled |
| [x] | P2/E12-T13/P2 | P2 | 固化完整 Realtime 不进入当前路线 | `docs/plan`, `docs/tasks.md` | 保留 disabled contract、session 预留和 WebSocket stub，不再规划 WebSocket/WebRTC、session memory、双向音视频、provider realtime adapter 或 realtime billing |

## P3 Production Hardening

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P3/E13-T01/P0 | P0 | 修正 Redis 限流算法语义 | `internal/dataplane/limit`, `internal/infra/redis` | QPS/RPM token bucket、TPM estimated pre-charge、concurrency lease、daily budget 和 cost-per-minute 在 Lua 中原子判断 |
| [x] | P3/E13-T02/P0 | P0 | 引入统一 billability policy | `internal/billing`, `internal/dataplane/stream`, `internal/task` | 无有效输出不计费，部分输出、客户端断开、provider error、任务取消和任务成功都有确定性计费判定 |
| [x] | P3/E13-T03/P0 | P0 | 补齐 Native OpenAI media adapter | `internal/provider/openai`, `internal/dataplane/parser`, `internal/transport/httpserver` | `/v1/images/*`、`/v1/audio/*` native compatible 路径有转发、模型改写、错误映射和 SDK/HTTP 测试 |
| [x] | P3/E13-T04/P1 | P1 | 建立 Unified Media provider adapter contract | `internal/provider`, `internal/task` | image/video/audio/music submit、poll、cancel 有供应商字段映射，mock 只作为测试辅助 |
| [x] | P3/E13-T05/P1 | P1 | 接入真实 RouteSignals 数据源 | `internal/dataplane/router`, `internal/dataplane/observe`, `internal/infra/redis` | health/cost/latency/quota 策略读取真实信号，信号缺失时安全退回 priority/weighted |
| [x] | P3/E13-T06/P1 | P1 | 强化 configd snapshot 分发验证 | `cmd/configd`, `internal/controlplane/snapshot`, `internal/dataplane/snapshot` | publish、watch、rollback、diagnostics、configd 重启和 gateway stale policy 有进程级 smoke |
| [x] | P3/E13-T07/P1 | P1 | 完成生产集成验收与文档校准 | `docs/design`, `docs/runbook`, `tests/`, `deployments/` | OpenAPI/runbook/ADR 与实现一致，Redis integration、compose smoke、load test 和 failure drills 可复现 |

## P4 Release Candidate Readiness

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P4/E14-T01/P0 | P0 | 跑通干净依赖环境 RC smoke | `tests/rc/clean_env_smoke.sh`, `configs`, `cmd/*`, `migrations`, `docs/runbook` | 独立 project/volume 或 staging 下 MySQL、Redis、gateway、control-api、configd、worker、migration、health/ready、snapshot publish/watch/rollback 全部通过 |
| [x] | P4/E14-T02/P0 | P0 | 补齐 worker 生产运营 job | `internal/worker/jobs`, `internal/billing`, `internal/task`, `internal/bootstrap/worker.go` | failed settlement replay、provider task poller、callback dispatcher、balance hold reaper、reconciliation scheduled job 均有 lease、metrics、重试和 focused tests |
| [x] | P4/E14-T03/P0 | P0 | 接入真实 provider release channel | `internal/provider`, `internal/task`, `configs`, `docs/runbook` | OpenAI/Claude/Gemini 真实 channel 最小回归通过，至少一个真实媒体 provider task 完成 submit、poll、result、callback、settlement |
| [x] | P4/E14-T04/P1 | P1 | 生产化 configd snapshot 分发 | `cmd/configd`, `internal/controlplane/snapshot`, `internal/dataplane/snapshot`, `internal/snapshotdist` | Redis active snapshot key/pubsub 或明确 fallback 模式可复现，gateway reload、checksum、rollback、stale policy 和 diagnostics 覆盖 |
| [x] | P4/E14-T05/P1 | P1 | 补齐 OpenAPI 管理面合同 | `docs/design/ai_gateway_openapi.yaml`, `internal/transport/controlhttp`, contract tests | tenant/project/api-key/model/channel/route/price/limit/plugin/snapshot/emergency 管理接口与实现一致，关键请求/响应示例完整 |
| [x] | P4/E14-T06/P1 | P1 | 建立发布级观测与安全验收 | `deployments/observability`, `pkg/redaction`, `tests/failure`, `tools/loadtest` | dashboard、alert、redaction audit、provider attempt、billing backlog、snapshot stale、worker job、Redis latency 和 SLO 预算均可验证 |
| [ ] | P4/E14-T07/P1 | P1 | 固化 staging 灰度上线 runbook | `docs/runbook`, `docs/plan`, `docs/tasks.md` | 发布 checklist 覆盖环境准备、密钥注入、迁移、配置发布、真实请求、压测、故障演练、回滚、数据核对和风险记录 |

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
| P0 | worker、异步任务、公开 API、snapshot stale policy 和 emergency disable 形成生产闭环 |
| P1 | 多维限流、策略路由、显式 policy stage 和 model catalog 达到设计能力要求 |
| P2 | 独立 configd、剩余插件、分类器增强和 Realtime disabled contract 边界与完整架构蓝图一致 |
| P3 | 限流算法语义、账务计费策略、Native media、RouteSignals、configd 分发和生产集成验收达到商用硬化要求 |
| P4 | 干净依赖环境、真实 provider、worker 运营 job、OpenAPI 管理面、观测安全和灰度回滚验收达到 release candidate 要求 |
