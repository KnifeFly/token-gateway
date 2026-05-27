# 商用 AI Gateway 执行任务清单

本文是从 `docs/design` 设计包整理出的执行看板。任务状态默认全部为 `[ ]`，实现时按阶段逐项勾选。

## 使用规则

- 任务 ID 采用 `M0/E0-T01/P0` 格式，保持阶段、Epic、原始任务号和优先级稳定。
- 第一轮优先完成 `E0` 到 `E10` 的 P0 任务，形成最小商用内核。
- M4-M8 保留可执行的 P1/P2 粗粒度任务，避免在早期过度拆分。
- 每个任务完成时必须满足阶段通用 Definition of Done：代码、测试、错误码、metrics、trace、日志、审计、配置、OpenAPI、文档和故障场景说明按影响范围更新。

## 第一轮 P0 任务

| 状态 | ID | 阶段 | 优先级 | 任务 | 目标文件或模块 | 依赖 | 验收标准 |
|---|---|---|---|---|---|---|---|
| [ ] | M0/E0-T01/P0 | M0 | P0 | 初始化 Go module 和目录结构 | `cmd/`, `internal/`, `pkg/` | 无 | `go test ./...` 可运行 |
| [ ] | M0/E0-T02/P0 | M0 | P0 | 实现 YAML 配置加载和 ENV override | `internal/infra/conf/config.go` | E0-T01 | 默认配置、文件配置、ENV 覆盖测试通过 |
| [ ] | M0/E0-T03/P0 | M0 | P0 | 实现统一错误包 | `pkg/apperr/errors.go` | E0-T01 | 支持 code、status、type、retryable、safe |
| [ ] | M0/E0-T04/P0 | M0 | P0 | 接入结构化日志 | `internal/infra/log/logger.go` | E0-T02 | request_id 和 trace_id 字段可输出 |
| [ ] | M0/E0-T05/P0 | M0 | P0 | 建立 HTTP server 基础 | `internal/transport/httpserver` | E0-T02,E0-T04 | healthz、readyz、metrics 可访问 |
| [ ] | M0/E0-T06/P0 | M0 | P0 | 初始化 OpenTelemetry | `internal/infra/tracing` | E0-T02 | trace exporter 可配置关闭和开启 |
| [ ] | M0/E0-T07/P0 | M0 | P0 | 初始化 Prometheus metrics | `internal/infra/metrics` | E0-T05 | `/metrics` 输出标准格式 |
| [ ] | M0/E0-T08/P0 | M0 | P0 | 初始化 DB | `internal/infra/db` | E0-T02 | 支持 ping、pool config、close |
| [ ] | M0/E0-T09/P0 | M0 | P0 | 初始化 Redis | `internal/infra/redis` | E0-T02 | 支持 ping、close、db 选择 |
| [ ] | M0/E0-T10/P0 | M0 | P0 | 建立 migration 框架 | `migrations/`, `internal/infra/db/migrate.go` | E0-T08 | 本地能 migrate up/down |
| [ ] | M0/E0-T11/P0 | M0 | P0 | 补齐 Makefile 开发入口 | `Makefile` | E0-T01 | 提供 test、lint、race、build、run |
| [ ] | M0/E0-T12/P0 | M0 | P0 | 建立 CI | `.github/workflows/ci.yml` | E0-T11 | PR 触发 test、race、lint |
| [ ] | M0/E1-T01/P0 | M0 | P0 | 实现 Tenant 和 Account 实体 | `internal/domain/tenant` | E0-T01 | Validate 测试通过 |
| [ ] | M0/E1-T02/P0 | M0 | P0 | 实现 Project 和 APIKey 实体 | `internal/domain/identity` | E1-T01 | key status、expiry、scope 校验通过 |
| [ ] | M0/E1-T03/P0 | M0 | P0 | 实现 API key hash 工具 | `internal/domain/identity/key_hasher.go` | E1-T02 | 不存明文，可校验 |
| [ ] | M0/E1-T04/P0 | M0 | P0 | 实现 PublicModel 和 Capability | `internal/domain/modelcatalog` | E0-T01 | 支持 chat、image、video、audio |
| [ ] | M0/E1-T05/P0 | M0 | P0 | 实现 ModelSchema | `internal/domain/modelcatalog/schema.go` | E1-T04 | JSON schema 可校验 |
| [ ] | M0/E1-T06/P0 | M0 | P0 | 实现 ProviderChannel | `internal/domain/provider/channel.go` | E0-T01 | status、priority、weight 校验通过 |
| [ ] | M0/E1-T07/P0 | M0 | P0 | 实现 ProviderCredentialRef | `internal/domain/provider/credential.go` | E1-T06 | 不暴露明文 |
| [ ] | M0/E1-T08/P0 | M0 | P0 | 实现 ProviderModelMapping | `internal/domain/provider/mapping.go` | E1-T04,E1-T06 | capability 和 stream 校验通过 |
| [ ] | M0/E1-T09/P0 | M0 | P0 | 实现 RoutePolicy | `internal/domain/routing/policy.go` | E1-T08 | retry、fallback、strategy 校验通过 |
| [ ] | M0/E1-T10/P0 | M0 | P0 | 实现 PriceRule | `internal/domain/pricing/price_rule.go` | E1-T04 | 金额使用 micros，不用 float |
| [ ] | M0/E1-T11/P0 | M0 | P0 | 实现 LimitRule | `internal/domain/policy/limit_rule.go` | E1-T02,E1-T04 | scope 和 values 校验通过 |
| [ ] | M0/E1-T12/P0 | M0 | P0 | 实现 Billing entities | `internal/domain/billing` | E1-T10 | hold、attempt、ledger 状态机测试通过 |
| [ ] | M0/E1-T13/P0 | M0 | P0 | 实现 Task entities | `internal/domain/task` | E0-T01 | queued、running、succeeded、failed、canceled 状态有效 |
| [ ] | M0/E1-T14/P0 | M0 | P0 | 实现 Audit entity | `internal/domain/audit` | E1-T01 | actor、action、resource、metadata 可表达 |
| [ ] | M0/E2-T01/P0 | M0 | P0 | 新增 identity tables migration | `migrations/000001_identity.sql` | E0-T10,E1-T01,E1-T02 | accounts、projects、api_keys 表可迁移 |
| [ ] | M0/E2-T02/P0 | M0 | P0 | 新增 model catalog tables | `migrations/000002_models.sql` | E0-T10,E1-T04,E1-T05 | models、aliases、schema 表可迁移 |
| [ ] | M0/E2-T03/P0 | M0 | P0 | 新增 provider tables | `migrations/000003_provider.sql` | E0-T10,E1-T06,E1-T08 | channels、credentials、mappings 表可迁移 |
| [ ] | M0/E2-T04/P0 | M0 | P0 | 新增 routing tables | `migrations/000004_routing.sql` | E0-T10,E1-T09 | policies 和 channels 绑定可迁移 |
| [ ] | M0/E2-T05/P0 | M0 | P0 | 新增 pricing、limit、plugin tables | `migrations/000005_policy.sql` | E0-T10,E1-T10,E1-T11 | price_rules、limit_rules、plugin_bindings 表可迁移 |
| [ ] | M0/E2-T06/P0 | M0 | P0 | 新增 billing tables | `migrations/000006_billing.sql` | E0-T10,E1-T12 | balances、holds、attempts、records、ledger 表可迁移 |
| [ ] | M0/E2-T07/P0 | M0 | P0 | 新增 task 和 file tables | `migrations/000007_tasks_files.sql` | E0-T10,E1-T13 | tasks、files、callback_outbox 表可迁移 |
| [ ] | M0/E2-T08/P0 | M0 | P0 | 新增 audit 和 outbox tables | `migrations/000008_audit_outbox.sql` | E0-T10,E1-T14 | audit_events、outbox_events 表可迁移 |
| [ ] | M0/E2-T09/P0 | M0 | P0 | 实现 Identity repository | `internal/infra/store/sql/identity_repository.go` | E2-T01 | CRUD 和 repository tests 通过 |
| [ ] | M0/E2-T10/P0 | M0 | P0 | 实现 Model repository | `internal/infra/store/sql/model_repository.go` | E2-T02 | 可 list active models |
| [ ] | M0/E2-T11/P0 | M0 | P0 | 实现 Provider repository | `internal/infra/store/sql/provider_repository.go` | E2-T03 | 返回 encrypted credential ref |
| [ ] | M0/E2-T12/P0 | M0 | P0 | 实现 Routing repository | `internal/infra/store/sql/routing_repository.go` | E2-T04,E2-T05 | 可读取 policies 和 bindings |
| [ ] | M0/E2-T13/P0 | M0 | P0 | 实现 Billing repository | `internal/infra/store/sql/billing_repository.go` | E2-T06 | transaction tests 通过 |
| [ ] | M1/E3-T01/P0 | M1 | P0 | 定义 RequestState | `internal/dataplane/engine/state.go` | E1 全部 P0 | 字段覆盖同步、stream、任务和结算生命周期 |
| [ ] | M1/E3-T02/P0 | M1 | P0 | 定义 GatewayEngine 结构体 | `internal/dataplane/engine/engine.go` | E3-T01 | 可注入 snapshot、parser、auth、router、dispatcher 等依赖 |
| [ ] | M1/E3-T03/P0 | M1 | P0 | 实现 APIClassifier | `internal/dataplane/classifier` | E3-T02 | method/path 映射到 canonical API |
| [ ] | M1/E3-T04/P0 | M1 | P0 | 实现 BodyStore | `internal/dataplane/parser/body_store.go` | E3-T03 | 小 body 进内存，大 body 进临时文件 |
| [ ] | M1/E3-T05/P0 | M1 | P0 | 实现 OpenAI chat parser | `internal/dataplane/parser/openai_parser.go` | E3-T04 | 解析 model、messages、stream、usage estimate |
| [ ] | M1/E3-T06/P0 | M1 | P0 | 实现 CredentialExtractor | `internal/dataplane/auth/credential.go` | E3-T03 | Bearer、x-api-key、query key 冲突检测 |
| [ ] | M1/E3-T07/P0 | M1 | P0 | 实现 Authenticator | `internal/dataplane/auth/authenticator.go` | E1-T03,E3-T06,E4-T04 | API key hash 校验通过 |
| [ ] | M1/E3-T08/P0 | M1 | P0 | 实现 PolicyEvaluator | `internal/dataplane/policy/evaluator.go` | E3-T07 | IP、model、scope 校验通过 |
| [ ] | M1/E3-T09/P0 | M1 | P0 | 实现 Engine Handle 主流程 | `internal/dataplane/engine/handle.go` | E3-T02 至 E3-T08 | happy path 单测通过 |
| [ ] | M1/E3-T10/P0 | M1 | P0 | 实现 ErrorMapper | `internal/dataplane/errors/mapper.go` | E0-T03,E3-T03 | OpenAI、Claude、Gemini 错误格式可输出 |
| [ ] | M1/E4-T01/P0 | M1 | P0 | 定义 RuntimeSnapshot struct | `internal/controlplane/snapshot/types.go` | E1 全部 P0 | 包含 api_keys、models、channels、policies |
| [ ] | M1/E4-T02/P0 | M1 | P0 | 实现 Snapshot codec | `internal/controlplane/snapshot/codec.go` | E4-T01 | JSON marshal 和 checksum 测试通过 |
| [ ] | M1/E4-T03/P0 | M1 | P0 | 实现 Snapshot validator | `internal/controlplane/snapshot/validator.go` | E4-T01 | 坏配置被拒绝 |
| [ ] | M1/E4-T04/P0 | M1 | P0 | 实现 IndexedSnapshot | `internal/dataplane/snapshot/index.go` | E4-T01,E4-T03 | key、model、route、price 索引可用 |
| [ ] | M1/E4-T05/P0 | M1 | P0 | 实现 Redis snapshot store | `internal/infra/snapshotstore/redis.go` | E0-T09,E4-T02 | publish、load、watch 可用 |
| [ ] | M1/E4-T06/P0 | M1 | P0 | 实现 Gateway snapshot cache | `internal/dataplane/snapshot/cache.go` | E4-T04,E4-T05 | local atomic pointer 热更新 |
| [ ] | M1/E5-T01/P0 | M1 | P0 | 实现 RoutePlanner | `internal/dataplane/routing/planner.go` | E4-T04 | Plan 输出 RouteDecision |
| [ ] | M1/E5-T02/P0 | M1 | P0 | 实现 PolicyResolver | `internal/dataplane/routing/policy_resolver.go` | E5-T01 | project > account > tenant > global |
| [ ] | M1/E5-T03/P0 | M1 | P0 | 实现 ModelResolver | `internal/dataplane/routing/model_resolver.go` | E4-T04 | alias 可解析到 model |
| [ ] | M1/E5-T04/P0 | M1 | P0 | 实现 CandidateResolver | `internal/dataplane/routing/candidate_resolver.go` | E5-T02,E5-T03 | 过滤 disabled、capability、stream 不匹配候选 |
| [ ] | M1/E5-T05/P0 | M1 | P0 | 实现 StrategyRegistry | `internal/dataplane/routing/strategy_registry.go` | E5-T01 | selector 可注册和查找 |
| [ ] | M1/E5-T06/P0 | M1 | P0 | 实现 PrioritySelector | `internal/dataplane/routing/selectors/priority.go` | E5-T05 | priority stable sort |
| [ ] | M1/E5-T07/P0 | M1 | P0 | 实现 WeightedRandomSelector | `internal/dataplane/routing/selectors/weighted_random.go` | E5-T05 | 无 key 随机，有 key 粘性 |
| [ ] | M1/E6-T01/P0 | M1 | P0 | 定义 Provider relay types | `internal/provider/relay/types.go` | E3-T01 | Request、Response、Stream、Error 类型可表达 |
| [ ] | M1/E6-T02/P0 | M1 | P0 | 实现 Provider registry | `internal/provider/registry.go` | E6-T01 | register 和 get capabilities 可用 |
| [ ] | M1/E6-T03/P0 | M1 | P0 | 实现 OpenAI-compatible adapter | `internal/provider/openai` | E6-T01,E6-T02 | chat completions 可调用 |
| [ ] | M1/E6-T04/P0 | M1 | P0 | 实现带超时 HTTP client | `internal/provider/internal/httpclient` | E6-T03 | non-stream 和 stream timeout 可配置 |
| [ ] | M1/E6-T05/P0 | M1 | P0 | 实现 Provider error mapping | `internal/provider/openai/errors.go` | E6-T03 | 429、5xx、401、400 分类正确 |
| [ ] | M1/E6-T06/P0 | M1 | P0 | 实现 Usage parser | `internal/provider/openai/usage.go` | E6-T03 | input/output tokens 可解析 |
| [ ] | M1/E7-T01/P0 | M1 | P0 | 实现 ProviderDispatcher | `internal/dataplane/dispatch/dispatcher.go` | E5-T01,E6-T02 | 可遍历 candidate |
| [ ] | M1/E7-T02/P0 | M1 | P0 | 实现 AttemptExecutor | `internal/dataplane/dispatch/attempt.go` | E7-T01 | 单次 provider 调用记录 attempt |
| [ ] | M1/E7-T03/P0 | M1 | P0 | 实现 ErrorClassifier | `internal/dataplane/dispatch/error_classifier.go` | E6-T05 | retryable、fallbackable、health_penalty 分类正确 |
| [ ] | M1/E7-T04/P0 | M1 | P0 | 实现 RetryController | `internal/dataplane/dispatch/retry.go` | E7-T03 | max、backoff、jitter 生效 |
| [ ] | M1/E7-T05/P0 | M1 | P0 | 实现 FallbackController | `internal/dataplane/dispatch/fallback.go` | E7-T03 | 按错误类型 fallback |
| [ ] | M2/E8-T01/P0 | M2 | P0 | 实现 PriceEstimator | `internal/dataplane/admission/price_estimator.go` | E1-T10,E4-T04 | 可计算预估费用 |
| [ ] | M2/E8-T02/P0 | M2 | P0 | 实现 AdmissionController | `internal/dataplane/admission/controller.go` | E8-T01,E8-T03 | 可创建 balance_hold |
| [ ] | M2/E8-T03/P0 | M2 | P0 | 实现 Balance service | `internal/billing/balance_service.go` | E2-T13 | 可检查可用余额 |
| [ ] | M2/E8-T04/P0 | M2 | P0 | 实现 Redis token bucket | `internal/dataplane/limit/redis_token_bucket.go` | E0-T09 | QPS 和 TPM 限流可用 |
| [ ] | M2/E8-T05/P0 | M2 | P0 | 实现 Redis concurrency lease | `internal/dataplane/limit/redis_concurrency.go` | E0-T09 | 多副本并发准确 |
| [ ] | M2/E8-T06/P0 | M2 | P0 | 实现 Limit rule matcher | `internal/dataplane/limit/rule_matcher.go` | E4-T04 | 支持 tenant、project、key、model、channel |
| [ ] | M2/E9-T01/P0 | M2 | P0 | 实现 Settlement planner | `internal/billing/settlement_planner.go` | E8-T02 | success、failure、stream plan 可生成 |
| [ ] | M2/E9-T02/P0 | M2 | P0 | 实现 Settlement executor | `internal/billing/settlement_executor.go` | E2-T13,E9-T01 | 事务扣费正确 |
| [ ] | M2/E9-T03/P0 | M2 | P0 | 实现 UsageAttempt writer | `internal/billing/usage_attempt_service.go` | E7-T02,E2-T13 | 每次 provider attempt 有记录 |
| [ ] | M2/E9-T04/P0 | M2 | P0 | 实现 UsageRecord writer | `internal/billing/usage_record_service.go` | E9-T01,E2-T13 | 最终客户用量可记录 |
| [ ] | M2/E9-T05/P0 | M2 | P0 | 实现 Ledger service | `internal/billing/ledger_service.go` | E9-T02 | ledger entry 幂等 |
| [ ] | M2/E9-T06/P0 | M2 | P0 | 实现 FailedSettlement service | `internal/billing/failed_settlement_service.go` | E9-T02 | 失败 settlement 可修复 |
| [ ] | M2/E9-T07/P0 | M2 | P0 | 实现 Settlement replay job | `internal/worker/jobs/failed_settlement_replayer.go` | E9-T06 | 故障后可恢复扣费 |
| [ ] | M3/E10-T01/P0 | M3 | P0 | 定义 ProviderStream interface | `internal/provider/relay/stream.go` | E6-T01 | Read、Close、Usage 可表达 |
| [ ] | M3/E10-T02/P0 | M3 | P0 | 实现 Stream writer | `internal/transport/gatewayhttp/stream_writer.go` | E10-T01 | SSE 和 chunk 写出可用 |
| [ ] | M3/E10-T03/P0 | M3 | P0 | 实现 StreamFinalizer | `internal/dataplane/stream/finalizer.go` | E9-T01,E10-T01,E10-T02 | close-time settlement 可执行 |
| [ ] | M3/E10-T04/P0 | M3 | P0 | 实现 Downstream error report | `internal/dataplane/stream/downstream.go` | E10-T02 | client disconnect 可识别 |
| [ ] | M3/E10-T05/P0 | M3 | P0 | 记录 First token latency | `internal/dataplane/stream/metrics.go` | E10-T02 | first_token_ms 可观测 |
| [ ] | M3/E10-T06/P0 | M3 | P0 | 补充 Stream tests | `internal/dataplane/stream/finalizer_test.go` | E10-T03 至 E10-T05 | 正常、中断、客户端断开测试通过 |

## 后续阶段任务

| 状态 | ID | 阶段 | 优先级 | 任务 | 目标文件或模块 | 依赖 | 验收标准 |
|---|---|---|---|---|---|---|---|
| [ ] | M3/E6-T07/P1 | M3 | P1 | 实现 OpenAI Responses adapter | `internal/provider/openai/responses.go` | E6-T03 | `/v1/responses` 可用 |
| [ ] | M3/E6-T08/P1 | M3 | P1 | 实现 Anthropic adapter | `internal/provider/anthropic` | E6-T01 | `/v1/messages` 可用 |
| [ ] | M3/E6-T09/P1 | M3 | P1 | 实现 Gemini adapter | `internal/provider/gemini` | E6-T01 | generateContent 和 stream 可用 |
| [ ] | M4/E11-T01/P1 | M4 | P1 | 实现 Task entity 和 service | `internal/task` | M2 完成 | create、get、cancel 可用 |
| [ ] | M4/E11-T02/P1 | M4 | P1 | 实现 Media request parser | `internal/dataplane/parser/media_parser.go` | E3-T04,E1-T05 | image、video、audio、music 请求可解析 |
| [ ] | M4/E11-T03/P1 | M4 | P1 | 实现 Model schema validation | `internal/dataplane/parser/schema_validator.go` | E1-T05 | `model_params` 校验通过 |
| [ ] | M4/E11-T04/P1 | M4 | P1 | 实现 File service | `internal/fileasset` | E2-T07 | base64、stream、url upload 可用 |
| [ ] | M4/E11-T05/P1 | M4 | P1 | 实现 ProviderTaskDispatcher | `internal/task/provider_dispatcher.go` | E11-T01 | 可提交 external task |
| [ ] | M4/E11-T06/P1 | M4 | P1 | 实现 ProviderTaskPoller job | `internal/worker/jobs/provider_task_poller.go` | E11-T05 | 可轮询 provider task 状态 |
| [ ] | M4/E11-T07/P1 | M4 | P1 | 实现 Result normalizer | `internal/task/result_normalizer.go` | E11-T06 | provider result 可转统一 assets |
| [ ] | M4/E11-T08/P1 | M4 | P1 | 实现 CallbackOutbox | `internal/task/callback_outbox.go` | E11-T01 | callback 失败可重试 |
| [ ] | M4/E11-T09/P1 | M4 | P1 | 实现 Task settlement | `internal/task/settlement.go` | E9-T02,E11-T01 | 任务最终扣费 |
| [ ] | M5/E13-T01/P1 | M5 | P1 | 实现 Admin auth | `internal/transport/controlhttp/middleware_auth.go` | M2 完成 | admin token 或 RBAC 可用 |
| [ ] | M5/E13-T02/P1 | M5 | P1 | 实现 Tenant API | `internal/transport/controlhttp/handler_tenant.go` | E13-T01 | Tenant CRUD 可用 |
| [ ] | M5/E13-T03/P1 | M5 | P1 | 实现 APIKey API | `internal/transport/controlhttp/handler_api_key.go` | E13-T01 | create、disable、list 可用 |
| [ ] | M5/E13-T04/P1 | M5 | P1 | 实现 Model API | `internal/transport/controlhttp/handler_model.go` | E13-T01 | model、schema、alias 可管理 |
| [ ] | M5/E13-T05/P1 | M5 | P1 | 实现 Provider API | `internal/transport/controlhttp/handler_provider.go` | E13-T01 | channel、credential、mapping 可管理 |
| [ ] | M5/E13-T06/P1 | M5 | P1 | 实现 Route API | `internal/transport/controlhttp/handler_route.go` | E13-T01 | route policy 和 binding 可管理 |
| [ ] | M5/E13-T07/P1 | M5 | P1 | 实现 Price API | `internal/transport/controlhttp/handler_price.go` | E13-T01 | price rules 可管理 |
| [ ] | M5/E13-T08/P1 | M5 | P1 | 实现 Limit API | `internal/transport/controlhttp/handler_limit.go` | E13-T01 | limit rules 可管理 |
| [ ] | M5/E13-T10/P1 | M5 | P1 | 实现 Snapshot API | `internal/transport/controlhttp/handler_snapshot.go` | E4-T01 至 E4-T06 | build、publish、rollback 可用 |
| [ ] | M5/E13-T11/P1 | M5 | P1 | 实现 Audit API | `internal/transport/controlhttp/handler_audit.go` | E1-T14,E2-T08 | audit list 和 search 可用 |
| [ ] | M6/E12-T01/P1 | M6 | P1 | 定义 Plugin interface | `internal/dataplane/plugin/plugin.go` | M3 完成 | Name、Phases、Execute 可用 |
| [ ] | M6/E12-T02/P1 | M6 | P1 | 实现 PluginManager | `internal/dataplane/plugin/manager.go` | E12-T01 | phase 执行可用 |
| [ ] | M6/E12-T03/P1 | M6 | P1 | 实现 PluginBinding resolver | `internal/dataplane/plugin/binding_resolver.go` | E12-T02,E4-T04 | scope 和 priority 排序正确 |
| [ ] | M6/E12-T04/P1 | M6 | P1 | 实现 RequestSizePlugin | `internal/dataplane/plugin/builtin/request_size.go` | E12-T02 | 超大请求被拒绝 |
| [ ] | M6/E12-T05/P1 | M6 | P1 | 实现 PromptTokenLimitPlugin | `internal/dataplane/plugin/builtin/prompt_token_limit.go` | E12-T02 | prompt token 超限被拒绝 |
| [ ] | M6/E12-T06/P1 | M6 | P1 | 实现 PiiRedactionPlugin | `internal/dataplane/plugin/builtin/pii_redaction.go` | E12-T02 | 日志和审计脱敏 |
| [ ] | M6/E12-T07/P2 | M6 | P2 | 实现 PromptGuardPlugin | `internal/dataplane/plugin/builtin/prompt_guard.go` | E12-T02 | prompt policy deny 可用 |
| [ ] | M6/E12-T08/P2 | M6 | P2 | 实现 ResponseGuardPlugin | `internal/dataplane/plugin/builtin/response_guard.go` | E12-T02 | response safety 可用 |
| [ ] | M7/E14-T01/P1 | M7 | P1 | 强化 Access log middleware | `internal/transport/httpserver/middleware_access_log.go` | M3 完成 | 日志不含敏感字段 |
| [ ] | M7/E14-T02/P1 | M7 | P1 | 固化 Metrics names | `internal/infra/metrics` | M3 完成 | gateway、provider、billing、task 指标齐全 |
| [ ] | M7/E14-T03/P1 | M7 | P1 | 补齐 Tracing spans | `internal/dataplane/observe` | M3 完成 | 每阶段 span 可见 |
| [ ] | M7/E14-T04/P1 | M7 | P1 | 实现 Redactor | `pkg/redaction` | M3 完成 | api key、prompt、provider key 脱敏 |
| [ ] | M7/E15-T03/P1 | M7 | P1 | 补充 OpenAI e2e | `test/e2e/openai_chat_test.go` | M1 完成 | mock provider e2e 通过 |
| [ ] | M7/E15-T04/P1 | M7 | P1 | 补充 Stream disconnect test | `test/e2e/stream_disconnect_test.go` | M3 完成 | 不误计 provider failure |
| [ ] | M7/E15-T05/P1 | M7 | P1 | 补充 Settlement failure test | `test/e2e/settlement_replay_test.go` | M2 完成 | failed 到 replay success 通过 |
| [ ] | M8/OPS-T01/P2 | M8 | P2 | 实现客户余额和用量报表 | reporting 或 control API | M2 完成 | 客户可查余额和用量 |
| [ ] | M8/OPS-T02/P2 | M8 | P2 | 实现渠道成本和利润报表 | reporting 或 control API | M2 完成 | 运营可查渠道利润 |
| [ ] | M8/OPS-T03/P2 | M8 | P2 | 实现财务对账报表 | `internal/billing/reconciliation_service.go` | E9-T08 | 财务可对账并定位差异 |
| [ ] | M8/OPS-T04/P2 | M8 | P2 | 实现模型市场配置 | control API | M5 完成 | 租户可见模型可配置 |
| [ ] | M8/OPS-T05/P2 | M8 | P2 | 实现 Agent metadata 报表 | reporting 或 analytics | M4 完成 | workflow、scene、shot 维度可分析 |

## 阶段验收总览

| 阶段 | 验收标准 |
|---|---|
| M0 | `go test ./...`、`make lint`、healthz、readyz、metrics 和基础目录结构通过 |
| M1 | `/v1/chat/completions` 可认证、路由、调用 provider、返回标准响应并记录基础观测 |
| M2 | provider 成功后的本地结算失败可修复，ledger 与 balance 可对账 |
| M3 | OpenAI、Claude、Gemini 主协议和 stream close-time accounting 可用 |
| M4 | 统一媒体任务可创建、查询、轮询、回调和最终结算 |
| M5 | 新增模型、渠道、价格、路由和限流无需重启 gateway |
| M6 | 插件可按 scope 绑定并执行 deny、redact、audit 和 metrics 行为 |
| M7 | dashboard、alert、压测和 failure drills 可支撑灰度商用 |
| M8 | 客户、运营和财务能围绕余额、用量、成本、利润和对账开展运营 |
