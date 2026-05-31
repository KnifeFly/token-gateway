# 商用 AI Gateway 执行任务清单

本文是最终设计包的唯一执行看板，综合了 v0.2 的细粒度商用任务和 v0.3 的架构修订。实现时只维护本文件的任务状态，不再在 `docs/design` 下维护第二份 task list。

## 当前状态

- 已完成：v2/v3 设计包归一为最终版文档；M0 Go 工程骨架、配置、错误、日志、HTTP server、metrics、DB/Redis client、migration、Makefile、compose 和 CI 已落地；M1 `/v1/chat/completions` 非流式数据面已跑通；M2 账务闭环已落地 balance account/hold、usage attempt、usage record、ledger、failed settlement replay、Redis limit 和 reconciliation；M3 已扩展 OpenAI stream/Responses/Embeddings、Claude Messages、Gemini GenerateContent/streamGenerateContent、ProviderStream/AccountingStream、SSE writer、StreamFinalizer 和 downstream disconnect 分类；M4 已落地 Unified Media async task、Task/File domain、Idempotency-Key、TaskBridge、provider task poller、callback outbox、task settlement 和 file service；M5 已落地 control API、credential encryption、snapshot build/validate/publish/watch/rollback、request pinned price、Redis revocation 和 snapshot metrics/header；M6 已落地 snapshot 驱动 plugin chain、9 个 MVP phase、内置安全/审计/指标插件和 `policy_denied` 映射；M7 已落地 metrics/tracing/redaction/load test/failure drills/dashboard/alert rules/性能预算；M8 已落地 Realtime reserved extension，未启用时稳定返回 501/`feature_not_enabled`，完整 Realtime 不进入当前路线；M9 已落地商用运营报表、对账、人工调账、模型市场配置、Agent metadata report、OpenAPI 管理接口和 backup/restore runbook；P0 已补齐 worker、异步任务、公开 API、snapshot stale policy、emergency disable 和 focused tests；P1 已补齐多维 limit rule runtime index、Redis Lua 多维限流、local deny cache、routing strategy registry、RouteSignals、显式 policy decision stage、model catalog/schema/alias/provider mapping 和 focused tests；P2 已补齐独立 configd、snapshot publish/rollback/diagnostics、IP allowlist、Model ACL、RouteOverride、Callback、CostGuard decision、classifier registry hint/body schema inference、`ambiguous_protocol` 测试和 Realtime disabled contract 边界；P3 已补齐 Redis token bucket/TPM 预扣、统一 billability policy、Native OpenAI images/audio adapter、Unified Media provider adapter contract、Redis RouteSignals、configd 分发 smoke 和生产验收文档；P4 已补齐干净依赖环境 RC smoke、worker 运营 job、真实 provider release channel、configd Redis active snapshot 分发、OpenAPI 管理面合同、发布级观测安全 release gate 和 staging 灰度上线 runbook；P5 已补齐 OpenAI/Claude/Gemini 协议兼容矩阵、SDK-compatible HTTP wire shape、stream 生命周期、tool/multimodal passthrough、usage/error normalization、contract tests 和 OpenAPI/runbook 同步；P6 已补齐 provider/channel 健康信号、熔断、retry budget、fallback 限制、provider attempt 追踪和 failure drills；P7 已补齐非存储媒体输入资产语义、media provider result asset contract、Replicate fixture 映射、callback/settlement result URL 衔接和媒体 contract tests；P8 已补齐 Portal 模型/schema、credits、usage、API key 自助管理、task 查询、权限边界测试、OpenAPI contract 和 runbook；P9 已补齐客户接入验收收口、Portal smoke CLI、RC smoke 集成、OpenAPI import preflight 和 customer acceptance runbook；P10 已补齐发布交接收口、release handoff CLI、PR 模板、发布证据字段、回滚说明和 runbook。
- 待执行：P12-P14 review remediation 可靠性收敛；P11 模型分类、复杂价格体系、渠道成本和模型目录运营能力仍待执行，但在 P12 review P0 正确性收敛前不作为下一步入口。控制面 RBAC/审计平台、复杂财务/发票闭环、对象存储、完整 Realtime、生产级 Observability 扩展、WASM/动态插件、New API 分组不进入当前路线，semantic routing/cache 和多地域 active-active 先不做。
- 阻塞：无。
- 下一步建议：从 P12/E22-T01/P0 开始，先修正 streaming concurrency lease 的释放 ownership，并用慢速 stream + Redis lease 回归测试证明 stream close 前 lease 不会提前释放。
- 最近验证：2026-05-31 review remediation 计划文档落库后，`git diff --check` 与 markdown trailing-whitespace scan 通过；本次为 docs-only 变更，未运行代码测试。2026-05-30 P11 计划文档落库后，`git diff --check` 与 markdown trailing-whitespace scan 通过；本次为 docs-only 变更，未运行代码测试；2026-05-30 review hardening 修复 Portal API key 快照刷新、异步任务终态结算、控制面 `enabled=false`、admin token fail-closed 和 fallback per-candidate 限流后，`go test ./...` 与 `go vet ./...` 通过；2026-05-30 P10 `go run ./tools/release-handoff -run-checks -output /tmp/token-gateway-p10-release-handoff.md` 通过，覆盖 `go test ./...`、`go vet ./...`、`go build ./cmd/...`、`go test ./tools/portal-smoke ./tools/release-handoff ./tests/contract`、`bash -n tests/rc/clean_env_smoke.sh` 和 `tests/failure/release_gate.sh`；`git diff --check` 通过；此前 P8 `go test ./internal/portal ./internal/transport/portalhttp ./internal/task ./internal/bootstrap` 通过；此前 P7 `go test ./internal/dataplane/parser ./internal/task ./internal/provider/replicate ./internal/worker/jobs ./tests/contract` 通过；此前 `go test ./internal/dataplane/observe ./internal/dataplane/router ./internal/dataplane/dispatch ./internal/billing ./internal/infra/redis` 通过；`bash -n tests/failure/provider_reliability_drills.sh` 和 `bash -n tests/failure/release_gate.sh` 通过；非 live `tests/failure/release_gate.sh` 通过；此前 `bash tests/rc/clean_env_smoke.sh` 使用独立 Docker compose project/volume 和自动避让端口跑通 MySQL、Redis、migration、gateway、control-api、configd、worker、health/ready、Redis active snapshot key、snapshot publish/watch/rollback、gateway chat 和 metrics，输出 `rc_smoke=passed`；`make lint`、`make build`、Redis 集成、failure drills 和 load test 均通过。

## 使用规则

- M0-M9 任务 ID 采用 `M{milestone}/E{epic}-T{number}/P{priority}` 格式。
- P0-P4 设计差距补齐、商用硬化和发布验收任务 ID 采用 `P{phase}/E{epic}-T{number}/P{priority}` 格式。
- P5-P11 剩余产品能力、验收、发布交接和模型价格目录增强任务 ID 采用 `P{phase}/E{epic}-T{number}/P{priority}` 格式，执行顺序固定为 P5、P6、P7、P8、P9、P10、P11。
- P12-P14 review remediation 任务 ID 采用 `P{phase}/E{epic}-T{number}/P{priority}` 格式，来源于 2026-05-31 静态 review 的 P0/P1/P2 风险分层，执行顺序固定为 P12、P13、P14。
- 第一轮优先完成 M0-M3 的 P0 任务，形成最小商用内核。
- M4-M9 只拆到可执行粒度，避免早期过度展开控制面、插件和运营后台。
- P0-P4 是 M0-M9 之后的设计差距补齐、商用硬化和发布候选验收阶段，执行顺序固定为 P0、P1、P2、P3、P4。
- P12 未验收前，暂停继续新增 P11 这类功能扩展，优先保证账务、stream、异步任务、限流和幂等不变量成立。
- 完整 Realtime 不进入当前路线；M8/P2 只维护 disabled contract、session 预留和 WebSocket stub。
- 文件能力按非存储输入资产处理；gateway 不做对象存储，不承诺媒体对象持久化、下载、生命周期或存储 SLA。
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
| [x] | M4/E4-T04/P0 | P0 | 实现非存储 input asset service | `internal/task/file_service.go` | base64、url、stream 输入可用于请求归一化和 provider 转发，不承诺对象存储 |
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
| [x] | P4/E14-T07/P1 | P1 | 固化 staging 灰度上线 runbook | `docs/runbook`, `docs/plan`, `docs/tasks.md` | 发布 checklist 覆盖环境准备、密钥注入、迁移、配置发布、真实请求、压测、故障演练、回滚、数据核对和风险记录 |

## P5 Provider Protocol Compatibility

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P5/E15-T01/P0 | P0 | 建立 provider 协议兼容矩阵 | `docs/plan/16-p5-provider-protocol-compatibility.md`, `docs/runbook` | OpenAI、Claude、Gemini endpoint 的 request/response、stream、tool、multimodal、usage、error 和 SDK 覆盖状态可追踪 |
| [x] | P5/E15-T02/P0 | P0 | 补齐 OpenAI-compatible SDK wire shape | `internal/dataplane/parser`, `internal/provider/openai` | chat、responses、embeddings、moderations、images、audio 的 JSON/multipart/stream/tool/usage/error 行为与 OpenAI SDK 合同一致 |
| [x] | P5/E15-T03/P0 | P0 | 补齐 Claude Messages 兼容行为 | `internal/dataplane/parser`, `internal/provider/claude` | messages、tools、stream event、multimodal content block、stop reason、usage 和错误映射有合同测试 |
| [x] | P5/E15-T04/P0 | P0 | 补齐 Gemini GenerateContent 兼容行为 | `internal/dataplane/parser`, `internal/provider/gemini` | contents、tools、safetySettings、usageMetadata、finishReason、stream event 和错误映射有合同测试 |
| [x] | P5/E15-T05/P0 | P0 | 统一 provider usage/error normalization | `internal/provider`, `internal/dataplane/dispatch`, `internal/billing` | 归一化 usage 可被结算和报表复用，上游 400/401/403/404/429/5xx/timeout 映射稳定 |
| [x] | P5/E15-T06/P1 | P1 | 增加 SDK/HTTP contract tests | `tests/contract`, provider focused tests | 官方 SDK 最小请求、stream、tool calling、错误响应和协议消歧均有可重复测试 |
| [x] | P5/E15-T07/P1 | P1 | 同步协议兼容 OpenAPI 和运行文档 | `docs/design/ai_gateway_openapi.yaml`, `docs/runbook`, `docs/plan` | OpenAPI 与实际协议行为一致，未支持能力有明确 stable error 或 not planned 标注 |

## P6 Provider Reliability

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P6/E16-T01/P0 | P0 | 建立 provider/channel 健康信号模型 | `internal/dataplane/router`, `internal/dataplane/observe`, `internal/infra/redis` | 成功率、错误率、429/5xx、超时、延迟、stream 中断和手动禁用状态可统一读取 |
| [x] | P6/E16-T02/P0 | P0 | 实现 circuit breaker 状态机 | `internal/dataplane/router`, `internal/provider` | closed、open、half_open 状态可按 provider/channel/model capability 维度进入和恢复 |
| [x] | P6/E16-T03/P0 | P0 | 实现 retry budget 和 retry eligibility | `internal/dataplane/dispatch` | 同一请求的重试次数、时间预算、错误类型和 provider 范围受控，且可观测 |
| [x] | P6/E16-T04/P0 | P0 | 固化 fallback 限制规则 | `internal/dataplane/router`, `internal/dataplane/dispatch`, `internal/dataplane/stream` | 请求可重放、未输出、错误可重试且预算未耗尽时才允许 fallback；stream 已输出后禁止透明 fallback |
| [x] | P6/E16-T05/P1 | P1 | 增强 provider attempt 可追踪记录 | `internal/billing`, `internal/dataplane/observe` | attempt 记录包含原 channel、目标 channel、错误类别、预算消耗、熔断状态和最终结果 |
| [x] | P6/E16-T06/P1 | P1 | 增加 provider failure drills | `tests/failure`, focused integration tests | 429/5xx、timeout、慢响应、坏 JSON、stream 中断、熔断恢复和 emergency disable 组合场景可复现 |

## P7 Media Forwarding Providers

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P7/E17-T01/P0 | P0 | 固化非存储媒体输入语义 | `docs/design`, `docs/plan/18-p7-media-forwarding-providers.md`, OpenAPI | `/v1/files/*` 被定义为 transient/non-storage input asset，不承诺对象存储、下载或生命周期 SLA |
| [x] | P7/E17-T02/P0 | P0 | 调整文件和媒体 parser 的 transient metadata 语义 | `internal/dataplane/parser`, `internal/task` | URL、base64、multipart 只落必要 metadata、hash、size 和 source，不生成长期可下载对象承诺 |
| [x] | P7/E17-T03/P0 | P0 | 建立 media provider adapter contract | `internal/provider`, `internal/task` | image/video/audio/music 的 submit、poll、cancel、result URL、usage、error 和 metadata 映射统一 |
| [x] | P7/E17-T04/P0 | P0 | 补齐关键真实媒体 provider 映射 | `internal/provider`, `configs`, `docs/runbook` | 至少一个 image 或 video provider 的 submit、poll、cancel、result 和错误状态有真实或 fixture 测试 |
| [x] | P7/E17-T05/P1 | P1 | 打通 result URL、callback 和 settlement 衔接 | `internal/task`, `internal/billing`, `internal/worker/jobs` | provider result URL 和 usage 进入 task response、callback 和最终结算 |
| [x] | P7/E17-T06/P1 | P1 | 增加媒体任务 contract tests | `tests/contract`, provider focused tests | submit、poll running/succeeded/failed、cancel、callback、usage、idempotency 和计费衔接可验证 |

## P8 Portal API

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P8/E18-T01/P0 | P0 | 补齐 `/v1/portal/*` OpenAPI 合同 | `docs/design/ai_gateway_openapi.yaml` | Portal tag、models、schema、credits、usage、api-keys、tasks paths、schemas、security 和错误响应完整 |
| [x] | P8/E18-T02/P0 | P0 | 设计 Portal handler/service 和 API key 鉴权复用 | `internal/transport/portalhttp`, `internal/portal` | Portal 复用现有 API key principal，不引入 RBAC，不暴露 admin/control 配置能力 |
| [x] | P8/E18-T03/P0 | P0 | 实现 portal 模型、schema、credits 和 usage 查询 | `internal/portal`, `internal/billing/reporting` | 返回结果按当前 tenant/project/api key scope 过滤，不暴露 provider cost 或 repair internals |
| [x] | P8/E18-T04/P0 | P0 | 实现 portal API key 自助管理 | `internal/portal`, `internal/controlplane/admin` | 派生 key 只能继承当前 tenant/project 和 allowed_models 子集，不能查看历史 plaintext key |
| [x] | P8/E18-T05/P1 | P1 | 实现 portal task 列表和详情查询 | `internal/portal`, `internal/task` | 只能查询当前 tenant/project 下 task，隐藏 provider secret、内部错误和敏感 metadata |
| [x] | P8/E18-T06/P1 | P1 | 增加 portal 权限边界和 contract tests | `internal/transport/portalhttp`, `tests/contract` | 跨 tenant/project、扩大模型权限、禁用 key、revoked key 和标准错误响应均有测试 |

## P9 Customer Acceptance Closure

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P9/E19-T01/P0 | P0 | 固化 P9 客户接入验收范围 | `docs/plan/20-p9-customer-acceptance.md`, `docs/design` | 明确 P9 只做验收收口，不新增 RBAC、invoice、对象存储、Realtime、动态插件或多地域能力 |
| [x] | P9/E19-T02/P0 | P0 | 实现 Portal customer smoke CLI | `tools/portal-smoke`, `Makefile` | 使用 customer Bearer API key 验证 models、schema、credits、usage、api-keys 和 tasks，默认不依赖 admin token |
| [x] | P9/E19-T03/P0 | P0 | 接入 RC Portal customer acceptance | `tests/rc/clean_env_smoke.sh` | 干净依赖环境 snapshot 发布后自动执行 `portal_smoke=passed`，并覆盖派生 key 创建和 disable 生命周期 |
| [x] | P9/E19-T04/P1 | P1 | 增加 OpenAPI import preflight | `tests/contract/openapi_import_contract_test.go` | OpenAPI YAML 可解析、本地 `$ref` 可解析、operationId 唯一、Portal 继承或声明 `bearerAuth` |
| [x] | P9/E19-T05/P1 | P1 | 编写客户接入验收 runbook | `docs/runbook/customer-acceptance.md`, `docs/runbook/portal-api.md` | runbook 给出本地、staging/RC、OpenAPI preflight 和证据收口命令 |

## P10 Release Handoff Closure

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P10/E20-T01/P0 | P0 | 固化 P10 发布交接范围 | `docs/plan/21-p10-release-handoff.md`, `docs/design` | 明确 P10 只做 release/PR 收口，不新增 public API、RBAC、invoice、对象存储、Realtime、动态插件或多地域能力 |
| [x] | P10/E20-T02/P0 | P0 | 实现 release handoff CLI | `tools/release-handoff`, `Makefile` | 可生成当前 branch、commit、migration、验证命令、客户验收、发布字段和 rollback 的 Markdown 交接文档 |
| [x] | P10/E20-T03/P0 | P0 | 增加 release handoff check | `tools/release-handoff` | `-run-checks` 可执行本地 release verification 并对输出做 secret redaction |
| [x] | P10/E20-T04/P1 | P1 | 增加 PR 模板 | `.github/pull_request_template.md` | PR 模板覆盖 scope、validation、customer acceptance、release notes 和范围边界 |
| [x] | P10/E20-T05/P1 | P1 | 编写发布交接 runbook | `docs/runbook/release-handoff.md`, `docs/runbook/staging-rollout.md` | runbook 固定 handoff 生成、RC/staging 证据、PR 填写和 rollback 检查 |

## P11 Model Pricing Catalog

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | P11/E21-T01/P0 | P0 | 定义模型 category 枚举和默认模板 | `internal/controlplane/admin`, `internal/controlplane/snapshot`, `docs/plan/22-p11-model-pricing-catalog.md` | chat、embedding、rerank、image、video、audio_speech、audio_transcription、music、moderation、realtime_reserved 可表达，并能进入 snapshot |
| [ ] | P11/E21-T02/P0 | P0 | 实现分类价格模板和单位校验 | `internal/domain/pricing`, `internal/controlplane/admin` | 每个 category 只允许配置受支持的价格单位，非法单位发布前被拒绝 |
| [ ] | P11/E21-T03/P0 | P0 | 实现客户售价组件化并兼容旧 token 字段 | `internal/controlplane/admin`, `internal/controlplane/snapshot`, `internal/domain/pricing` | 旧 input/output token 字段可读写，新逻辑统一规范化为 price components |
| [ ] | P11/E21-T04/P0 | P0 | 将 hold、settlement、failed replay、async task settlement 切到统一报价器 | `internal/dataplane/admission`, `internal/billing`, `internal/task` | chat、image、video、audio task 的 hold、ledger、settlement 和 failed replay 金额一致 |
| [ ] | P11/E21-T05/P1 | P1 | 实现 provider cost 组件化并保持与客户售价隔离 | `internal/billing/reporting`, migrations, reporting tests | provider cost 使用同构组件结构，但不参与客户扣费，利润报表可按 provider/channel/model 聚合 |
| [ ] | P11/E21-T06/P1 | P1 | 增强模型目录展示字段和 tags/metadata | `internal/controlplane/admin`, `internal/controlplane/snapshot`, `internal/portal` | 模型 category、tags、modalities、capabilities、status、sort order 和 metadata 可发布并被 Portal/模型列表读取 |
| [ ] | P11/E21-T07/P1 | P1 | 增强渠道模型 metadata、测试状态和成本配置状态 | `internal/controlplane/admin`, `internal/controlplane/snapshot` | public model 到 upstream model 映射保留，渠道模型能力覆盖、测试状态和成本状态可追踪 |
| [ ] | P11/E21-T08/P1 | P1 | 增加渠道测试和上游模型同步 preview 的后台能力 | control-plane service/CLI, provider adapters | 单渠道、单模型、批量测试和上游模型列表 preview 可输出新增、删除、变更、未知 category 和缺价格/成本配置 |
| [ ] | P11/E21-T09/P1 | P1 | 补齐 OpenAPI、runbook、focused tests 和兼容测试 | `docs/design/ai_gateway_openapi.yaml`, `docs/runbook`, focused tests | 复杂价格、模型分类、渠道成本、目录展示、同步 preview 和旧价格字段兼容均可验证 |

## P12 Review P0 Correctness

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | P12/E22-T01/P0 | P0 | 修正 streaming concurrency lease 释放 ownership | `internal/dataplane/engine`, `internal/dataplane/dispatch`, `internal/dataplane/stream` | stream response 返回后 request/provider concurrency lease 仍保留，`AccountingStream.Close()` 或等价 finalizer 后幂等释放 |
| [ ] | P12/E22-T02/P0 | P0 | 增加 stream lease 回归测试 | `internal/dataplane/stream`, `internal/dataplane/limit`, Redis integration tests | 慢速/未结束 stream close 前 Redis lease 存在，close、read error、client disconnect 后 lease 消失 |
| [ ] | P12/E22-T03/P0 | P0 | 修正 async idempotency replay 的 hold 泄漏 | `internal/dataplane/engine`, `internal/task`, `internal/dataplane/admission` | 并发同 `Idempotency-Key` replay 不新增 provider request，不遗留第二个 active hold |
| [ ] | P12/E22-T04/P0 | P0 | 补齐 async terminal submit 结算路径 | `internal/task`, `internal/billing`, `internal/worker/jobs` | provider submit 直接返回 succeeded/failed/canceled 时，先 settlement 或 failed settlement repair，再推进任务终态 |
| [ ] | P12/E22-T05/P0 | P0 | 明确 zero-price/no-hold settlement 语义 | `internal/billing`, `internal/dataplane/admission`, migrations if needed | 免费模型、0 价格规则、non-billable policy 不因空 hold 结算失败，并保留 usage audit 或 0 金额 ledger |
| [ ] | P12/E22-T06/P0 | P0 | 补齐账务不变量真实依赖测试 | `tests/`, `internal/billing`, `internal/task` | 所有 billable request 的 hold 最终 settled、released 或进入 repair，MySQL/Redis 集成测试可重复执行 |

## P13 Review P1 Commercial Hardening

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | P13/E23-T01/P0 | P0 | 为 async task 持久化 price snapshot | `internal/task`, `internal/controlplane/snapshot`, migrations | task 创建后保存 currency、price rule/rate、estimated output 和 route snapshot version，后续价格变更不影响结算 |
| [ ] | P13/E23-T02/P0 | P0 | 拆分 rate limit 与 spend budget 语义 | `internal/dataplane/limit`, `internal/billing`, `docs/runbook` | Redis 预扣作为 admission guard，真实 spend budget 以 settlement/ledger/reconciliation 可解释 |
| [ ] | P13/E23-T03/P0 | P0 | 收敛文件和媒体输入资产策略 | `internal/task`, `internal/dataplane/parser`, `docs/design`, OpenAPI | upload 若支持则真实 streaming spool/reference；若不支持则明确 URL passthrough/inline 边界并拒绝托管语义 |
| [ ] | P13/E23-T04/P0 | P0 | 实现统一 egressguard | `pkg/egressguard` 或 `internal/infra/egress`, callback/file/provider clients | file URL、callback URL 和 provider base URL 禁止访问 private/reserved/link-local/loopback/multicast IP 与 metadata service |
| [ ] | P13/E23-T05/P1 | P1 | 强化 callback 签名、重试和 dead-letter | `internal/task`, `internal/worker/jobs`, `docs/runbook` | callback 带 HMAC 签名，支持 allowlist、超时、retry 上限和可追踪失败终态 |
| [ ] | P13/E23-T06/P1 | P1 | 对齐 classifier 推断顺序 | `internal/dataplane/classifier`, `internal/dataplane/parser` | 按 header、path、model registry、body schema、content-type/accept、ambiguous 顺序执行并有表驱动测试 |
| [ ] | P13/E23-T07/P1 | P1 | 补齐 async dispatcher fallback 能力 | `internal/task`, `internal/dataplane/router`, `internal/dataplane/dispatch` | 第一个 candidate submit 失败且未创建外部任务时，可按 retry/fallback 限制尝试后续 provider/channel |
| [ ] | P13/E23-T08/P1 | P1 | 增加商业边界 focused tests | focused package tests, contract tests | Async price pin、预算语义、egress、classifier 和 async fallback 均有可重复测试 |

## P14 Review P2 Engineering Readiness

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | P14/E24-T01/P1 | P1 | API key hash 迁移到 HMAC-SHA256 | `internal/dataplane/auth`, `internal/controlplane/admin`, migrations/config | 新 key 默认 `hmac-sha256`，旧 SHA-256 在兼容窗口可验证，server secret 缺失时生产 fail-closed |
| [ ] | P14/E24-T02/P1 | P1 | 加固 control plane 静态 token 安全基线 | `internal/transport/controlhttp`, `internal/controlplane/admin` | constant-time compare、body size limit、写 API idempotency、operator audit hook 和网络隔离说明可验证 |
| [ ] | P14/E24-T03/P1 | P1 | 为 stream close settlement 增加 timeout | `internal/dataplane/stream`, `internal/billing` | 客户端断开不取消账务修复，DB 慢或 settlement 失败不会无限阻塞 HTTP goroutine |
| [ ] | P14/E24-T04/P1 | P1 | 实现按 SSE event 边界的 usage parser | `internal/provider/openai`, `internal/dataplane/stream` | usage JSON 跨 read chunk 时仍可解析，`[DONE]` 和多 `data:` 行处理稳定 |
| [ ] | P14/E24-T05/P2 | P2 | 修复 nil metrics panic 风险 | `internal/billing`, `internal/worker/jobs`, bootstrap tests | metrics 缺失时使用 no-op 或构造 fail-fast，测试/复用路径不 panic |
| [ ] | P14/E24-T06/P2 | P2 | 引入 trusted proxy client IP 解析 | `internal/transport/httpserver`, `internal/dataplane/plugin/builtin` | 只有可信代理 CIDR 下才信任 `X-Forwarded-For`/`X-Real-IP`，默认不信任外部 header |
| [ ] | P14/E24-T07/P2 | P2 | 补齐 README 交付入口 | `README.md`, `docs/runbook` | README 覆盖产品定位、本地启动、curl、支持 API、配置、账务、安全、worker、已知限制和生产 checklist |

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
| P5 | OpenAI、Claude、Gemini 的 SDK 兼容、stream、tool calling、multimodal、usage/error 映射和合同测试达到可客户接入要求 |
| P6 | Provider/channel 健康信号、熔断、retry budget、fallback 限制和 failure drills 能隔离上游故障 |
| P7 | 媒体能力以非存储输入资产转发为边界，真实 provider task 生命周期、result URL、callback 和 settlement 可验证 |
| P8 | Portal API 支持模型、schema、credits、usage、API key 自助管理和 task 查询，权限边界清晰 |
| P9 | 客户接入验收可重复执行，Portal smoke、OpenAPI import preflight 和 RC smoke 集成可形成发布证据 |
| P10 | 发布交接可重复生成，PR 模板、release handoff、验证命令、客户验收和 rollback 字段可形成交付证据 |
| P11 | 模型分类、复杂价格、渠道成本、模型目录展示和渠道测试/同步 preview 可验证，客户售价与 provider 成本边界清晰 |
| P12 | Review P0 正确性问题收敛，stream lease、async idempotency、terminal task settlement、zero-price/no-hold settlement 和真实依赖账务不变量可验证 |
| P13 | Review P1 商业账务与安全边界收敛，async price pin、预算语义、transient input asset、egressguard、classifier 和 async fallback 可验证 |
| P14 | Review P2 工程交付与安全基线收敛，API key HMAC、管理面安全基线、stream timeout、SSE parser、trusted proxy 和 README 可验证 |
