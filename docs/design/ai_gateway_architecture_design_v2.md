# 商用 AI Gateway 架构设计文档 v0.2

版本：v0.2  
目标：给出可以直接落代码的架构、目录、包职责、核心结构体、插件链、路由和计费设计。

---

## 1. 总体架构

```text
Client / Agent / SDK
    │
    ▼
┌──────────────────────────────────────────────┐
│ Gateway Data Plane                           │
│ auth -> plugin -> route -> admission -> relay │
└──────────────────────────────────────────────┘
    │                  │
    │ provider call     │ usage/task/settlement event
    ▼                  ▼
Provider APIs      Worker / Billing / Task Plane
    │                  │
    ▼                  ▼
LLM / Image / Video    Ledger / Usage / Callback / Repair
```

系统分四个平面：

| 平面 | 职责 | 进程 |
|---|---|---|
| Data Plane | 高并发请求入口、鉴权、路由、限流、provider relay、stream accounting | `cmd/gateway` |
| Control Plane | 租户、模型、供应商、路由、价格、限流、插件配置、快照发布 | `cmd/control-api` / `cmd/configd` |
| Billing & Task Plane | 余额、预占、结算、ledger、异步媒体任务、provider polling、callback | `cmd/worker` |
| Observability & Security | metrics、logs、tracing、audit、redaction、security events | shared libs + infra |

---

## 2. 代码分层

采用分层架构：

```text
transport -> application -> domain <- infra/adapter
                    │
                    └── dataplane engine
```

规则：

1. `domain` 不依赖 HTTP、Gin、chi、SQL、Redis。
2. `application` 编排 usecase，可以依赖 domain interface。
3. `infra` 实现 repository、Redis、object storage、KMS、metrics。
4. `provider` adapter 是外部供应商协议翻译层。
5. `dataplane` 是请求热路径，必须少查库、少分配、可观测、可补偿。
6. `transport` 只做协议解析和响应写回。

---

## 3. 推荐目录结构

```text
cmd/
  gateway/
    main.go
  control-api/
    main.go
  worker/
    main.go
  configd/
    main.go
  admin-cli/
    main.go

configs/
  local.yaml
  production.example.yaml
  test.yaml

deployments/
  Dockerfile
  docker-compose.yml
  helm/

migrations/
  000001_init_identity.sql
  000002_init_model_catalog.sql
  000003_init_provider.sql
  000004_init_routing.sql
  000005_init_billing.sql
  000006_init_tasks.sql
  000007_init_audit.sql
  000008_init_outbox.sql

internal/
  bootstrap/
  transport/
  app/
  domain/
  dataplane/
  controlplane/
  provider/
  billing/
  task/
  infra/
  worker/

pkg/
  apperr/
  money/
  ptr/
  xcontext/
  xtime/
  redaction/
  tokenusage/
```

---

## 4. `cmd` 目录

### 4.1 `cmd/gateway/main.go`

职责：

```text
加载配置
初始化 logger / tracing / metrics
初始化 Redis / snapshot reader / limit store
构建 GatewayEngine
注册 HTTP routes
优雅启动和关闭
```

### 4.2 `cmd/control-api/main.go`

职责：

```text
加载配置
初始化 DB / repository / usecase
注册 admin routes
提供配置管理、租户管理、模型管理、价格管理、路由管理、任务管理
```

### 4.3 `cmd/worker/main.go`

职责：

```text
启动 job scheduler
执行 provider task polling
执行 failed settlement replay
执行 outbox projection
执行 expired hold release
执行 callback retry
执行 snapshot consistency check
```

### 4.4 `cmd/configd/main.go`

MVP 可并入 control-api。独立后职责：

```text
生成 runtime snapshot
校验 snapshot
发布 snapshot
推送/广播配置变更
提供 gateway watch 接口
```

---

## 5. `internal/bootstrap`

负责进程装配，不写业务逻辑。

```text
internal/bootstrap/
  common.go              通用资源：logger、db、redis、metrics、tracing、clock
  gateway.go             构建 gateway data plane
  control.go             构建 control api
  worker.go              构建 worker runner
  configd.go             构建 configd
  db.go                  DB 初始化
  redis.go               Redis 初始化
  telemetry.go           OTel 初始化
  cleanup.go             资源释放
```

关键结构：

```go
type Common struct {
    Config   conf.Config
    Logger   log.Logger
    DB       *sql.DB
    Redis    redis.Client
    Metrics  metrics.Registry
    Tracer   trace.TracerProvider
    Clock    clock.Clock
    KMS      secret.KMS
}
```

---

## 6. `internal/transport`

HTTP 协议层，严格不写领域规则。

```text
internal/transport/httpserver/
  server.go              http.Server 构造
  router.go              路由注册
  middleware_request_id.go
  middleware_access_log.go
  middleware_recovery.go
  middleware_metrics.go
  middleware_tracing.go
  response.go            JSON/SSE/error 写回

internal/transport/gatewayhttp/
  routes.go              对外 gateway routes
  handler_openai.go      OpenAI 兼容接口
  handler_claude.go      Claude 兼容接口
  handler_gemini.go      Gemini 兼容接口
  handler_media.go       images/videos/audio/music 统一接口
  handler_tasks.go       task 查询/取消
  handler_files.go       file upload
  request_decoder.go     HTTP -> IncomingRequest
  response_writer.go     GatewayResponse -> HTTP
  stream_writer.go       SSE/chunk stream 写回
  error_mapper.go        apperr -> provider-compatible error response

internal/transport/controlhttp/
  routes.go
  handler_tenant.go
  handler_project.go
  handler_api_key.go
  handler_model.go
  handler_provider.go
  handler_route.go
  handler_price.go
  handler_limit.go
  handler_plugin.go
  handler_snapshot.go
  handler_audit.go
```

`gatewayhttp` 输出统一 `dataplane.Request`，不要让核心层知道 Gin/chi。

---

## 7. `internal/domain`

领域层只定义实体、值对象、领域服务和 repository interface。

```text
internal/domain/
  tenant/
  identity/
  modelcatalog/
  provider/
  routing/
  pricing/
  billing/
  task/
  audit/
  policy/
  fileasset/
```

### 7.1 tenant

```text
tenant/
  entity.go              Tenant, Account
  repository.go
  service.go             tenant invariant
  errors.go
```

核心结构：

```go
type Tenant struct {
    ID        string
    Name      string
    Status    TenantStatus
    Plan      string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Account struct {
    ID                  string
    TenantID            string
    Name                string
    SettlementCurrency  string
    Status              AccountStatus
    CreatedAt           time.Time
    UpdatedAt           time.Time
}
```

### 7.2 identity

```text
identity/
  api_key.go
  principal.go
  repository.go
  service.go
  password.go
  errors.go
```

```go
type APIKey struct {
    ID                 string
    TenantID           string
    AccountID          string
    ProjectID          string
    Name               string
    KeyPrefix          string
    KeyHash            string
    Status             APIKeyStatus
    Scopes             []string
    AllowIPs           []string
    ModelAllowlist     []string
    ExpiresAt          *time.Time
    RateLimitProfileID *string
    CreatedAt          time.Time
    UpdatedAt          time.Time
}

type Principal struct {
    Tenant  TenantRef
    Account AccountRef
    Project ProjectRef
    APIKey  APIKeyRef
    User    *UserRef
}
```

### 7.3 modelcatalog

```text
modelcatalog/
  model.go
  capability.go
  schema.go
  alias.go
  repository.go
  service.go
```

```go
type PublicModel struct {
    ID            string
    Name          string
    DisplayName   string
    Family        string
    Capability    Capability
    InputModalities  []Modality
    OutputModalities []Modality
    TaskMode      TaskMode // sync async both
    SupportsStream bool
    ContextWindow int
    MaxOutputTokens int
    Status        ModelStatus
    SchemaID      string
}

type ModelSchema struct {
    ID          string
    ModelName   string
    Version     string
    JSONSchema  []byte
    Defaults    map[string]any
    Required    []string
}
```

### 7.4 provider domain

```text
provider/
  provider.go
  channel.go
  credential.go
  mapping.go
  health.go
  repository.go
```

```go
type ProviderChannel struct {
    ID              string
    TenantID         string
    ProviderType     string
    Name             string
    BaseURL          string
    CredentialRef    CredentialRef
    Priority         int
    Weight           int
    TimeoutPolicy    TimeoutPolicy
    StreamTimeoutPolicy TimeoutPolicy
    Status           ChannelStatus
    Region           string
    CostProfileID    *string
    HealthPolicyID   *string
}

type ProviderModelMapping struct {
    ID              string
    PublicModelID   string
    ChannelID        string
    UpstreamModel    string
    Capability       Capability
    SupportsStream   bool
    MaxInputTokens   int
    MaxOutputTokens  int
    Status           MappingStatus
}
```

### 7.5 routing domain

```text
routing/
  policy.go
  decision.go
  candidate.go
  strategy.go
  selector.go
  signal.go
  repository.go
```

```go
type RoutePolicy struct {
    ID             string
    TenantID        string
    AccountID       *string
    ProjectID       *string
    ModelID         string
    Strategy        RouteStrategy
    MaxRetries      int
    MaxFallbacks    int
    TimeoutPolicy   TimeoutPolicy
    RetryPolicy     RetryPolicy
    FallbackPolicy  FallbackPolicy
    BillingMode     BillingMode
    Status          PolicyStatus
}

type RouteDecision struct {
    RequestedModel string
    PublicModel    PublicModelRef
    BillingModel   PublicModelRef
    Strategy       RouteStrategy
    Candidates     []RouteCandidate
    MaxRetries     int
    MaxFallbacks   int
    Reason         string
    EstimatedUsage UsageEstimate
    EstimatedCost  Money
}
```

### 7.6 pricing / billing

```text
pricing/
  price_rule.go
  estimator.go
  repository.go

billing/
  balance.go
  hold.go
  usage_attempt.go
  usage_record.go
  ledger.go
  settlement.go
  failed_settlement.go
  repository.go
```

```go
type BalanceHold struct {
    ID             string
    RequestID      string
    AccountID      string
    ProjectID      string
    APIKeyID        string
    AmountMicros    int64
    Currency        string
    Status          HoldStatus
    ExpiresAt       time.Time
    CreatedAt       time.Time
    ReleasedAt      *time.Time
    SettledAt       *time.Time
}

type UsageAttempt struct {
    ID             string
    RequestID      string
    AttemptIndex   int
    TenantID       string
    AccountID      string
    ProjectID      string
    APIKeyID       string
    ProviderType   string
    ChannelID      string
    PublicModel    string
    UpstreamModel  string
    Status         AttemptStatus
    HTTPStatus     int
    ErrorCode      string
    Stream         bool
    FirstTokenMS   *int
    DurationMS     int
    Usage          UsageQuantities
    StartedAt      time.Time
    EndedAt        *time.Time
}
```

---

## 8. `internal/dataplane`

数据面是请求热路径。

```text
internal/dataplane/
  engine/
  request/
  response/
  auth/
  classifier/
  parser/
  plugin/
  routing/
  admission/
  limit/
  dispatch/
  stream/
  settlement/
  taskbridge/
  observe/
  errors/
```

### 8.1 GatewayEngine 总结构

```go
type GatewayEngine struct {
    cfg       EngineConfig
    clock     clock.Clock
    ids       idgen.Generator
    logger    log.Logger
    tracer    trace.Tracer
    metrics   observe.Metrics

    snapshot  SnapshotProvider
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
    task       TaskBridge
    files      FileService
    audit      Auditor
    redactor   Redactor
    errors     ErrorMapper
}
```

字段说明：

| 字段 | 类型 | 职责 |
|---|---|---|
| `cfg` | `EngineConfig` | 热路径配置，超时、body 限制、插件开关 |
| `clock` | `clock.Clock` | 统一时间源，方便测试 |
| `ids` | `idgen.Generator` | request_id/task_id/attempt_id 生成 |
| `snapshot` | `SnapshotProvider` | 读取本地/Redis 运行时快照 |
| `classifier` | `APIClassifier` | 从 method/path/content-type 判断 canonical API |
| `parser` | `RequestParser` | 解析 body、model、stream、usage estimate |
| `auth` | `Authenticator` | API key/JWT/mTLS 等认证 |
| `policy` | `PolicyEvaluator` | 模型权限、租户策略、IP allowlist |
| `plugins` | `PluginManager` | 执行插件链 |
| `router` | `RoutePlanner` | 生成 RoutePlan |
| `admission` | `AdmissionController` | 价格预估、余额预占 |
| `limiter` | `LimitEnforcer` | QPS/TPM/并发/成本限制 |
| `dispatcher` | `ProviderDispatcher` | provider attempt、retry、fallback |
| `stream` | `StreamFinalizer` | 流式响应 close-time accounting |
| `settlement` | `SettlementService` | 用量结算、ledger、failed settlement |
| `task` | `TaskBridge` | 异步媒体任务创建和 provider task dispatch |
| `files` | `FileService` | 文件上传、素材管理 |
| `audit` | `Auditor` | 审计事件 |
| `redactor` | `Redactor` | 日志/错误/审计脱敏 |
| `errors` | `ErrorMapper` | 内部错误转外部协议错误 |

### 8.2 EngineConfig

```go
type EngineConfig struct {
    ServiceName              string
    MaxRequestBodyBytes      int64
    MaxPromptTokens          int
    DefaultProviderTimeout   time.Duration
    DefaultStreamIdleTimeout time.Duration
    DefaultStreamMaxDuration time.Duration
    EnableDiagnosticHeaders  bool
    EnableQueryAPIKey        bool
    EnableGeminiQueryAPIKey  bool
    TrustedProxies           []string
    FailOpenOnAuditError     bool
    FailOpenOnMetricsError   bool
}
```

### 8.3 RequestState

请求在数据面内部流转的唯一状态对象。

```go
type RequestState struct {
    RequestID    string
    TraceID      string
    StartedAt    time.Time

    Transport    TransportInfo
    Operation    OperationInfo
    Principal    *Principal
    Snapshot     *RuntimeSnapshot

    Parsed       ParsedRequest
    RoutePlan    *RoutePlan
    Admission    *AdmissionResult
    LimitRelease LimitRelease

    ProviderAttempts []ProviderAttemptResult
    ProviderResponse *ProviderResponse
    StreamResponse   ProviderStream

    UsageEstimate UsageQuantities
    FinalUsage     UsageQuantities
    CostEstimate   Money
    FinalCost      Money

    Task           *TaskRef
    Files          []FileRef
    Metadata       map[string]string

    PluginData     map[string]any
    AuditEvents    []AuditEvent
}
```

### 8.4 GatewayEngine.Handle 主流程

```go
func (e *GatewayEngine) Handle(ctx context.Context, req IncomingRequest) (*GatewayResponse, error) {
    state := e.newState(req)

    if err := e.plugins.Run(ctx, PhasePreRequest, state); err != nil { return nil, err }
    if err := e.classifier.Classify(ctx, state); err != nil { return nil, err }
    if err := e.parser.Parse(ctx, state); err != nil { return nil, err }
    if err := e.auth.Authenticate(ctx, state); err != nil { return nil, err }
    if err := e.plugins.Run(ctx, PhaseAuthenticated, state); err != nil { return nil, err }
    if err := e.policy.Evaluate(ctx, state); err != nil { return nil, err }
    if err := e.plugins.Run(ctx, PhasePrePrompt, state); err != nil { return nil, err }
    if err := e.router.Plan(ctx, state); err != nil { return nil, err }
    if err := e.plugins.Run(ctx, PhasePostRoute, state); err != nil { return nil, err }
    if err := e.admission.Admit(ctx, state); err != nil { return nil, err }
    if err := e.limiter.Acquire(ctx, state); err != nil { return nil, e.compensate(ctx, state, err) }

    result, err := e.dispatcher.Dispatch(ctx, state)
    if err != nil { return nil, e.compensate(ctx, state, err) }

    if result.Stream != nil {
        return e.stream.Wrap(ctx, state, result), nil
    }

    if err := e.plugins.Run(ctx, PhasePreResponse, state); err != nil { return nil, err }
    if err := e.settlement.Settle(ctx, state); err != nil { return nil, e.repair(ctx, state, err) }
    _ = e.plugins.Run(ctx, PhaseAudit, state)
    return e.buildResponse(state), nil
}
```

---

## 9. 插件架构

### 9.1 插件目录

```text
internal/dataplane/plugin/
  manager.go
  phase.go
  plugin.go
  context.go
  registry.go
  config.go
  result.go

internal/dataplane/plugin/builtin/
  auth_api_key.go
  ip_allowlist.go
  model_acl.go
  request_size.go
  prompt_token_limit.go
  pii_redaction.go
  prompt_guard.go
  response_guard.go
  rate_limit.go
  cost_guard.go
  audit_log.go
  llm_metrics.go
  callback.go
```

### 9.2 Phase

```go
type Phase string

const (
    PhasePreRequest      Phase = "pre_request"
    PhaseCredential      Phase = "credential_extract"
    PhaseAuthenticated   Phase = "authenticated"
    PhasePrePolicy       Phase = "pre_policy"
    PhasePrePrompt       Phase = "pre_prompt"
    PhasePostPrompt      Phase = "post_prompt"
    PhasePreRoute        Phase = "pre_route"
    PhasePostRoute       Phase = "post_route"
    PhasePreAdmission    Phase = "pre_admission"
    PhasePostAdmission   Phase = "post_admission"
    PhasePreProvider     Phase = "pre_provider"
    PhasePostProvider    Phase = "post_provider"
    PhaseStreamChunk     Phase = "stream_chunk"
    PhasePreResponse     Phase = "pre_response"
    PhasePostResponse    Phase = "post_response"
    PhasePreSettlement   Phase = "pre_settlement"
    PhasePostSettlement  Phase = "post_settlement"
    PhaseAudit           Phase = "audit"
)
```

### 9.3 插件接口

```go
type Plugin interface {
    Name() string
    Version() string
    Phases() []Phase
    Init(ctx context.Context, cfg PluginConfig) error
    Execute(ctx context.Context, input PluginInput) (PluginResult, error)
}

type PluginInput struct {
    Phase   Phase
    State   *RequestState
    Config  map[string]any
    Logger  log.Logger
    Metrics observe.Metrics
}

type PluginResult struct {
    Continue        bool
    Deny            *DenyResponse
    Mutations       []Mutation
    AuditEvents     []AuditEvent
    Tags            map[string]string
    RedactedFields   []string
}
```

### 9.4 PluginManager

```go
type PluginManager struct {
    registry PluginRegistry
    bindings PluginBindingResolver
    metrics  PluginMetrics
    logger   log.Logger
}

func (m *PluginManager) Run(ctx context.Context, phase Phase, state *RequestState) error {
    plugins := m.bindings.Resolve(state.Snapshot, state.Principal, state.Operation, phase)
    for _, binding := range plugins {
        plugin := m.registry.Get(binding.PluginName)
        result, err := plugin.Execute(ctx, PluginInput{Phase: phase, State: state, Config: binding.Config})
        if err != nil { return err }
        applyPluginResult(state, result)
        if !result.Continue { return result.Deny.AsError() }
    }
    return nil
}
```

### 9.5 插件绑定

插件配置来自 runtime snapshot：

```go
type PluginBinding struct {
    ID          string
    TenantID    string
    AccountID   *string
    ProjectID   *string
    APIKeyID     *string
    ModelID     *string
    RouteID     *string
    PluginName  string
    Phase       Phase
    Priority    int
    Enabled     bool
    Config      map[string]any
}
```

执行顺序：

```text
scope specificity desc -> priority asc -> plugin name asc
```

---

## 10. 内置插件设计

### 10.1 APIKeyAuthPlugin

阶段：`credential_extract`, `authenticated`

职责：

```text
提取 Authorization Bearer
提取 x-api-key
提取 x-goog-api-key
可配置是否允许 query key
冲突凭证拒绝
prefix 查找
hash 校验
状态/过期校验
写入 Principal
```

### 10.2 PromptGuardPlugin

阶段：`pre_prompt`

配置：

```go
type PromptGuardConfig struct {
    DenyPatterns       []string
    AllowPatterns      []string
    MaxPromptChars     int
    MaxPromptTokens    int
    BlockPII           bool
    BlockJailbreak     bool
    Provider           string // local / remote
    FailOpen           bool
}
```

输出：

```text
allow / deny / redact / tag
```

### 10.3 PiiRedactionPlugin

阶段：`pre_prompt`, `pre_response`, `audit`

职责：

```text
检测 email/phone/id_card/address/api_key/url token
对日志和审计内容脱敏
可选对发给 provider 的 prompt 脱敏
```

### 10.4 CostGuardPlugin

阶段：`pre_admission`

职责：

```text
按租户/项目/API key 限制日成本/月成本/单请求成本
超过预算拒绝或降级到低成本模型
```

### 10.5 LLMMetricPlugin

阶段：`post_provider`, `post_settlement`

职责：

```text
记录 input_tokens/output_tokens/reasoning_tokens/cache_tokens
记录 provider latency/first_token_latency
记录 cost_micros
记录 retry/fallback
```

### 10.6 CallbackPlugin

阶段：`post_settlement`

职责：

```text
对异步任务发送 callback
失败写 callback_outbox
worker 重试
签名 callback payload
```

---

## 11. 路由架构

### 11.1 路由包目录

```text
internal/dataplane/routing/
  planner.go
  request.go
  decision.go
  candidate_resolver.go
  policy_resolver.go
  strategy_registry.go
  signal_provider.go
  selectors/
    priority.go
    weighted_random.go
    round_robin.go
    consistent_hash.go
    least_latency.go
    least_cost.go
    health_weighted.go
    quota_aware.go
    semantic.go
```

### 11.2 RoutePlanner

```go
type RoutePlanner struct {
    policyResolver    PolicyResolver
    modelResolver     ModelResolver
    candidateResolver CandidateResolver
    strategyRegistry  StrategyRegistry
    signalProvider    RouteSignalProvider
    priceEstimator    PriceEstimator
    clock             clock.Clock
}
```

### 11.3 CandidateResolver

职责：

```text
根据 public model 找到可用 provider mappings
过滤 disabled channel
过滤 capability 不匹配
过滤 stream 不支持
过滤 region 不匹配
过滤 provider support 不匹配
过滤 credential 不可用
补齐 priority / weight / cost / timeout
```

### 11.4 RouteSignalProvider

```go
type RouteSignalProvider interface {
    ChannelHealth(ctx context.Context, channelID string) HealthSignal
    ChannelLatency(ctx context.Context, channelID string, model string) LatencySignal
    ChannelCost(ctx context.Context, channelID string, model string, usage UsageEstimate) CostSignal
    ChannelQuota(ctx context.Context, channelID string) QuotaSignal
}
```

### 11.5 策略接口

```go
type CandidateSelector interface {
    Strategy() RouteStrategy
    Order(ctx context.Context, candidates []RouteCandidate, input SelectionInput) ([]RouteCandidate, error)
}

type SelectionInput struct {
    TenantID      string
    ProjectID     string
    APIKeyID       string
    RequestedModel string
    SelectionKey   string
    UsageEstimate  UsageEstimate
    Signals        map[string]RouteSignal
}
```

### 11.6 策略说明

| 策略 | 说明 | 适用场景 |
|---|---|---|
| `priority` | priority 小的优先 | 主备线路 |
| `weighted_random` | 同优先级内按权重随机 | 成本/额度分摊 |
| `round_robin` | 同组轮询 | 简单负载均衡 |
| `consistent_hash` | 按 selection key 粘性 | Agent 会话、缓存命中 |
| `least_latency` | 按实时延迟最低 | 低延迟业务 |
| `least_cost` | 按预计成本最低 | 低成本转售 |
| `health_weighted` | 健康分和权重综合 | 商用默认推荐 |
| `quota_aware` | 避免 quota 快耗尽渠道 | 多供应商额度管理 |
| `semantic_route` | 按 prompt 语义选择模型 | Agent 智能路由 |

### 11.7 推荐默认策略

商用默认：

```text
health_weighted
```

排序公式示例：

```text
score = base_weight
      * health_score
      * quota_score
      * latency_score
      * cost_score
      * capability_score
```

其中：

```text
health_score: 0.0 - 1.0
quota_score:  0.0 - 1.0
latency_score: p95 越小越高
cost_score:  单价越低越高
capability_score: 完全匹配为 1，降级匹配为 0.5
```

---

## 12. Provider Dispatch 架构

```text
internal/dataplane/dispatch/
  dispatcher.go
  attempt.go
  retry.go
  fallback.go
  circuit.go
  timeout.go
  error_classifier.go
```

```go
type ProviderDispatcher struct {
    registry       ProviderRegistry
    retryPolicy    RetryPolicyEvaluator
    fallbackPolicy FallbackPolicyEvaluator
    circuit        CircuitBreaker
    limiter        ChannelLimitEnforcer
    observer       ProviderObserver
}
```

Attempt 流程：

```text
for candidate in candidates limited by max_fallbacks:
  for retry <= max_retries:
    check circuit
    acquire channel limit
    call adapter
    classify error
    observe attempt
    if success return
    if retryable wait backoff+jitter
  if fallbackable continue next candidate
return final error
```

---

## 13. 异步任务架构

```text
internal/task/
  task.go
  service.go
  repository.go
  scheduler.go
  provider_task.go
  callback.go
  result_normalizer.go
  errors.go

internal/worker/jobs/
  provider_task_poller.go
  callback_retry.go
  expired_task_reaper.go
```

TaskService：

```go
type TaskService struct {
    repo        TaskRepository
    dispatcher  ProviderTaskDispatcher
    admission   AdmissionController
    settlement  SettlementService
    files       FileService
    callbacks   CallbackOutbox
    clock       clock.Clock
}
```

Task 创建：

```text
validate model schema
normalize input files
estimate cost
reserve balance
create internal task
send provider async request
save external_task_id
return task object
```

---

## 14. 账务架构

```text
internal/billing/
  balance_service.go
  hold_service.go
  settlement_service.go
  ledger_service.go
  usage_service.go
  failed_settlement_service.go
  reconciliation_service.go
```

结算必须满足：

```text
幂等
事务内写 usage_record + ledger_entry + hold status
失败写 failed_settlement
worker 可重放
可对账
```

推荐事务边界：

```text
BEGIN
  SELECT balance FOR UPDATE
  SELECT hold FOR UPDATE
  INSERT usage_record ON CONFLICT DO NOTHING
  INSERT ledger_entry ON CONFLICT DO NOTHING
  UPDATE balance
  UPDATE hold SET settled
COMMIT
```

---

## 15. Snapshot 架构

```text
internal/controlplane/snapshot/
  builder.go
  validator.go
  publisher.go
  codec.go
  watcher.go
  rollout.go
  rollback.go

internal/dataplane/snapshot/
  reader.go
  cache.go
  index.go
  watch.go
```

RuntimeSnapshot：

```go
type RuntimeSnapshot struct {
    Version       string
    SchemaVersion string
    Checksum      string
    CreatedAt     time.Time

    Tenants       []RuntimeTenant
    Accounts      []RuntimeAccount
    Projects      []RuntimeProject
    APIKeys       []RuntimeAPIKey
    Models        []RuntimeModel
    ModelAliases  []RuntimeModelAlias
    Providers     []RuntimeProvider
    Channels      []RuntimeChannel
    ModelMappings []RuntimeModelMapping
    RoutePolicies []RuntimeRoutePolicy
    PriceRules    []RuntimePriceRule
    LimitRules    []RuntimeLimitRule
    PluginBindings []RuntimePluginBinding
}
```

数据面加载后建立索引：

```text
api_key_prefix -> APIKey
model_name -> Model
alias -> model_name
model_id -> mappings[]
route_policy by model/account/project
price_rule by model/currency/api
limit_rule by scope
plugin_binding by phase/scope
```

---

## 16. 错误体系

```go
type AppError struct {
    Code       string
    Message    string
    HTTPStatus int
    Type       string
    Retryable  bool
    Temporary  bool
    Safe       bool
    Cause      error
}
```

错误类别：

```text
invalid_request
unauthorized
forbidden
not_found
rate_limited
quota_exceeded
insufficient_balance
policy_denied
provider_error
provider_timeout
provider_rate_limited
provider_unavailable
settlement_failed
config_unavailable
internal_error
```

---

## 17. 商用设计底线

1. Provider 成功后，本地结算失败必须可修复。
2. API key、provider key、prompt、response 默认不得进入日志。
3. 每个请求必须有 request_id 和 trace_id。
4. 每个 provider attempt 必须有记录。
5. 每个扣费必须有 ledger entry。
6. 每次配置发布必须有 snapshot version 和 audit。
7. 多副本并发限制不能用本地内存假装全局限制。
8. 流式响应必须在 close 时结算。
