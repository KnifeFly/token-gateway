# 商用 AI Gateway 代码蓝图 v0.2

版本：v0.2  
用途：直接指导代码落地。本文聚焦 package、文件、接口、结构体字段和核心流程。

---

## 1. 推荐 Go Module

```text
module github.com/your-org/ai-gateway

go 1.22+
```

建议依赖：

```text
chi 或 gin：HTTP router
pgx 或 go-sql-driver/mysql：数据库
redis/go-redis：Redis
prometheus/client_golang：指标
go.opentelemetry.io/otel：Tracing
zap 或 slog：日志
yaml.v3：配置
```

---

## 2. 最小可编译目录

第一天先创建这些目录：

```text
cmd/gateway/main.go
cmd/control-api/main.go
cmd/worker/main.go

internal/bootstrap/common.go
internal/bootstrap/gateway.go
internal/bootstrap/control.go
internal/bootstrap/worker.go

internal/infra/conf/config.go
internal/infra/log/logger.go
internal/infra/db/db.go
internal/infra/redis/client.go
internal/infra/metrics/metrics.go
internal/infra/tracing/tracing.go

internal/dataplane/engine/engine.go
internal/dataplane/request/request.go
internal/dataplane/response/response.go
internal/dataplane/plugin/plugin.go
internal/dataplane/routing/planner.go
internal/provider/relay/types.go

pkg/apperr/errors.go
```

先让项目能：

```text
go test ./...
go run ./cmd/gateway -config configs/local.yaml
GET /healthz
GET /readyz
GET /metrics
```

---

## 3. 配置结构

文件：`internal/infra/conf/config.go`

```go
type Config struct {
    Environment string         `yaml:"environment"`
    Service     ServiceConfig  `yaml:"service"`
    HTTP        HTTPConfig     `yaml:"http"`
    Database    DatabaseConfig `yaml:"database"`
    Redis       RedisConfig    `yaml:"redis"`
    Security    SecurityConfig `yaml:"security"`
    Gateway     GatewayConfig  `yaml:"gateway"`
    Provider    ProviderConfig `yaml:"provider"`
    Billing     BillingConfig  `yaml:"billing"`
    Worker      WorkerConfig   `yaml:"worker"`
    Telemetry   TelemetryConfig `yaml:"telemetry"`
    Log         LogConfig      `yaml:"log"`
}

type HTTPConfig struct {
    Addr              string        `yaml:"addr"`
    ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
    ReadTimeout       time.Duration `yaml:"read_timeout"`
    WriteTimeout      time.Duration `yaml:"write_timeout"`
    IdleTimeout       time.Duration `yaml:"idle_timeout"`
    ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
    MaxHeaderBytes    int           `yaml:"max_header_bytes"`
}

type GatewayConfig struct {
    SnapshotRefreshInterval time.Duration `yaml:"snapshot_refresh_interval"`
    MaxRequestBodyBytes     int64         `yaml:"max_request_body_bytes"`
    MaxPromptTokens         int           `yaml:"max_prompt_tokens"`
    DiagnosticHeaders       bool          `yaml:"diagnostic_headers"`
    QueryAPIKeyEnabled      bool          `yaml:"query_api_key_enabled"`
    GeminiQueryAPIKeyEnabled bool         `yaml:"gemini_query_api_key_enabled"`
    TrustedProxies          []string      `yaml:"trusted_proxies"`
    Stream                  StreamConfig  `yaml:"stream"`
    Plugins                 PluginConfig  `yaml:"plugins"`
}

type StreamConfig struct {
    HeaderTimeout time.Duration `yaml:"header_timeout"`
    IdleTimeout   time.Duration `yaml:"idle_timeout"`
    MaxDuration   time.Duration `yaml:"max_duration"`
}
```

实现要求：

```text
Default()
Load(path string)
ApplyEnv()
Normalize()
Validate()
```

---

## 4. 统一错误包

文件：`pkg/apperr/errors.go`

```go
type Error struct {
    Code       string
    Message    string
    HTTPStatus int
    Type       string
    Retryable  bool
    Temporary  bool
    Safe       bool
    Cause      error
}

func (e *Error) Error() string
func (e *Error) Unwrap() error
func New(code string, message string, status int) *Error
func Wrap(err error, code string, message string, status int) *Error
```

预定义错误：

```go
var (
    ErrInvalidRequest      = New("invalid_request", "invalid request", 400)
    ErrUnauthorized        = New("unauthorized", "unauthorized", 401)
    ErrForbidden           = New("forbidden", "forbidden", 403)
    ErrNotFound            = New("not_found", "not found", 404)
    ErrRateLimited         = New("rate_limited", "rate limited", 429)
    ErrQuotaExceeded       = New("quota_exceeded", "quota exceeded", 429)
    ErrInsufficientBalance = New("insufficient_balance", "insufficient balance", 402)
    ErrProviderUnavailable = New("provider_unavailable", "provider unavailable", 503)
    ErrSettlementFailed    = New("settlement_failed", "settlement failed", 500)
    ErrConfigUnavailable   = New("config_unavailable", "config unavailable", 503)
)
```

---

## 5. Data Plane Request / Response

目录：`internal/dataplane/request`

```go
type IncomingRequest struct {
    Method        string
    Path          string
    RawQuery      string
    Headers       map[string][]string
    Body          BodySource
    RemoteAddr    string
    ContentLength int64
    ReceivedAt    time.Time
}

type BodySource interface {
    Open() (io.ReadCloser, error)
    Size() int64
    Cleanup() error
}

type OperationInfo struct {
    CanonicalAPI    CanonicalAPI
    RequestedModel  string
    RequireStream   bool
    TaskMode        TaskMode
    InputModalities []Modality
    OutputModality  Modality
}

type ParsedRequest struct {
    Model           string
    Prompt          string
    Messages        []Message
    Contents        []GeminiContent
    Stream          bool
    ImageURLs       []string
    VideoURLs       []string
    AudioURLs       []string
    Files           []FileRef
    ModelParams     map[string]any
    Metadata        map[string]string
    EstimatedUsage  UsageQuantities
}
```

目录：`internal/dataplane/response`

```go
type GatewayResponse struct {
    StatusCode int
    Headers    map[string][]string
    Body       []byte
    Stream     ProviderStream
    Usage      UsageQuantities
    Task       *TaskObject
    Error      *ErrorObject
}

type ErrorObject struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Type    string `json:"type"`
}
```

---

## 6. GatewayEngine 详细结构

文件：`internal/dataplane/engine/engine.go`

```go
type GatewayEngine struct {
    cfg     EngineConfig
    deps    EngineDependencies

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
    tasks      TaskBridge
    files      FileService
    audit      Auditor
    redactor   Redactor
    errmap     ErrorMapper
}

type EngineDependencies struct {
    Clock   Clock
    IDGen   IDGenerator
    Logger  Logger
    Metrics Metrics
    Tracer  trace.Tracer
}
```

### 6.1 SnapshotProvider

```go
type SnapshotProvider interface {
    Current(ctx context.Context) (*RuntimeSnapshot, error)
    CurrentIndexed(ctx context.Context) (*IndexedRuntimeSnapshot, error)
    Version() string
    Staleness() time.Duration
}
```

### 6.2 APIClassifier

```go
type APIClassifier interface {
    Classify(method string, path string, headers map[string][]string) (APIClassification, error)
}

type APIClassification struct {
    CanonicalAPI CanonicalAPI
    Protocol     Protocol // openai claude gemini unified
    Endpoint     string
    Native       bool
    RequiresBody bool
    Async        bool
}
```

文件划分：

```text
internal/dataplane/classifier/classifier.go
internal/dataplane/classifier/openai.go
internal/dataplane/classifier/claude.go
internal/dataplane/classifier/gemini.go
internal/dataplane/classifier/unified_media.go
```

### 6.3 RequestParser

```go
type RequestParser interface {
    Parse(ctx context.Context, state *RequestState) error
}
```

内部组件：

```go
type RequestParserImpl struct {
    bodyStore       BodyStore
    modelExtractor  ModelExtractor
    usageEstimator  UsageEstimator
    schemaValidator ModelSchemaValidator
}
```

文件划分：

```text
internal/dataplane/parser/parser.go
internal/dataplane/parser/body_store.go
internal/dataplane/parser/openai_parser.go
internal/dataplane/parser/claude_parser.go
internal/dataplane/parser/gemini_parser.go
internal/dataplane/parser/media_parser.go
internal/dataplane/parser/usage_estimator.go
internal/dataplane/parser/schema_validator.go
```

### 6.4 Authenticator

```go
type Authenticator interface {
    Authenticate(ctx context.Context, state *RequestState) error
}

type AuthenticatorImpl struct {
    snapshot SnapshotProvider
    hasher   APIKeyHasher
    extractor CredentialExtractor
    observer AuthObserver
}
```

认证结果写入：

```go
state.Principal = &Principal{
    TenantID:  apiKey.TenantID,
    AccountID: apiKey.AccountID,
    ProjectID: apiKey.ProjectID,
    APIKeyID:  apiKey.ID,
    Scopes:    apiKey.Scopes,
}
```

### 6.5 PolicyEvaluator

```go
type PolicyEvaluator interface {
    Evaluate(ctx context.Context, state *RequestState) error
}

type PolicyEvaluatorImpl struct {
    modelACL     ModelACLChecker
    ipChecker    IPAllowlistChecker
    tenantPolicy TenantPolicyChecker
    pluginPolicy PluginPolicyChecker
}
```

### 6.6 RoutePlanner

```go
type RoutePlanner interface {
    Plan(ctx context.Context, state *RequestState) error
}
```

实现字段：

```go
type RoutePlannerImpl struct {
    snapshot          SnapshotProvider
    policyResolver    PolicyResolver
    modelResolver     ModelResolver
    candidateResolver CandidateResolver
    strategyRegistry  StrategyRegistry
    signalProvider    RouteSignalProvider
    priceEstimator    PriceEstimator
    clock             Clock
}
```

### 6.7 AdmissionController

```go
type AdmissionController interface {
    Admit(ctx context.Context, state *RequestState) error
    Release(ctx context.Context, state *RequestState, reason string) error
}

type AdmissionControllerImpl struct {
    priceEstimator PriceEstimator
    billing        BillingGateway
    idempotency    IdempotencyStore
    clock          Clock
}
```

`Admit` 做：

```text
计算预估费用
创建 balance_hold
写 state.Admission
失败时返回 insufficient_balance / price_unavailable
```

### 6.8 LimitEnforcer

```go
type LimitEnforcer interface {
    Acquire(ctx context.Context, state *RequestState) error
}

type LimitRelease interface {
    Release(ctx context.Context) error
}
```

实现字段：

```go
type LimitEnforcerImpl struct {
    ruleMatcher       LimitRuleMatcher
    tokenBucketStore  TokenBucketStore
    concurrencyStore  ConcurrencyLeaseStore
    localDenyCache    DenyCache
    clock             Clock
}
```

### 6.9 ProviderDispatcher

```go
type ProviderDispatcher interface {
    Dispatch(ctx context.Context, state *RequestState) (*DispatchResult, error)
}

type ProviderDispatcherImpl struct {
    registry        ProviderRegistry
    retry           RetryController
    fallback        FallbackController
    circuit         CircuitBreaker
    channelLimiter  ChannelLimitEnforcer
    observer        ProviderObserver
    clock           Clock
}
```

### 6.10 StreamFinalizer

```go
type StreamFinalizer interface {
    Wrap(ctx context.Context, state *RequestState, result *DispatchResult) (*GatewayResponse, error)
}
```

职责：

```text
包装 ProviderStream
记录 upstream/downstream bytes
记录 first token time
close 时调用 settlement
close 时释放 limit
close 时释放/结算 balance hold
```

### 6.11 SettlementService

```go
type SettlementService interface {
    Settle(ctx context.Context, state *RequestState) error
    RecordFailedSettlement(ctx context.Context, state *RequestState, err error) error
    ReplayFailedSettlement(ctx context.Context, id string) error
}
```

### 6.12 TaskBridge

```go
type TaskBridge interface {
    CreateMediaTask(ctx context.Context, state *RequestState) (*TaskObject, error)
    CancelTask(ctx context.Context, principal Principal, taskID string) error
    GetTask(ctx context.Context, principal Principal, taskID string) (*TaskObject, error)
}
```

---

## 7. 插件核心代码骨架

文件：`internal/dataplane/plugin/plugin.go`

```go
type Plugin interface {
    Name() string
    Version() string
    Phases() []Phase
    Init(ctx context.Context, cfg PluginInitConfig) error
    Execute(ctx context.Context, input Input) (Result, error)
}

type Input struct {
    Phase Phase
    State *engine.RequestState
    Config map[string]any
}

type Result struct {
    Continue bool
    Deny     *Deny
    Mutations []Mutation
    AuditEvents []audit.Event
    Tags map[string]string
}
```

文件：`internal/dataplane/plugin/manager.go`

```go
type Manager struct {
    registry Registry
    resolver BindingResolver
    logger   Logger
    metrics  Metrics
}

func (m *Manager) Run(ctx context.Context, phase Phase, state *engine.RequestState) error {
    bindings := m.resolver.Resolve(phase, state)
    sortBindings(bindings)
    for _, b := range bindings {
        p, ok := m.registry.Get(b.PluginName)
        if !ok { return apperr.ErrConfigUnavailable }
        result, err := p.Execute(ctx, Input{Phase: phase, State: state, Config: b.Config})
        if err != nil { return err }
        ApplyResult(state, result)
        if !result.Continue {
            return result.Deny.Error()
        }
    }
    return nil
}
```

---

## 8. 内置插件落地文件

```text
builtin/request_size.go
builtin/api_key_auth.go
builtin/ip_allowlist.go
builtin/model_acl.go
builtin/prompt_token_limit.go
builtin/pii_redaction.go
builtin/prompt_guard.go
builtin/response_guard.go
builtin/cost_guard.go
builtin/audit_log.go
builtin/llm_metrics.go
builtin/callback.go
```

每个插件都要有：

```text
Config struct
ValidateConfig()
Execute()
Unit tests
```

示例：`PromptTokenLimitPlugin`

```go
type PromptTokenLimitConfig struct {
    MaxInputTokens int
    CountMode string // approximate / tokenizer
    FailOpen bool
}

type PromptTokenLimitPlugin struct {
    tokenizer TokenCounter
}

func (p *PromptTokenLimitPlugin) Execute(ctx context.Context, in plugin.Input) (plugin.Result, error) {
    cfg := parseConfig[PromptTokenLimitConfig](in.Config)
    tokens, err := p.tokenizer.Count(in.State.Parsed)
    if err != nil && !cfg.FailOpen { return plugin.Result{}, err }
    if tokens > cfg.MaxInputTokens {
        return plugin.Deny("prompt_too_large", "prompt token limit exceeded"), nil
    }
    in.State.UsageEstimate.InputTokens = tokens
    return plugin.Continue(), nil
}
```

---

## 9. Provider Adapter 骨架

目录：`internal/provider/relay`

```go
type ProviderAdapter interface {
    Type() string
    Capabilities() Capabilities
    ValidateCredential(ctx context.Context, cred Credential) error
    Relay(ctx context.Context, req Request) (*Response, error)
    SubmitTask(ctx context.Context, req TaskRequest) (*TaskSubmitResult, error)
    PollTask(ctx context.Context, req TaskPollRequest) (*TaskPollResult, error)
}
```

如果某 provider 不支持 async task，可以让 `SubmitTask/PollTask` 返回 `ErrNotSupported`。

目录示例：

```text
internal/provider/openai/
  module.go
  adapter.go
  chat.go
  responses.go
  embeddings.go
  images.go
  stream.go
  errors.go
  usage.go

internal/provider/anthropic/
  module.go
  adapter.go
  messages.go
  stream.go
  errors.go
  usage.go

internal/provider/gemini/
  module.go
  adapter.go
  generate_content.go
  stream_generate_content.go
  images.go
  errors.go
  usage.go

internal/provider/seedance/
  module.go
  adapter.go
  video_generation.go
  task_poll.go
  errors.go
  usage.go
```

---

## 10. 路由策略代码骨架

文件：`internal/dataplane/routing/strategy.go`

```go
type StrategyRegistry struct {
    selectors map[RouteStrategy]CandidateSelector
}

func (r *StrategyRegistry) Register(s CandidateSelector)
func (r *StrategyRegistry) Select(strategy RouteStrategy) (CandidateSelector, bool)
```

`weighted_random.go`：

```go
type WeightedRandomSelector struct{}

func (s WeightedRandomSelector) Order(ctx context.Context, candidates []Candidate, input SelectionInput) ([]Candidate, error) {
    groups := groupByPriority(candidates)
    for _, group := range groups {
        key := input.SelectionKey
        if key == "" { key = input.RequestID }
        weightedShuffle(group, key)
    }
    return flatten(groups), nil
}
```

`health_weighted.go`：

```go
type HealthWeightedSelector struct {
    signals RouteSignalProvider
}

func (s HealthWeightedSelector) Score(c Candidate, input SelectionInput) float64 {
    health := input.Signals[c.ChannelID].HealthScore
    latency := input.Signals[c.ChannelID].LatencyScore
    cost := input.Signals[c.ChannelID].CostScore
    quota := input.Signals[c.ChannelID].QuotaScore
    weight := max(c.Weight, 1)
    return float64(weight) * health * latency * cost * quota
}
```

---

## 11. Worker 代码骨架

```text
internal/worker/
  runner.go
  scheduler.go
  job.go
  lease.go
  metrics.go
  jobs/
    expired_hold_reaper.go
    failed_settlement_replayer.go
    provider_task_poller.go
    callback_retry.go
    outbox_projector.go
    snapshot_consistency_checker.go
```

```go
type Job interface {
    Name() string
    Interval() time.Duration
    Timeout() time.Duration
    MaxConcurrency() int
    Run(ctx context.Context) error
}

type Runner struct {
    jobs []Job
    lease LeaseStore
    logger Logger
    metrics Metrics
}
```

每个 job 必须：

```text
有 timeout
可取消
有 lease
有 metrics
panic recover
失败 backoff
```

---

## 12. 数据库表优先级

MVP 第一批表：

```text
tenants
accounts
projects
api_keys
models
model_aliases
model_schemas
providers
provider_channels
provider_credentials
provider_model_mappings
route_policies
route_policy_channels
price_rules
limit_rules
plugin_bindings
runtime_snapshots
balances
balance_holds
usage_attempts
usage_records
ledger_entries
failed_settlements
tasks
task_events
files
audit_events
outbox_events
callback_outbox
```

---

## 13. 测试文件要求

每个核心包都要有测试：

```text
engine/engine_test.go
plugin/manager_test.go
routing/weighted_random_test.go
routing/health_weighted_test.go
admission/admission_test.go
limit/redis_limit_test.go
dispatch/retry_fallback_test.go
stream/finalizer_test.go
settlement/settlement_test.go
task/service_test.go
provider/openai/conformance_test.go
provider/gemini/conformance_test.go
```

必须有 e2e：

```text
test/e2e/openai_chat_test.go
test/e2e/claude_messages_test.go
test/e2e/gemini_generate_content_test.go
test/e2e/video_task_test.go
test/e2e/settlement_replay_test.go
test/e2e/stream_client_disconnect_test.go
```

---

## 14. 第一版编码顺序

```text
1. conf/log/httpserver/bootstrap
2. apperr/request/response
3. runtime snapshot minimal
4. API key auth
5. model catalog minimal
6. provider relay openai-compatible
7. route priority/weighted
8. balance hold + settlement mock
9. streaming close-time accounting
10. metrics/tracing/logs
11. async task skeleton
12. video provider adapter
13. failed settlement replay
14. control API and snapshot publishing
```
