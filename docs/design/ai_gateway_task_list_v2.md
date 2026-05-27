# 商用 AI Gateway 详细 Task 列表 v0.2

版本：v0.2  
用途：可以直接放入项目管理系统。每个任务尽量明确包、文件、产出、验收。

---

## Epic 0：项目基础工程

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E0-T01 | P0 | 初始化 Go module 和目录结构 | `cmd/`, `internal/`, `pkg/` | `go test ./...` 可运行 |
| E0-T02 | P0 | 配置加载：YAML + ENV override | `internal/infra/conf/config.go` | 默认配置、文件配置、ENV 覆盖测试通过 |
| E0-T03 | P0 | 统一错误包 | `pkg/apperr/errors.go` | 支持 code/status/type/retryable/safe |
| E0-T04 | P0 | 结构化日志 | `internal/infra/log/logger.go` | request_id/trace_id 字段可输出 |
| E0-T05 | P0 | HTTP server 基础 | `internal/transport/httpserver` | healthz/readyz/metrics 可访问 |
| E0-T06 | P0 | OpenTelemetry 初始化 | `internal/infra/tracing` | trace exporter 可配置关闭/开启 |
| E0-T07 | P0 | Prometheus metrics 初始化 | `internal/infra/metrics` | `/metrics` 输出标准格式 |
| E0-T08 | P0 | DB 初始化 | `internal/infra/db` | ping、pool config、close |
| E0-T09 | P0 | Redis 初始化 | `internal/infra/redis` | ping、close、db 选择 |
| E0-T10 | P0 | Migration 框架 | `migrations/`, `internal/infra/db/migrate.go` | 本地能 migrate up/down |
| E0-T11 | P0 | Makefile | `Makefile` | test/lint/race/build/run |
| E0-T12 | P0 | CI | `.github/workflows/ci.yml` | PR 触发 test/race/lint |
| E0-T13 | P1 | Docker Compose | `deployments/docker-compose.yml` | gateway/control/worker/db/redis 可启动 |
| E0-T14 | P1 | API 文档流程 | `openapi/ai_gateway_openapi.yaml` | 可导入 Apifox |

---

## Epic 1：Domain 基础模型

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E1-T01 | P0 | Tenant/Account 实体 | `internal/domain/tenant` | Validate 测试通过 |
| E1-T02 | P0 | Project/APIKey 实体 | `internal/domain/identity` | key status/expiry/scope 校验 |
| E1-T03 | P0 | API key hash 工具 | `internal/domain/identity/key_hasher.go` | 不存明文，可校验 |
| E1-T04 | P0 | PublicModel/Capability | `internal/domain/modelcatalog` | 支持 chat/image/video/audio |
| E1-T05 | P0 | ModelSchema | `internal/domain/modelcatalog/schema.go` | JSON schema 可校验 |
| E1-T06 | P0 | ProviderChannel | `internal/domain/provider/channel.go` | status/priority/weight 校验 |
| E1-T07 | P0 | ProviderCredentialRef | `internal/domain/provider/credential.go` | 不暴露明文 |
| E1-T08 | P0 | ProviderModelMapping | `internal/domain/provider/mapping.go` | capability/stream 校验 |
| E1-T09 | P0 | RoutePolicy | `internal/domain/routing/policy.go` | retry/fallback/strategy 校验 |
| E1-T10 | P0 | PriceRule | `internal/domain/pricing/price_rule.go` | 金额用 micros，不用 float |
| E1-T11 | P0 | LimitRule | `internal/domain/policy/limit_rule.go` | scope/values 校验 |
| E1-T12 | P0 | Billing entities | `internal/domain/billing` | hold/attempt/ledger 状态机 |
| E1-T13 | P0 | Task entities | `internal/domain/task` | queued/running/succeeded/failed/canceled |
| E1-T14 | P0 | Audit entity | `internal/domain/audit` | actor/action/resource metadata |

---

## Epic 2：Repository 与数据库

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E2-T01 | P0 | identity tables migration | `migrations/000001_identity.sql` | accounts/projects/api_keys |
| E2-T02 | P0 | model catalog tables | `migrations/000002_models.sql` | models/aliases/schema |
| E2-T03 | P0 | provider tables | `migrations/000003_provider.sql` | channels/credentials/mappings |
| E2-T04 | P0 | routing tables | `migrations/000004_routing.sql` | policies/channels |
| E2-T05 | P0 | pricing/limit/plugin tables | `migrations/000005_policy.sql` | price_rules/limit_rules/plugin_bindings |
| E2-T06 | P0 | billing tables | `migrations/000006_billing.sql` | balances/holds/attempts/records/ledger |
| E2-T07 | P0 | task/file tables | `migrations/000007_tasks_files.sql` | tasks/files/callback_outbox |
| E2-T08 | P0 | audit/outbox tables | `migrations/000008_audit_outbox.sql` | audit_events/outbox_events |
| E2-T09 | P0 | Identity repository | `internal/infra/store/sql/identity_repository.go` | CRUD + tests |
| E2-T10 | P0 | Model repository | `internal/infra/store/sql/model_repository.go` | list active models |
| E2-T11 | P0 | Provider repository | `internal/infra/store/sql/provider_repository.go` | encrypted credential ref |
| E2-T12 | P0 | Routing repository | `internal/infra/store/sql/routing_repository.go` | policies and bindings |
| E2-T13 | P0 | Billing repository | `internal/infra/store/sql/billing_repository.go` | transaction tests |
| E2-T14 | P0 | Task repository | `internal/infra/store/sql/task_repository.go` | lock/poll/update |
| E2-T15 | P0 | Audit repository | `internal/infra/store/sql/audit_repository.go` | append-only |

---

## Epic 3：GatewayEngine 基础链路

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E3-T01 | P0 | RequestState 定义 | `internal/dataplane/engine/state.go` | 字段覆盖生命周期 |
| E3-T02 | P0 | GatewayEngine 结构体 | `internal/dataplane/engine/engine.go` | 可注入全部依赖 |
| E3-T03 | P0 | APIClassifier | `internal/dataplane/classifier` | path -> canonical API |
| E3-T04 | P0 | BodyStore | `internal/dataplane/parser/body_store.go` | 小 body 内存，大 body 临时文件 |
| E3-T05 | P0 | OpenAI chat parser | `internal/dataplane/parser/openai_parser.go` | model/messages/stream/usage estimate |
| E3-T06 | P0 | CredentialExtractor | `internal/dataplane/auth/credential.go` | Bearer/x-api-key/query key 冲突检测 |
| E3-T07 | P0 | Authenticator | `internal/dataplane/auth/authenticator.go` | API key hash 校验 |
| E3-T08 | P0 | PolicyEvaluator | `internal/dataplane/policy/evaluator.go` | IP/model/scope 校验 |
| E3-T09 | P0 | Engine Handle 主流程 | `internal/dataplane/engine/handle.go` | happy path 单测 |
| E3-T10 | P0 | ErrorMapper | `internal/dataplane/errors/mapper.go` | OpenAI/Claude/Gemini 错误格式 |
| E3-T11 | P1 | Diagnostic headers | `internal/transport/gatewayhttp` | 可配置输出 route/task/snapshot |

---

## Epic 4：Runtime Snapshot

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E4-T01 | P0 | RuntimeSnapshot struct | `internal/controlplane/snapshot/types.go` | 包含 api_keys/models/channels/policies |
| E4-T02 | P0 | Snapshot codec | `internal/controlplane/snapshot/codec.go` | JSON marshal/checksum |
| E4-T03 | P0 | Snapshot validator | `internal/controlplane/snapshot/validator.go` | 坏配置拒绝 |
| E4-T04 | P0 | IndexedSnapshot | `internal/dataplane/snapshot/index.go` | key/model/route/price 索引 |
| E4-T05 | P0 | Redis snapshot store | `internal/infra/snapshotstore/redis.go` | publish/load/watch |
| E4-T06 | P0 | Gateway snapshot cache | `internal/dataplane/snapshot/cache.go` | local atomic pointer |
| E4-T07 | P1 | Snapshot metrics | `internal/dataplane/snapshot/metrics.go` | version/staleness/errors |
| E4-T08 | P1 | Snapshot rollback | `internal/controlplane/snapshot/rollback.go` | previous active 可恢复 |

---

## Epic 5：路由系统

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E5-T01 | P0 | RoutePlanner | `internal/dataplane/routing/planner.go` | Plan 输出 RouteDecision |
| E5-T02 | P0 | PolicyResolver | `internal/dataplane/routing/policy_resolver.go` | project > account > tenant > global |
| E5-T03 | P0 | ModelResolver | `internal/dataplane/routing/model_resolver.go` | alias -> model |
| E5-T04 | P0 | CandidateResolver | `internal/dataplane/routing/candidate_resolver.go` | 过滤 disabled/capability/stream |
| E5-T05 | P0 | StrategyRegistry | `internal/dataplane/routing/strategy_registry.go` | selector 注册/查找 |
| E5-T06 | P0 | PrioritySelector | `internal/dataplane/routing/selectors/priority.go` | priority stable sort |
| E5-T07 | P0 | WeightedRandomSelector | `internal/dataplane/routing/selectors/weighted_random.go` | 无 key 也随机，有 key 粘性 |
| E5-T08 | P1 | RoundRobinSelector | `internal/dataplane/routing/selectors/round_robin.go` | 并发安全 |
| E5-T09 | P1 | ConsistentHashSelector | `internal/dataplane/routing/selectors/consistent_hash.go` | same key same channel |
| E5-T10 | P1 | RouteSignalProvider | `internal/dataplane/routing/signal_provider.go` | health/latency/cost/quota |
| E5-T11 | P1 | HealthWeightedSelector | `internal/dataplane/routing/selectors/health_weighted.go` | score 排序 |
| E5-T12 | P1 | LeastCostSelector | `internal/dataplane/routing/selectors/least_cost.go` | 成本最低优先 |
| E5-T13 | P2 | SemanticSelector | `internal/dataplane/routing/selectors/semantic.go` | prompt embedding route |

---

## Epic 6：Provider Adapter

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E6-T01 | P0 | Provider relay types | `internal/provider/relay/types.go` | Request/Response/Stream/Error |
| E6-T02 | P0 | Provider registry | `internal/provider/registry.go` | register/get capabilities |
| E6-T03 | P0 | OpenAI-compatible adapter | `internal/provider/openai` | chat completions |
| E6-T04 | P0 | HTTP client with timeout | `internal/provider/internal/httpclient` | non-stream/stream timeout |
| E6-T05 | P0 | Provider error mapping | `internal/provider/openai/errors.go` | 429/5xx/401/400 分类 |
| E6-T06 | P0 | Usage parser | `internal/provider/openai/usage.go` | input/output tokens |
| E6-T07 | P1 | OpenAI Responses adapter | `internal/provider/openai/responses.go` | `/v1/responses` |
| E6-T08 | P1 | Anthropic adapter | `internal/provider/anthropic` | `/v1/messages` |
| E6-T09 | P1 | Gemini adapter | `internal/provider/gemini` | generateContent/stream |
| E6-T10 | P1 | Seedance video adapter | `internal/provider/seedance` | submit/poll video task |
| E6-T11 | P1 | Provider conformance suite | `internal/provider/internal/conformance` | 各 provider contract 测试 |

---

## Epic 7：Dispatch / Retry / Fallback / Circuit

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E7-T01 | P0 | ProviderDispatcher | `internal/dataplane/dispatch/dispatcher.go` | 遍历 candidate |
| E7-T02 | P0 | AttemptExecutor | `internal/dataplane/dispatch/attempt.go` | 单次调用记录 attempt |
| E7-T03 | P0 | ErrorClassifier | `internal/dataplane/dispatch/error_classifier.go` | retryable/fallbackable/health_penalty |
| E7-T04 | P0 | RetryController | `internal/dataplane/dispatch/retry.go` | max/backoff/jitter |
| E7-T05 | P0 | FallbackController | `internal/dataplane/dispatch/fallback.go` | 按错误类型 fallback |
| E7-T06 | P1 | CircuitBreaker | `internal/dataplane/dispatch/circuit.go` | closed/open/half-open |
| E7-T07 | P1 | Channel limiter | `internal/dataplane/dispatch/channel_limit.go` | channel QPS/concurrency |
| E7-T08 | P1 | Provider health metrics | `internal/dataplane/dispatch/observer.go` | latency/error/first token |

---

## Epic 8：限流与准入

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E8-T01 | P0 | PriceEstimator | `internal/dataplane/admission/price_estimator.go` | 预估费用 |
| E8-T02 | P0 | AdmissionController | `internal/dataplane/admission/controller.go` | 创建 balance_hold |
| E8-T03 | P0 | Balance service | `internal/billing/balance_service.go` | 可用余额检查 |
| E8-T04 | P0 | Redis token bucket | `internal/dataplane/limit/redis_token_bucket.go` | QPS/TPM |
| E8-T05 | P0 | Redis concurrency lease | `internal/dataplane/limit/redis_concurrency.go` | 多副本并发准确 |
| E8-T06 | P0 | Limit rule matcher | `internal/dataplane/limit/rule_matcher.go` | tenant/project/key/model/channel |
| E8-T07 | P1 | Cost budget limiter | `internal/dataplane/limit/cost_budget.go` | 日/月成本限制 |
| E8-T08 | P1 | Local deny cache | `internal/dataplane/limit/deny_cache.go` | 降低 Redis 压力 |

---

## Epic 9：Settlement / Billing

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E9-T01 | P0 | Settlement planner | `internal/billing/settlement_planner.go` | success/failure/stream plan |
| E9-T02 | P0 | Settlement executor | `internal/billing/settlement_executor.go` | 事务扣费 |
| E9-T03 | P0 | UsageAttempt writer | `internal/billing/usage_attempt_service.go` | 每次 provider attempt 记录 |
| E9-T04 | P0 | UsageRecord writer | `internal/billing/usage_record_service.go` | 最终客户用量 |
| E9-T05 | P0 | Ledger service | `internal/billing/ledger_service.go` | 幂等 ledger entry |
| E9-T06 | P0 | FailedSettlement service | `internal/billing/failed_settlement_service.go` | 失败可修复 |
| E9-T07 | P0 | Settlement replay job | `internal/worker/jobs/failed_settlement_replayer.go` | 故障后恢复扣费 |
| E9-T08 | P1 | Reconciliation report | `internal/billing/reconciliation_service.go` | ledger 与 balance 对账 |
| E9-T09 | P1 | Usage API | `internal/transport/controlhttp/handler_usage.go` | 客户查询用量 |

---

## Epic 10：Streaming

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E10-T01 | P0 | ProviderStream interface | `internal/provider/relay/stream.go` | Read/Close/Usage |
| E10-T02 | P0 | Stream writer | `internal/transport/gatewayhttp/stream_writer.go` | SSE/chunk 写出 |
| E10-T03 | P0 | StreamFinalizer | `internal/dataplane/stream/finalizer.go` | close-time settlement |
| E10-T04 | P0 | Downstream error report | `internal/dataplane/stream/downstream.go` | client disconnect 可识别 |
| E10-T05 | P0 | First token latency | `internal/dataplane/stream/metrics.go` | first_token_ms |
| E10-T06 | P0 | Stream tests | `internal/dataplane/stream/finalizer_test.go` | 正常/中断/客户端断开 |

---

## Epic 11：异步媒体任务

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E11-T01 | P0 | Task entity/service | `internal/task` | create/get/cancel |
| E11-T02 | P0 | Media request parser | `internal/dataplane/parser/media_parser.go` | image/video/audio/music |
| E11-T03 | P0 | Model schema validation | `internal/dataplane/parser/schema_validator.go` | model_params 校验 |
| E11-T04 | P0 | File service | `internal/fileasset` | base64/stream/url upload |
| E11-T05 | P0 | ProviderTaskDispatcher | `internal/task/provider_dispatcher.go` | submit external task |
| E11-T06 | P0 | ProviderTaskPoller job | `internal/worker/jobs/provider_task_poller.go` | 轮询状态 |
| E11-T07 | P0 | Result normalizer | `internal/task/result_normalizer.go` | provider result -> assets |
| E11-T08 | P0 | CallbackOutbox | `internal/task/callback_outbox.go` | callback 重试 |
| E11-T09 | P0 | Task settlement | `internal/task/settlement.go` | 最终扣费 |
| E11-T10 | P1 | Task cancel | `internal/task/cancel.go` | cancel provider 或本地取消 |
| E11-T11 | P1 | Task expiration | `internal/worker/jobs/expired_task_reaper.go` | 过期资产清理 |

---

## Epic 12：插件系统

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E12-T01 | P0 | Plugin interface | `internal/dataplane/plugin/plugin.go` | Name/Phases/Execute |
| E12-T02 | P0 | PluginManager | `internal/dataplane/plugin/manager.go` | phase 执行 |
| E12-T03 | P0 | PluginBinding resolver | `internal/dataplane/plugin/binding_resolver.go` | scope + priority |
| E12-T04 | P0 | RequestSizePlugin | `internal/dataplane/plugin/builtin/request_size.go` | 超大请求拒绝 |
| E12-T05 | P0 | PromptTokenLimitPlugin | `.../builtin/prompt_token_limit.go` | prompt token 限制 |
| E12-T06 | P0 | PiiRedactionPlugin | `.../builtin/pii_redaction.go` | 日志/审计脱敏 |
| E12-T07 | P1 | PromptGuardPlugin | `.../builtin/prompt_guard.go` | prompt policy deny |
| E12-T08 | P1 | ResponseGuardPlugin | `.../builtin/response_guard.go` | response safety |
| E12-T09 | P1 | CostGuardPlugin | `.../builtin/cost_guard.go` | 成本预算 |
| E12-T10 | P1 | AuditLogPlugin | `.../builtin/audit_log.go` | 审计事件 |
| E12-T11 | P1 | LLMMetricPlugin | `.../builtin/llm_metrics.go` | token/cost 指标 |
| E12-T12 | P1 | CallbackPlugin | `.../builtin/callback.go` | task callback |

---

## Epic 13：Control API

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E13-T01 | P0 | Admin auth | `internal/transport/controlhttp/middleware_auth.go` | admin token/RBAC |
| E13-T02 | P0 | Tenant API | `handler_tenant.go` | CRUD |
| E13-T03 | P0 | APIKey API | `handler_api_key.go` | create/disable/list |
| E13-T04 | P0 | Model API | `handler_model.go` | model/schema/alias |
| E13-T05 | P0 | Provider API | `handler_provider.go` | channel/credential/mapping |
| E13-T06 | P0 | Route API | `handler_route.go` | policy/binding |
| E13-T07 | P0 | Price API | `handler_price.go` | price rules |
| E13-T08 | P0 | Limit API | `handler_limit.go` | limit rules |
| E13-T09 | P1 | Plugin API | `handler_plugin.go` | plugin binding |
| E13-T10 | P0 | Snapshot API | `handler_snapshot.go` | build/publish/rollback |
| E13-T11 | P0 | Audit API | `handler_audit.go` | list/search |

---

## Epic 14：Observability / Security

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E14-T01 | P0 | Access log middleware | `internal/transport/httpserver/middleware_access_log.go` | 不含敏感字段 |
| E14-T02 | P0 | Metrics names | `internal/infra/metrics` | gateway/provider/billing/task |
| E14-T03 | P0 | Tracing spans | `internal/dataplane/observe` | 每阶段 span |
| E14-T04 | P0 | Redactor | `pkg/redaction` | api key/prompt/provider key 脱敏 |
| E14-T05 | P0 | Audit service | `internal/domain/audit`, `internal/app/audit` | sensitive operation audit |
| E14-T06 | P1 | Grafana dashboard | `deployments/grafana` | provider/task/billing dashboard |
| E14-T07 | P1 | Alert rules | `deployments/prometheus/alerts.yml` | settlement backlog/provider error |
| E14-T08 | P1 | Security event log | `internal/security/events.go` | policy denied/jailbreak/pii |

---

## Epic 15：测试和故障演练

| ID | 优先级 | 任务 | 代码位置 | 验收 |
|---|---|---|---|---|
| E15-T01 | P0 | Unit test baseline | 各包 `_test.go` | 核心包覆盖率门槛 |
| E15-T02 | P0 | Integration DB/Redis | `test/integration` | docker compose 跑通 |
| E15-T03 | P0 | OpenAI e2e | `test/e2e/openai_chat_test.go` | mock provider |
| E15-T04 | P0 | Stream disconnect test | `test/e2e/stream_disconnect_test.go` | 不误计 provider failure |
| E15-T05 | P0 | Settlement failure test | `test/e2e/settlement_replay_test.go` | failed -> replay success |
| E15-T06 | P1 | Video task e2e | `test/e2e/video_task_test.go` | task lifecycle |
| E15-T07 | P1 | Load test script | `tools/loadtest` | chat/video/stream 压测 |
| E15-T08 | P1 | Failure drills | `tools/failure-drills` | Redis/DB/provider/callback 故障 |

---

## 推荐开发顺序清单

第一轮只做这些，不要分散：

```text
E0 全部 P0
E1 全部 P0
E2-T01~E2-T13
E3 全部 P0
E4-T01~E4-T06
E5-T01~E5-T07
E6-T01~E6-T06
E7-T01~E7-T05
E8-T01~E8-T06
E9-T01~E9-T07
E10-T01~E10-T06
```

完成后系统就具备最小商用内核：

```text
认证
路由
provider relay
stream
限流
余额预占
结算
失败修复
基础观测
```
