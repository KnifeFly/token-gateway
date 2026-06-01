# 商用 AI Gateway 架构设计文档

## 1. 总体架构

系统采用四个核心运行平面，并在 P19-P22 后新增 Human Console Plane：

```text
Data Plane
  cmd/gateway
  高并发请求、认证、路由、限流、provider relay、stream、同步结算

Control Plane
  cmd/control-api
  租户、项目、API key、模型、渠道、路由、价格、插件、发布 snapshot

Config Plane
  cmd/configd
  snapshot 分发、订阅、版本管理、回滚、数据面配置热加载

Worker Plane
  cmd/worker
  异步媒体任务、provider task polling、failed settlement replay、callback、对账

Human Console Plane
  cmd/console
  Portal/Admin browser BFF、session、CSRF、RBAC、audit、dashboard/read model、可选静态资源
```

Observability 和 Security 是横切能力，不单独作为进程。当前路线保留基础日志、metrics、trace、redaction 和审计插件接入点，不规划独立生产级 Observability 平台。

Human Console Plane 不进入数据面热路径，不拥有账务、task、snapshot 或 provider 状态机。它只能通过应用服务和 owner service 读取或发起受控操作。

---

## 2. Go Module 与代码分层

推荐 module：

```text
module github.com/your-org/token-gateway
```

代码分层：

```text
cmd/                  进程入口
internal/bootstrap/   依赖组装
internal/transport/   HTTP/SSE/WebSocket 传输层
internal/app/         Portal/Admin human app use case
internal/domain/      纯领域模型和领域服务
internal/dataplane/   数据面核心链路
internal/controlplane/控制面配置与发布
internal/provider/    provider adapter
internal/billing/     账务、ledger、结算
internal/task/        异步任务领域
internal/worker/      worker job
internal/infra/       DB/Redis/KMS/OTel 等基础设施
pkg/                  可被外部复用的小包
```

依赖方向：

```text
transport -> dataplane -> domain
controlplane -> domain
app/portal -> billing/task/controlplane/domain read ports
app/admin -> controlplane/billing/task/worker read ports and owner services
worker -> billing/task/provider/domain
infra -> domain interfaces implementation
provider -> provider/relay + domain types
```

禁止：

```text
domain import gin/chi/sql/redis
provider adapter 写 ledger
handler 写业务事务
worker 直接绕过 billing service 写账务表
adminhttp 直接写 control/config 表
portalwebhttp 直接暴露 provider secret、raw prompt 或 raw response
```

---

## 3. 推荐目录结构

```text
cmd/
  gateway/main.go
  control-api/main.go
  configd/main.go
  worker/main.go
  console/main.go

internal/
  bootstrap/
    app.go
    config.go
    logger.go
    telemetry.go
    database.go
    redis.go
    gateway.go
    control_api.go
    configd.go
    worker.go
    console.go

  transport/
    httpserver/
      server.go
      router.go
      middleware_request_id.go
      middleware_access_log.go
      middleware_recovery.go
      middleware_metrics.go
      middleware_tracing.go
      error_writer.go
    gatewayhttp/
      routes.go
      handler.go
      classifier_adapter.go
      body_reader.go
      sse_writer.go
      response_writer.go
      protocol_headers.go
    controlhttp/
      routes.go
      auth_middleware.go
      model_handler.go
      provider_handler.go
      route_policy_handler.go
      price_handler.go
      snapshot_handler.go
    realtimehttp/
      session_handler.go
      websocket_handler.go
    portalhttp/
      handler.go
      error_writer.go
    consolehttp/
      handler.go
      session.go
      csrf.go
      static.go
    portalwebhttp/
      handler.go
      auth_handler.go
      dashboard_handler.go
      response.go
    adminhttp/
      handler.go
      auth_handler.go
      dashboard_handler.go
      audit_handler.go
      response.go

  app/
    portal/
      types.go
      service/
        service.go
        ports.go
        dashboard.go
        models.go
        credits.go
        usage.go
        api_keys.go
        tasks.go
        onboarding.go
        sessions.go
      repository/
        repository.go
        mysql.go
        mysql_dashboard.go
        mysql_usage.go
        mysql_api_keys.go
        mysql_tasks.go
        mysql_sessions.go
    admin/
      types.go
      service/
        service.go
        ports.go
        auth.go
        dashboard.go
        tenants.go
        projects.go
        api_keys.go
        models.go
        channels.go
        routes.go
        pricing.go
        limits.go
        snapshots.go
        operations.go
        audit.go
        operators.go
        sessions.go
      repository/
        repository.go
        mysql.go
        mysql_sessions.go
        mysql_operators.go
        mysql_dashboard.go
        mysql_config_views.go
        mysql_operations.go
        mysql_audit.go

  portal/
    service.go     # temporary compatibility shim for /v1/portal/* during migration
    types.go

  domain/
    tenant/
      types.go
      repository.go
      service.go
    identity/
      api_key.go
      api_key_hash.go
      principal.go
      repository.go
      service.go
    modelcatalog/
      model.go
      alias.go
      schema.go
      capability.go
      repository.go
      service.go
    provider/
      channel.go
      credential.go
      mapping.go
      repository.go
      service.go
    routing/
      policy.go
      decision.go
      candidate.go
      selector.go
      signal.go
      repository.go
      service.go
    pricing/
      price_rule.go
      quote.go
      calculator.go
      repository.go
    billing/
      balance.go
      hold.go
      ledger.go
      usage.go
      settlement.go
      failed_settlement.go
      repository.go
    audit/
      event.go
      repository.go
      service.go
    policy/
      acl.go
      content_policy.go
      cost_policy.go

  dataplane/
    engine/
      engine.go
      config.go
      dependencies.go
      state.go
      lifecycle.go
      compensation.go
      errors.go
    classifier/
      classifier.go
      endpoint_table.go
      protocol_mode.go
      model_hint.go
    parser/
      parser.go
      openai_parser.go
      claude_parser.go
      gemini_parser.go
      unified_media_parser.go
    auth/
      authenticator.go
      credential_extractor.go
      api_key_authenticator.go
      revocation_checker.go
    policy/
      evaluator.go
      model_acl.go
      tenant_policy.go
    plugin/
      phase.go
      plugin.go
      manager.go
      binding.go
      result.go
      builtin_registry.go
    plugin/builtin/
      request_size.go
      ip_allowlist.go
      model_acl.go
      prompt_token_limit.go
      pii_redaction.go
      prompt_guard.go
      response_guard.go
      cost_guard.go
      audit_log.go
      llm_metrics.go
      callback.go
    router/
      planner.go
      policy_resolver.go
      model_resolver.go
      candidate_resolver.go
      strategy_registry.go
      strategy_priority.go
      strategy_weighted_random.go
      strategy_round_robin.go
      strategy_consistent_hash.go
      strategy_health_weighted.go
      degradation.go
    admission/
      controller.go
      balance_reservation.go
      idempotency.go
    limiter/
      enforcer.go
      redis_token_bucket.go
      redis_concurrency.go
      local_deny_cache.go
    dispatch/
      dispatcher.go
      attempt.go
      retry.go
      fallback.go
      circuit.go
      timeout.go
    stream/
      finalizer.go
      accounting_stream.go
      sse_copy.go
      usage_collector.go
    settlement/
      service.go
      planner.go
      executor.go
      repair.go
    snapshot/
      provider.go
      runtime_snapshot.go
      atomic_store.go
      validator.go
      stale_policy.go
    observe/
      recorder.go
      metrics.go
      tracing.go
      log_fields.go
    realtime/
      engine.go
      session.go

  provider/
    relay/
      request.go
      response.go
      stream.go
      error.go
      adapter.go
      capability.go
    registry/
      registry.go
      module.go
      descriptor.go
    openai/
      module.go
      adapter.go
      request_mapper.go
      response_mapper.go
      stream_mapper.go
      usage.go
    claude/
      module.go
      adapter.go
      messages_mapper.go
    gemini/
      module.go
      adapter.go
      generate_content_mapper.go
    compatible/
      openai_compatible.go
      profile.go

  controlplane/
    snapshot/
      builder.go
      publisher.go
      rollout.go
      rollback.go
      validator.go
    admin/
      model_usecase.go
      provider_usecase.go
      route_policy_usecase.go
      price_usecase.go
      api_key_usecase.go
    schema/
      model_schema_validator.go
      openapi_exporter.go

  billing/
    service.go
    reservation.go
    settlement.go
    ledger.go
    failed_settlement.go
    reconciliation.go

  task/
    task.go
    idempotency.go
    provider_task.go
    callback.go
    repository.go
    service.go

  worker/
    runner.go
    job.go
    lease.go
    failed_settlement_replayer.go
    provider_task_poller.go
    callback_dispatcher.go
    balance_hold_reaper.go
    reconciliation_job.go

  infra/
    db/
      db.go
      transaction.go
      migrations/
    mysql/
      repositories.go
      identity_repository.go
      model_repository.go
      provider_repository.go
      routing_repository.go
      billing_repository.go
      task_repository.go
      audit_repository.go
    redis/
      client.go
      token_bucket.go
      concurrency_lease.go
      snapshot_store.go
      pubsub.go
      revocation.go
    kms/
      store.go
      aesgcm.go
      envelope.go
    telemetry/
      metrics.go
      tracing.go
    redaction/
      redactor.go
      pii.go

pkg/
  apperr/
  money/
  tokenusage/
  ptr/
  clock/
```

---

## 4. 进程入口设计

### 4.1 `cmd/gateway`

职责：启动数据面 HTTP 服务。

```go
func main() {
    cfg := bootstrap.MustLoadConfig()
    app := bootstrap.MustNewGatewayApp(context.Background(), cfg)
    bootstrap.Run(app)
}
```

### 4.2 `cmd/control-api`

职责：启动控制面 API。

支持：租户、API key、模型、provider channel、路由策略、价格、插件绑定、snapshot 发布。

### 4.3 `cmd/configd`

职责：snapshot 分发和 watch。MVP 可以并入 control-api，生产建议独立。

### 4.4 `cmd/worker`

职责：运行 job runner。

必须 job：

```text
failed_settlement_replayer
provider_task_poller
callback_dispatcher
balance_hold_reaper
reconciliation_job
```

### 4.5 `cmd/console`

职责：启动 Human Console HTTP 服务。

支持：

```text
/api/portal/v1/*  Portal Web BFF
/api/admin/v1/*   Admin Web BFF
/portal/*         optional Portal static assets
/admin-ui/*       optional Admin static assets
```

`cmd/console` 不服务 `/v1/*` 数据面，不服务 `/admin/*` machine Control API。生产环境可以只让它承载 BFF，由 CDN/Nginx 托管 `/portal/*` 和 `/admin-ui/*` 静态资源。

---

## 5. GatewayEngine 详细结构

```go
type GatewayEngine struct {
    cfg        EngineConfig
    deps       EngineDependencies

    snapshot   SnapshotProvider
    classifier APIClassifier
    parser     RequestParser
    auth       Authenticator
    policy     PolicyEvaluator
    plugins    PluginManager
    router     RoutePlanner
    admission  AdmissionController
    limiter    LimitEnforcer
    dispatcher ProviderDispatcher
    stream     StreamFinalizer
    settlement SettlementService
    tasks      TaskBridge
    files      FileService
    audit      Auditor
    redactor   Redactor
    errmap     ErrorMapper
    observe    ObserveRecorder
}
```

`FileService` 只表达 transient input asset 的 metadata、幂等、大小限制和 provider 转发辅助能力，不代表对象存储服务。P13 选择 URL passthrough 和有限 base64/inline 边界，multipart/stream upload 在没有真实 streaming spool/object reference 前返回 `feature_not_enabled`。Gateway 不承诺文件持久化、下载地址、生命周期管理或存储 SLA。Provider 返回的媒体结果以 `results`、`assets`、`usage` 和非敏感 `provider_metadata` 进入 task response、callback 和 settlement，不依赖 gateway 本地文件对象。

### 5.1 EngineConfig

```go
type EngineConfig struct {
    ServiceName string
    Environment string

    Body BodyConfig
    Timeout TimeoutConfig
    Stream StreamConfig
    Snapshot SnapshotConfig
    Protocol ProtocolConfig
    Idempotency IdempotencyConfig
    Degradation DegradationConfig

    EnableDebugHeaders bool
}
```

### 5.2 EngineDependencies

```go
type EngineDependencies struct {
    Clock       clock.Clock
    Logger      *slog.Logger
    Metrics     MetricsRecorder
    Tracer      trace.Tracer
    EventBus    EventBus
    Repositories Repositories
}
```

### 5.3 RequestState

详见系统设计文档。原则：所有阶段修改同一个 state，但核心字段只允许单向演进，不允许后续阶段覆盖认证身份和 snapshot pin。

字段修改规则：

| 字段 | 可写阶段 | 是否可覆盖 |
|---|---|---|
| RequestID/TraceID | newState | 否 |
| ProtocolMode | classifier | 否 |
| Principal | auth | 否 |
| SnapshotRef | snapshot load | 否 |
| RoutePlan | route | 可由 degradation 显式重写，但需记录 |
| BalanceHold | admission | 否 |
| ActualUsage | provider/stream | 可补充 |
| SettlementResult | settlement | 否 |

### 5.4 GatewayEngine.Handle

```go
func (e *GatewayEngine) Handle(ctx context.Context, req IncomingRequest) (*GatewayResponse, error) {
    state := e.newState(req)
    defer e.observe.Finish(ctx, state)

    if err := e.plugins.Run(ctx, PhasePreRequest, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.classifier.Classify(ctx, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.parser.Parse(ctx, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.snapshot.Attach(ctx, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.auth.Authenticate(ctx, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.plugins.Run(ctx, PhasePostAuth, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.policy.Evaluate(ctx, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.plugins.Run(ctx, PhasePrePrompt, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.router.Plan(ctx, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.plugins.Run(ctx, PhasePostRoute, state); err != nil { return nil, e.fail(ctx, state, err) }
    if err := e.admission.Reserve(ctx, state); err != nil { return nil, e.fail(ctx, state, err) }
    release, err := e.limiter.Acquire(ctx, state)
    if err != nil { return nil, e.compensate(ctx, state, err) }
    state.LimitReleases = append(state.LimitReleases, release)

    if state.Async {
        response, replayHit, err := e.tasks.CreateAndDispatch(ctx, state)
        if replayHit { e.admission.Release(ctx, state, ErrIdempotencyReplay) }
        return response, err
    }

    result, err := e.dispatcher.Dispatch(ctx, state)
    if err != nil { return nil, e.compensate(ctx, state, err) }

    if result.Stream != nil {
        // StreamFinalizer drains LimitReleases and releases them after stream close.
        return e.stream.Wrap(ctx, state, result)
    }

    state.ProviderResult = result
    if err := e.settlement.Settle(ctx, state); err != nil { return nil, e.repair(ctx, state, err) }
    if err := e.plugins.Run(ctx, PhaseAudit, state); err != nil { return nil, e.fail(ctx, state, err) }
    return result.Response, nil
}
```

---

## 6. APIClassifier 架构

### 6.1 EndpointSpec

```go
type EndpointSpec struct {
    Method        string
    PathPattern   string
    CanonicalAPI   CanonicalAPI
    AllowedModes   []ProtocolMode
    BodySchemaHint BodySchemaHint
    AsyncDefault   bool
    StreamAllowed  bool
    ModelInPath    bool
}
```

### 6.2 Classifier

```go
type Classifier struct {
    endpoints EndpointTable
    models    ModelLookup
}
```

核心行为：

```text
1. match endpoint
2. read X-Gateway-Protocol
3. if header explicit -> validate allowed
4. if auto -> infer from model registry
5. if still unknown -> infer from body schema
6. if ambiguous -> error ambiguous_protocol
```

### 6.3 ModelLookup

```go
type ModelLookup interface {
    LookupModel(ctx context.Context, snapshot SnapshotRef, model string) (*ModelRuntimeView, error)
}
```

---

## 7. Plugin 架构

### 7.1 Phase enum

```go
type Phase string

const (
    PhasePreRequest    Phase = "pre_request"
    PhasePostAuth      Phase = "post_auth"
    PhasePrePrompt     Phase = "pre_prompt"
    PhasePreRoute      Phase = "pre_route"
    PhasePostRoute     Phase = "post_route"
    PhasePreProvider   Phase = "pre_provider"
    PhasePostProvider  Phase = "post_provider"
    PhaseStreamChunk   Phase = "stream_chunk"
    PhasePreResponse   Phase = "pre_response"
    PhasePreSettlement Phase = "pre_settlement"
    PhasePostSettlement Phase = "post_settlement"
    PhaseAudit         Phase = "audit"
)
```

MVP only active：

```text
pre_request, post_auth, pre_prompt, pre_route, post_route, pre_provider, post_provider, pre_settlement, audit
```

### 7.2 Plugin interface

```go
type Plugin interface {
    Name() string
    Phase() Phase
    Validate(config json.RawMessage) error
    Execute(ctx context.Context, input PluginInput) (PluginResult, error)
}
```

### 7.3 PluginResult

```go
type PluginResult struct {
    Action          PluginAction
    ErrorCode       string
    Message         string
    Mutations       []StateMutation
    SuggestedModel  string
    SuggestedRoute  *RouteOverride
    AuditFields     map[string]string
}
```

### 7.4 PluginManager 快路径

```go
type PluginManagerImpl struct {
    chains atomic.Value // map[Phase][]CompiledPlugin
}
```

`Run` 行为：

```go
chain := m.chain(phase)
if len(chain) == 0 { return nil } // O(1) skip
for _, plugin := range chain { ... }
```

插件绑定在 snapshot 中编译成阶段链，数据面请求不做 DB 查询。

---

## 8. Built-in Plugin 设计

### 8.1 RequestSizePlugin

阶段：`pre_request`

配置：

```go
type RequestSizeConfig struct {
    MaxHeaderBytes int64
    MaxBodyBytes   int64
    MaxFileBytes   int64
    MaxFiles       int
}
```

### 8.2 PromptGuardPlugin

阶段：`pre_prompt`

职责：审核 prompt 文本、图片描述、视频描述、metadata 中的用户输入字段。

输出：

```text
allow
deny
redact
require_review
```

### 8.3 CostGuardPlugin

阶段：`post_route`

职责：根据 route plan 和 price quote 判断是否允许执行。

可以返回：

```go
PluginResult{Action: ActionDegrade, SuggestedModel: "cheap-video-720p"}
```

### 8.4 AuditLogPlugin

阶段：`audit`

职责：写 data audit event。只能写脱敏后的字段。

---

## 9. 路由架构

### 9.1 组件

```text
RoutePlanner
  -> ModelResolver
  -> PolicyResolver
  -> CandidateResolver
  -> PriceQuoter
  -> StrategyRegistry
  -> DegradationPlanner
```

### 9.2 ProviderCandidate

```go
type ProviderCandidate struct {
    ChannelID       string
    ProviderType    string
    UpstreamModel   string
    PublicModel     string
    BillingModel    string
    Priority        int
    Weight          int
    TimeoutPolicy   TimeoutPolicy
    CostEstimate    CostEstimate
    Health          HealthSignal
    Latency         LatencySignal
    Quota           QuotaSignal
    Tags            map[string]string
}
```

### 9.3 Strategy interface

```go
type CandidateSelector interface {
    Strategy() RouteStrategy
    Order(ctx context.Context, candidates []ProviderCandidate, input SelectionInput) ([]ProviderCandidate, error)
}
```

### 9.4 health_weighted

```text
score = normalized_weight
      * health_score
      * latency_score
      * cost_score
      * quota_score
```

候选排序时保留 fallback list，并把不可用候选剔除或降权。

### 9.5 degraded candidates

`DegradationPlanner` 在路由后执行，生成可选降级候选：

```text
高价视频模型 -> 低价视频模型
1080p -> 720p
10s -> 5s
同步 -> 异步排队
```

降级是否自动执行由租户策略决定。

---

## 10. 限流架构

### 10.1 LimitEnforcer

```go
type LimitEnforcer interface {
    Acquire(ctx context.Context, state *RequestState) (LimitRelease, error)
}
```

### 10.2 Redis token bucket

QPS/RPM/TPM 使用 Lua 脚本，避免多次 round-trip。

### 10.3 Redis concurrency lease

```text
ai_gateway:concurrency:{scope}:{id}
  -> sorted set(request_id, expires_at)
```

Acquire：

```text
1. remove expired leases
2. count active leases
3. if count >= limit reject
4. add request_id with ttl
```

Release：删除 request_id。

异常退出：TTL 自动释放。

---

## 11. Billing 架构

### 11.1 请求前

```text
EstimateUsage
QuotePrice
CreateBalanceHold
```

### 11.2 请求后

```text
RecordUsageAttempt
SettleUsage
AppendLedgerEntry
ReleaseHold
```

### 11.3 失败修复

失败修复使用 `failed_settlement`：

```go
type FailedSettlement struct {
    ID            string
    RequestID     string
    TenantID      string
    APIKeyID      string
    HoldID        string
    Payload       []byte
    Status        FailedSettlementStatus
    RetryCount    int
    NextRetryAt   time.Time
    LastError     string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

worker 使用 lease 拉取并重放。

---

## 12. Task 架构

### 12.1 Task 状态

```text
queued
admitted
dispatched
running
succeeded
failed
canceled
expired
```

### 12.2 Task 表关键字段

```go
type Task struct {
    ID             string
    TenantID       string
    ProjectID      string
    APIKeyID       string
    RequestID      string
    IdempotencyKey string
    Type           TaskType
    Model          string
    Status         TaskStatus
    Progress       int
    ProviderType   string
    ChannelID      string
    ProviderTaskID string
    Input          []byte
    Result         []byte
    ErrorCode      string
    ErrorMessage   string
    CallbackURL    string
    Metadata       map[string]string
    BalanceHoldID  string
    CreatedAt      time.Time
    UpdatedAt      time.Time
    CompletedAt    *time.Time
}
```

### 12.3 Worker job

```text
provider_task_poller
  -> fetch running tasks
  -> query provider
  -> update task
  -> if completed settle
  -> enqueue callback
```

---

## 13. Snapshot 架构

### 13.1 RuntimeSnapshot

```go
type RuntimeSnapshot struct {
    Version        string
    SchemaVersion  string
    Checksum       string
    CreatedAt      time.Time
    Tenants        []TenantRuntime
    APIKeys        []APIKeyRuntime
    Models         []ModelRuntime
    Providers      []ProviderRuntime
    RoutePolicies  []RoutePolicyRuntime
    PriceRules     []PriceRuleRuntime
    LimitRules     []LimitRuleRuntime
    PluginBindings []PluginBindingRuntime
    RevokedKeys    []RevokedKeyRuntime
}
```

### 13.2 Atomic store

```go
type AtomicSnapshotStore struct {
    current atomic.Pointer[IndexedRuntime]
}
```

数据面每个请求只读取 pointer，不加锁。

### 13.3 发布流程

```text
control DB transaction
  -> build snapshot
  -> validate schema
  -> write snapshot table
  -> publish Redis key + pubsub
  -> gateway receives
  -> validate checksum
  -> build index
  -> atomic swap
```

---

## 14. 文档与 ADR

所有重要架构决策写 ADR：

```text
docs/design/ai_gateway_ADR.md#adr-0001-采用四平面架构
docs/design/ai_gateway_ADR.md#adr-0002-统一-uri-但引入-apiclassifier-消歧
docs/design/ai_gateway_ADR.md#adr-0003-数据面只读-runtime-snapshot
docs/design/ai_gateway_ADR.md#adr-0004-账务使用-balance-hold--settlement--ledger
docs/design/ai_gateway_ADR.md#adr-0005-mvp-插件-phase-精简为-9-个
```

ADR 格式：

```text
Context
Decision
Consequences
Alternatives
Status
```

---

## 15. 生产边界

必须满足：

```text
无明文 API key 落库
无明文 provider key 落库
provider 成功但结算失败可修复
异步任务重复提交不重复扣费
snapshot stale 有策略
所有限流多副本语义正确
所有敏感日志经过 redaction
```
