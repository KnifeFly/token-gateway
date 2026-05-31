# 商用 AI Gateway Go 代码蓝图

本文档是实现蓝图。目标是让开发者可以按 package 和文件直接开工。

## 1. Go Module

```go
module github.com/your-org/token-gateway

go 1.23
```

建议依赖：

```text
net/http 或 chi
database/sql + sqlc 或 sqlx
redis/go-redis/v9
OpenTelemetry
Prometheus client_golang
zap 或 slog
validator
yaml/json schema validator
```

---

## 2. 最小可编译骨架

```text
cmd/gateway/main.go
internal/bootstrap/config.go
internal/bootstrap/gateway.go
internal/transport/httpserver/server.go
internal/transport/gatewayhttp/routes.go
internal/dataplane/engine/engine.go
internal/dataplane/engine/state.go
internal/dataplane/classifier/classifier.go
pkg/apperr/errors.go
```

先保证 `go test ./...` 通过，再扩展业务。

---

## 3. `pkg/apperr`

```go
type Code string

const (
    CodeUnauthorized        Code = "unauthorized"
    CodeForbidden           Code = "forbidden"
    CodeInvalidArgument     Code = "invalid_argument"
    CodeNotFound            Code = "not_found"
    CodeRateLimited         Code = "rate_limited"
    CodeInsufficientBalance Code = "insufficient_balance"
    CodeProviderError       Code = "provider_error"
    CodeAmbiguousProtocol   Code = "ambiguous_protocol"
    CodeIdempotencyConflict Code = "idempotency_conflict"
    CodeSnapshotStale       Code = "snapshot_stale"
    CodeConfigUnavailable   Code = "config_unavailable"
)

type Error struct {
    Code       Code
    Message    string
    HTTPStatus int
    Temporary  bool
    Cause      error
}
```

要求：

- 所有对外错误必须映射到 `apperr.Error`。
- 不直接把内部 `err.Error()` 暴露给客户端。
- `Cause` 只用于日志，写日志前必须 redaction。

---

## 4. 配置结构

文件：`internal/bootstrap/config.go`

```go
type Config struct {
    Environment string        `yaml:"environment"`
    Service     ServiceConfig `yaml:"service"`
    HTTP        HTTPConfig    `yaml:"http"`
    Database    DBConfig      `yaml:"database"`
    Redis       RedisConfig   `yaml:"redis"`
    Security    SecurityConfig `yaml:"security"`
    Gateway     GatewayConfig `yaml:"gateway"`
    Worker      WorkerConfig  `yaml:"worker"`
    Telemetry   TelemetryConfig `yaml:"telemetry"`
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
    Body        BodyConfig        `yaml:"body"`
    Stream      StreamConfig      `yaml:"stream"`
    Snapshot    SnapshotConfig    `yaml:"snapshot"`
    Protocol    ProtocolConfig    `yaml:"protocol"`
    Idempotency IdempotencyConfig `yaml:"idempotency"`
    Degradation DegradationConfig `yaml:"degradation"`
}
```

---

## 5. Data Plane 类型

文件：`internal/dataplane/engine/types.go`

```go
type IncomingRequest struct {
    Method     string
    Path       string
    RawQuery   string
    Header     http.Header
    Body       io.ReadCloser
    RemoteAddr string
    ContentLength int64
}

type GatewayResponse struct {
    StatusCode int
    Header     http.Header
    Body       []byte
    Stream     ProviderStream
    Usage      UsageActual
}
```

---

## 6. RequestState

文件：`internal/dataplane/engine/state.go`

```go
type RequestState struct {
    RequestID       string
    TraceID         string
    ClientRequestID string
    StartedAt       time.Time

    Incoming        IncomingRequest
    ProtocolMode    classifier.ProtocolMode
    CanonicalAPI    CanonicalAPI
    Endpoint        classifier.EndpointSpec

    Principal       *auth.Principal
    TenantID        string
    AccountID       string
    ProjectID       string
    APIKeyID        string
    UserID          string
    ClientIP        string

    RequestedModel  string
    ResolvedModel   modelcatalog.PublicModel
    Stream          bool
    Async           bool

    SnapshotRef     snapshot.SnapshotRef
    PinnedPriceRef  pricing.PriceRef
    PolicyRef       routing.PolicyRef

    Parsed          ParsedRequest
    EstimatedUsage  tokenusage.Estimate
    ActualUsage     tokenusage.Actual
    RoutePlan       *router.RoutePlan
    DegradationPlan *router.DegradationPlan

    IdempotencyKey  string
    IdempotencyHit  bool
    IdempotencyRef  *admission.IdempotencyRecord

    BalanceHold     *billing.BalanceHold
    LimitReleases   []limiter.LimitRelease
    ProviderResult  *dispatch.ProviderResult
    Task            *task.Task
    Settlement      *settlement.Result

    Metadata        map[string]string
    Internal        map[string]any
}
```

方法：

```go
func (s *RequestState) SetProtocol(mode ProtocolMode) error
func (s *RequestState) PinSnapshot(ref SnapshotRef) error
func (s *RequestState) PinPrice(ref PriceRef) error
func (s *RequestState) AddLimitRelease(release LimitRelease)
func (s *RequestState) Cleanup()
```

---

## 7. GatewayEngine

文件：`internal/dataplane/engine/engine.go`

```go
type GatewayEngine struct {
    cfg        EngineConfig
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
    audit      Auditor
    redactor   Redactor
    errmap     ErrorMapper
    observe    ObserveRecorder
}
```

构造：

```go
type Option func(*GatewayEngine)

func New(opts ...Option) (*GatewayEngine, error) {
    e := &GatewayEngine{}
    for _, opt := range opts { opt(e) }
    if err := e.validate(); err != nil { return nil, err }
    return e, nil
}
```

`validate` 必须检查：

```text
snapshot != nil
classifier != nil
parser != nil
auth != nil
router != nil
admission != nil
limiter != nil
dispatcher != nil
settlement != nil
```

MVP 可以允许 `plugins` 是 no-op。

---

## 8. APIClassifier

文件：`internal/dataplane/classifier/classifier.go`

```go
type ProtocolMode string

const (
    ProtocolAuto         ProtocolMode = "auto"
    ProtocolUnified      ProtocolMode = "unified"
    ProtocolNativeOpenAI ProtocolMode = "native_openai"
    ProtocolNativeClaude ProtocolMode = "native_claude"
    ProtocolNativeGemini ProtocolMode = "native_gemini"
)

type Classifier interface {
    Classify(ctx context.Context, state *engine.RequestState) error
}
```

实现结构：

```go
type DefaultClassifier struct {
    endpoints EndpointTable
    models    ModelHintResolver
}
```

核心伪代码：

```go
func (c *DefaultClassifier) Classify(ctx context.Context, s *RequestState) error {
    endpoint, ok := c.endpoints.Match(s.Incoming.Method, s.Incoming.Path)
    if !ok { return apperr.NotFound("endpoint not found") }

    explicit := protocolFromHeader(s.Incoming.Header)
    if explicit != ProtocolAuto {
        if !endpoint.Allows(explicit) { return apperr.InvalidArgument("protocol not allowed") }
        s.ProtocolMode = explicit
        s.CanonicalAPI = endpoint.CanonicalAPIFor(explicit)
        return nil
    }

    if endpoint.IsUnique() {
        s.ProtocolMode = endpoint.DefaultMode
        s.CanonicalAPI = endpoint.CanonicalAPI
        return nil
    }

    model := peekModelFromBody(s.Incoming.Body)
    hint, err := c.models.Resolve(ctx, s.SnapshotRef, model)
    if err == nil && endpoint.Allows(hint.ProtocolMode) {
        s.ProtocolMode = hint.ProtocolMode
        s.CanonicalAPI = endpoint.CanonicalAPIFor(hint.ProtocolMode)
        return nil
    }

    inferred, ok := inferByBodySchema(endpoint, s.Incoming.Body)
    if ok {
        s.ProtocolMode = inferred
        s.CanonicalAPI = endpoint.CanonicalAPIFor(inferred)
        return nil
    }

    return apperr.AmbiguousProtocol("set X-Gateway-Protocol")
}
```

---

## 9. Parser

文件：`internal/dataplane/parser/parser.go`

```go
type RequestParser interface {
    Parse(ctx context.Context, state *RequestState) error
}
```

子 parser：

```text
openai_parser.go
claude_parser.go
gemini_parser.go
unified_media_parser.go
```

输出统一结构：

```go
type ParsedRequest struct {
    Model          string
    PromptText     string
    Messages       []Message
    InputFiles     []FileRef
    Stream         bool
    Async          bool
    CallbackURL    string
    Metadata       map[string]string
    ModelParams    map[string]any
    RawBody        ProviderRelayBody
}
```

---

## 10. Authenticator

文件：`internal/dataplane/auth/authenticator.go`

```go
type Authenticator interface {
    Authenticate(ctx context.Context, state *RequestState) error
}

type DefaultAuthenticator struct {
    extractor CredentialExtractor
    hasher    APIKeyHasher
    snapshot  APIKeySnapshotLookup
    revoked   RevocationChecker
}
```

CredentialExtractor 不是插件：

```go
type CredentialExtractor interface {
    Extract(r IncomingRequest) (Credential, error)
}
```

支持：

```text
Authorization: Bearer
x-api-key
api-key
x-goog-api-key
query key 仅在配置允许时
```

---

## 11. PluginManager

文件：`internal/dataplane/plugin/manager.go`

```go
type Manager struct {
    chains atomic.Value // map[Phase][]CompiledPlugin
    timeout time.Duration
}

type CompiledPlugin struct {
    Name   string
    Phase  Phase
    Config json.RawMessage
    Impl   Plugin
    FailurePolicy FailurePolicy
}
```

Run：

```go
func (m *Manager) Run(ctx context.Context, phase Phase, state *RequestState) error {
    chains := m.chains.Load().(map[Phase][]CompiledPlugin)
    chain := chains[phase]
    if len(chain) == 0 { return nil }
    for _, p := range chain {
        result, err := p.Impl.Execute(ctx, PluginInput{State: state, Config: p.Config})
        if err != nil { return handlePluginError(p, err) }
        if err := applyPluginResult(state, result); err != nil { return err }
    }
    return nil
}
```

---

## 12. RoutePlanner

文件：`internal/dataplane/router/planner.go`

```go
type RoutePlanner interface {
    Plan(ctx context.Context, state *RequestState) error
}

type DefaultRoutePlanner struct {
    models     ModelResolver
    policies   PolicyResolver
    candidates CandidateResolver
    prices     PriceQuoter
    signals    RouteSignalProvider
    strategies StrategyRegistry
    degrade    DegradationPlanner
}
```

Plan 步骤：

```text
1. resolve public model / alias
2. validate capability
3. resolve route policy
4. resolve provider candidates
5. quote price and pin price ref
6. collect route signals
7. order candidates by strategy
8. build fallback/degraded candidates
9. save RoutePlan to RequestState
```

---

## 13. LimitEnforcer

文件：`internal/dataplane/limiter/enforcer.go`

```go
type Enforcer struct {
    counters CounterStore
    leases   ConcurrencyStore
    deny     *LocalDenyCache
    rules    LimitRuleResolver
}
```

```go
type CounterStore interface {
    AllowN(ctx context.Context, key string, limit int64, window time.Duration, n int64) (bool, error)
}

type ConcurrencyStore interface {
    Acquire(ctx context.Context, key string, requestID string, limit int64, ttl time.Duration) (LimitRelease, error)
}
```

`LocalDenyCache`：

```go
type LocalDenyCache struct {
    ttl time.Duration
    items sync.Map // key -> expiresAt
}
```

只缓存 Redis 拒绝结果。

---

## 14. AdmissionController

文件：`internal/dataplane/admission/controller.go`

```go
type AdmissionController interface {
    Reserve(ctx context.Context, state *RequestState) error
    Release(ctx context.Context, state *RequestState, cause error) error
}

type Controller struct {
    idempotency IdempotencyStore
    billing     BillingPort
    quoter      PriceQuoter
}
```

流程：

```text
1. check Idempotency-Key for async/task/write ops
2. estimate usage
3. quote price
4. create balance hold
5. bind idempotency record to task/request
```

---

## 15. ProviderDispatcher

文件：`internal/dataplane/dispatch/dispatcher.go`

```go
type Dispatcher struct {
    registry ProviderRegistry
    retry    RetryPolicyEvaluator
    circuit  CircuitBreaker
    observer DispatchObserver
}
```

```go
type ProviderResult struct {
    Response     *GatewayResponse
    Stream       ProviderStream
    FinalAttempt ProviderAttempt
    Attempts     []ProviderAttempt
}
```

重试规则：

```text
未写出下游响应前允许 retry/fallback
stream header 未返回前可 retry
stream body 已开始后不透明 retry
client cancel 不计 provider failure
```

---

## 16. StreamFinalizer

文件：`internal/dataplane/stream/finalizer.go`

```go
type StreamFinalizer interface {
    Wrap(ctx context.Context, state *RequestState, result *dispatch.ProviderResult) (*GatewayResponse, error)
}
```

`AccountingStream` 字段：

```go
type AccountingStream struct {
    source ProviderStream
    state  *RequestState
    settlement SettlementService
    releases []LimitRelease
    once sync.Once

    upstreamBytes int64
    downstreamBytes int64
    upstreamErr error
    downstreamErr error
    usage tokenusage.Actual
}
```

Close 时：

```text
close provider source
collect usage
classify billable
settle or record failed settlement
release request/provider concurrency leases
```

---

## 17. SettlementService

文件：`internal/dataplane/settlement/service.go`

```go
type SettlementService interface {
    Settle(ctx context.Context, state *RequestState) error
    RecordFailed(ctx context.Context, state *RequestState, cause error) error
}
```

`SettlementPlanner` 输出：

```go
type SettlementPlan struct {
    RequestID      string
    HoldID         string
    Usage          tokenusage.Actual
    AmountMicros   int64
    Currency       string
    Billable       bool
    RecordAttempt  bool
    ChargeCustomer bool
    ReleaseHold    bool
    KeepHoldOnError bool
}
```

---

## 18. Worker Runner

文件：`internal/worker/runner.go`

```go
type Runner struct {
    jobs []Job
    lease LeaseStore
    logger *slog.Logger
    metrics WorkerMetrics
}

type Job interface {
    Name() string
    Interval() time.Duration
    Timeout() time.Duration
    MaxConcurrency() int
    Run(ctx context.Context) error
}
```

每个 job 必须：

```text
支持 context cancel
支持 lease
支持 retry/backoff
有 metrics
panic recover
```

---

## 19. 数据库表优先级

### P0 表

```text
tenants
projects
api_keys
models
model_aliases
model_schemas
provider_channels
provider_model_mappings
route_policies
route_policy_channels
price_rules
limit_rules
runtime_snapshots
balances
balance_holds
usage_attempts
usage_records
ledger_entries
failed_settlements
idempotency_records
tasks
audit_events
```

### 关键唯一约束

```sql
unique(api_key_hash)
unique(tenant_id, name) on models
unique(tenant_id, api_key_id, endpoint, idempotency_key)
unique(request_id, api_key_id, channel_id, attempt_index) on usage_attempts
unique(request_id, settlement_kind) on ledger_entries
```

---

## 20. 测试文件要求

每个核心 package 至少：

```text
*_test.go 单元测试
integration_test.go 外部依赖测试，可 build tag 控制
contract_test.go provider adapter contract
```

重点测试：

```text
ambiguous protocol classifier
snapshot price pinning
key revocation blacklist
Redis limiter
idempotency conflict
provider retry/fallback
stream client disconnect
failed settlement replay
redaction
```

---

## 21. 编码顺序

```text
1. pkg/apperr
2. bootstrap/config
3. domain 基础类型
4. transport/httpserver
5. classifier + parser
6. auth + snapshot mock
7. GatewayEngine non-stream 主链路
8. provider/openai mock adapter
9. routing priority/weighted
10. admission + billing hold
11. settlement
12. Redis limiter
13. streaming
14. async task
15. control plane snapshot
16. plugins
```
