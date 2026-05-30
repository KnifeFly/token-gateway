# 商用 AI Gateway 系统设计文档

## 1. 产品定位

本系统不是简单的 OpenAI-compatible proxy，而是面向两类商业场景的 AI 能力运营平台：

1. **上游 Agent / 网剧 Agent 使用**：需要统一的多模态生成、任务编排、非存储输入资产转发、callback、幂等提交、场景/镜头元数据、失败重试。
2. **API 转售 / Token 售卖**：需要 API key、余额、限额、模型权限、供应商路由、成本控制、账务结算、对账和基础审计线索。

系统核心目标：

```text
统一协议 + 多模型接入 + 动态路由 + 计费结算 + 失败补偿 + 安全治理 + 可观测性
```

不应该把系统设计成“请求转发器”。转发只是数据面中的一个阶段。

---

## 2. 对外协议策略

### 2.1 双出口协议

系统同时暴露两类协议。

#### 2.1.1 Native Compatible API

目标：让用户尽量不改 SDK，只改 `base_url` 和 API key。

```text
OpenAI Compatible:
  POST /v1/chat/completions
  POST /v1/responses
  POST /v1/embeddings
  POST /v1/images/generations
  POST /v1/audio/speech
  POST /v1/audio/transcriptions

Claude Compatible:
  POST /v1/messages

Gemini Compatible:
  POST /v1beta/models/{model}:generateContent
  POST /v1beta/models/{model}:streamGenerateContent
```

#### 2.1.2 Unified Media / Agent API

目标：让 Agent 使用统一模型入口，不关心供应商差异。

```text
POST /v1/images/generations
POST /v1/images/edits
POST /v1/videos/generations
POST /v1/audio/speech
POST /v1/audio/transcriptions
POST /v1/music/generations
GET  /v1/tasks/{task_id}
POST /v1/tasks/{task_id}/cancel
POST /v1/files/upload/base64
POST /v1/files/upload/stream
POST /v1/files/upload/url
GET  /v1/credits
```

统一媒体协议采用“**统一 URI + model 参数 + 动态模型 schema**”机制。

`/v1/files/*` 在当前路线中只表达 transient input asset：系统可接收 URL、base64 或 multipart 输入，用于请求归一化、幂等校验、大小限制和转发给上游 provider。Gateway 不做对象存储，不承诺文件持久化、下载地址、生命周期管理、病毒扫描或存储 SLA。生成结果优先透传 provider result URL 或客户自有存储 URL，并通过 task `results`、`assets`、`usage` 和非敏感 `provider_metadata` 进入查询、callback 和结算。

示例：

```json
{
  "model": "seedance-2.0-reference-to-video",
  "prompt": "首帧使用图片1，全程参考视频1的运镜，音频1作为背景音乐",
  "image_urls": ["https://example.com/character.png"],
  "video_urls": ["https://example.com/camera.mp4"],
  "audio_urls": ["https://example.com/bgm.mp3"],
  "duration": 10,
  "quality": "1080p",
  "aspect_ratio": "16:9",
  "callback_url": "https://example.com/webhooks/task",
  "metadata": {
    "agent_id": "drama-agent-001",
    "workflow_id": "wf_123",
    "scene_id": "scene_08",
    "shot_id": "shot_08_03"
  },
  "model_params": {
    "motion_strength": 0.7,
    "camera_movement": "push_in",
    "character_consistency": true
  }
}
```

#### 2.1.3 Portal API

Portal API 是客户自助接口，不是 admin/control API。第一版复用 API key 鉴权，只覆盖当前 tenant/project 范围内的模型、schema、余额、用量、API key 自助管理和任务查询。

```text
GET  /v1/portal/models
GET  /v1/portal/models/{model}/schema
GET  /v1/portal/credits
GET  /v1/portal/usage
GET  /v1/portal/api-keys
POST /v1/portal/api-keys
POST /v1/portal/api-keys/{key_id}/disable
GET  /v1/portal/tasks
GET  /v1/portal/tasks/{task_id}
```

Portal 不允许配置 provider channel、route、price、limit、plugin、snapshot、emergency action，也不引入 RBAC。首个 API key 仍由 admin/control API 创建；portal 创建的派生 key 只能继承当前 tenant/project 和模型权限子集，历史 plaintext key 不可查询。Task 查询只返回当前 tenant/project 范围，并过滤敏感 metadata。

---

## 3. URI 冲突与协议消歧

### 3.1 冲突来源

Native Compatible API 和 Unified Media API 会复用部分行业通用 URI：

```text
POST /v1/images/generations
POST /v1/audio/speech
POST /v1/audio/transcriptions
```

这不是错误。它可以让客户继续使用熟悉的 URI。但系统必须定义严格的消歧规则。

### 3.2 协议模式

系统定义 `ProtocolMode`：

```go
type ProtocolMode string

const (
    ProtocolAuto         ProtocolMode = "auto"
    ProtocolUnified      ProtocolMode = "unified"
    ProtocolNativeOpenAI ProtocolMode = "native_openai"
    ProtocolNativeClaude ProtocolMode = "native_claude"
    ProtocolNativeGemini ProtocolMode = "native_gemini"
)
```

客户端可通过 header 显式声明：

```http
X-Gateway-Protocol: unified
X-Gateway-Protocol: native_openai
X-Gateway-Protocol: native_claude
X-Gateway-Protocol: native_gemini
```

如果不传，默认 `auto`。

### 3.3 APIClassifier 推断顺序

`APIClassifier` 必须按照下面顺序推断协议：

```text
1. Header: X-Gateway-Protocol
2. Path 唯一性
3. Model registry 中的 protocol_family
4. Request body schema
5. Content-Type / Accept
6. 无法判断则返回 ambiguous_protocol
```

### 3.4 冲突示例

#### 3.4.1 OpenAI-compatible image

```json
{
  "model": "gpt-image-1",
  "prompt": "a cat",
  "size": "1024x1024"
}
```

如果 model registry 中 `gpt-image-1.protocol_family = native_openai`，则走 OpenAI-compatible adapter。

#### 3.4.2 Unified image task

```json
{
  "model": "nano-banana-2-beta",
  "prompt": "电影感人像",
  "callback_url": "https://example.com/callback",
  "metadata": {"scene_id": "s1"},
  "model_params": {"seed": 123}
}
```

如果 model registry 中 `nano-banana-2-beta.protocol_family = unified_media`，则走 Unified Media task。

#### 3.4.3 模糊请求

如果请求既没有 header，model 又不存在，body 也无法匹配，则返回：

```json
{
  "error": {
    "code": "ambiguous_protocol",
    "message": "request protocol cannot be inferred; set X-Gateway-Protocol",
    "type": "invalid_request_error"
  }
}
```

---

## 4. `model` 的系统含义

对外 `model` 是字符串；对内它是系统核心路由入口。

```text
PublicModel
  -> ModelAlias
  -> ModelCapability
  -> ModelSchema
  -> ProviderChannel
  -> ProviderModelMapping
  -> RoutePolicy
  -> PriceRule
  -> LimitRule
  -> PluginBinding
```

### 4.1 PublicModel

```go
type PublicModel struct {
    ID              string
    Name            string
    DisplayName     string
    ProtocolFamily  ProtocolFamily
    Capability      ModelCapability
    InputModalities []Modality
    OutputModalities []Modality
    SupportsStream  bool
    SupportsAsync   bool
    MaxInputTokens  int
    MaxOutputTokens int
    Status          ModelStatus
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

### 4.2 ModelSchema

每个模型可以绑定自己的参数 schema：

```go
type ModelSchema struct {
    ModelID      string
    SchemaID     string
    Version      string
    JSONSchema   []byte
    Examples     []ModelExample
    CreatedAt    time.Time
}
```

统一媒体接口只固定顶层字段，把供应商/模型私有字段放到 `model_params`，并通过 `/v1/models/{model}/schema` 获取 schema。

---

## 5. 请求生命周期

### 5.1 同步非流式请求

```text
HTTP receive
  -> request_id / trace_id
  -> APIClassifier
  -> RequestParser
  -> Authenticator
  -> PolicyEvaluator
  -> Plugin: pre_prompt
  -> RoutePlanner
  -> AdmissionController.reserve
  -> LimitEnforcer.acquire
  -> ProviderDispatcher.dispatch
  -> SettlementService.settle
  -> Plugin: audit
  -> ResponseWriter
```

### 5.2 流式请求

```text
HTTP receive
  -> auth / route / admission / limit
  -> provider stream open
  -> downstream starts receiving chunks
  -> no transparent retry after first downstream byte
  -> stream wrapper tracks upstream/downstream bytes
  -> close-time usage collection
  -> close-time settlement
  -> release limit lease
```

原则：

1. **未写出任何下游 token**：允许 retry/fallback。
2. **已经写出 token**：不允许透明切换 provider。
3. **客户端断开**：不惩罚 provider health。
4. **下游未收到任何有效内容**：默认不计费。
5. **下游收到部分内容**：根据模型和租户策略判断是否计费。

### 5.3 异步媒体任务

```text
POST /v1/videos/generations
  -> classify protocol
  -> parse unified media request
  -> idempotency check
  -> auth / policy / prompt guard
  -> route async provider
  -> reserve balance
  -> create task
  -> dispatch provider task
  -> return task object

worker:
  -> poll provider task
  -> collect result
  -> settle usage
  -> release hold
  -> callback
```

---

## 6. RequestState 关键字段

所有请求处理阶段共享同一个 `RequestState`。它是请求生命周期的事实载体。

```go
type RequestState struct {
    RequestID       string
    TraceID         string
    ClientRequestID string
    StartedAt       time.Time

    ProtocolMode    ProtocolMode
    CanonicalAPI    CanonicalAPI
    Endpoint        EndpointSpec

    TenantID        string
    AccountID       string
    ProjectID       string
    APIKeyID        string
    UserID          string
    ClientIP        string

    RequestedModel  string
    ResolvedModel   PublicModel
    Stream          bool
    Async           bool

    SnapshotRef     SnapshotRef
    PinnedPriceRef  PriceRef
    PolicyRef       PolicyRef
    RoutePlan       *RoutePlan
    DegradationPlan *DegradationPlan

    EstimatedUsage  UsageEstimate
    ActualUsage     UsageActual
    BalanceHold     *BalanceHold
    LimitLeases     []LimitLease

    IdempotencyKey  string
    IdempotencyHit  bool
    IdempotencyRef  *IdempotencyRecord

    ProviderAttempt []ProviderAttempt
    Response        *GatewayResponse
    Task            *Task

    Metadata        map[string]string
    Internal        map[string]any
}
```

---

## 7. Snapshot 一致性设计

### 7.1 Snapshot 是数据面真相源

数据面不直接读控制面管理表。它只读取本地内存中的 `RuntimeSnapshot`：

```go
type SnapshotRef struct {
    Version      string
    Checksum     string
    LoadedAt     time.Time
    SchemaVersion string
}
```

### 7.2 请求级 pinning

请求一旦开始，必须 pin：

```text
snapshot version
price rule version
route policy version
model schema version
```

结算时不重新读取最新价格，否则可能出现“请求开始时按低价预占，结算时按高价扣费”的问题。

### 7.3 API Key 吊销一致性

API key 吊销不能只依赖 snapshot 刷新。系统需要 Redis 黑名单：

```text
ai_gateway:revoked_api_keys:{api_key_id} -> expires_at
```

认证流程：

```text
先查本地 snapshot 中 key 状态
再查 revoked key cache / Redis blacklist
任一命中则拒绝
```

### 7.4 一致性 SLA

| 变更类型 | 生效机制 | 目标 SLA |
|---|---|---|
| API key revoke | Redis blacklist + snapshot | < 2s |
| price rule update | snapshot | < 10s，新请求生效，旧请求 pin old price |
| route policy update | snapshot | < 10s |
| limit rule update | snapshot + Redis key version | < 10s |
| provider credential update | snapshot secret ref | < 10s |
| emergency provider disable | Redis hot deny + snapshot | < 2s |

### 7.5 Snapshot stale policy

```text
snapshot_age <= soft_ttl: 正常处理
soft_ttl < snapshot_age <= hard_ttl: 继续处理但打 warning metric
snapshot_age > hard_ttl: 只允许低风险接口，写操作 fail closed
无 snapshot: 数据面不可用
```

---

## 8. 限流、准入与降级

### 8.1 准入类型

```text
余额准入：是否有钱
额度准入：tenant/project/key/model 是否超过预算
并发准入：是否有并发槽位
成本准入：本次预计成本是否超过策略
策略准入：模型、地区、用户、内容策略
```

### 8.2 限流维度

```text
tenant
account
project
api_key
user
model
provider
channel
ip
custom label
```

### 8.3 限流算法

| 限流类型 | 算法 | 存储 |
|---|---|---|
| QPS/RPM | token bucket | Redis Lua |
| TPM | estimated token pre-charge + Redis Lua | Redis Lua |
| concurrency | lease set with TTL | Redis |
| daily budget | counter + ledger projection | DB/Redis projection |
| cost per minute | rolling counter | Redis |

### 8.4 localDenyCache

`localDenyCache` 只能缓存“刚被 Redis 拒绝”的 key，TTL 建议 100ms~1000ms。

禁止用 localDenyCache 做正向 allow 缓存。否则多副本会超发。

### 8.5 TPM 预扣与结算校正

流程：

```text
route planning 得到 estimated_input_tokens / estimated_output_tokens
limit pre-check 消耗 estimated tokens
provider 返回后得到 actual tokens
settlement 记录 actual usage
worker 可按 actual usage 做预算 projection 修正
```

限流 token bucket 不做“返还 token”，因为返还会导致窗口语义复杂。预算和计费用 actual usage 修正。

### 8.6 DegradationPlan

当主计划不可用或成本超限时，可以选择降级：

```go
type DegradationPlan struct {
    Reason           DegradationReason
    OriginalModel     string
    SuggestedModel    string
    SuggestedQuality  string
    SuggestedDuration int
    Candidates        []ProviderCandidate
    UserVisible       bool
}
```

常见降级：

| 场景 | 降级方式 |
|---|---|
| provider 429 | fallback 到其他 channel |
| provider 5xx | fallback 到同模型其他 channel |
| 成本预算不足 | 降级到便宜模型或低清晰度 |
| 高质量视频排队过长 | 降级到低质量或异步排队 |
| prompt 风险较高 | 拒绝，不降级 |
| 余额不足 | 拒绝，不降级 |

---

## 9. 插件体系

### 9.1 插件设计原则

1. 插件不能破坏认证安全边界。
2. 插件必须声明阶段、输入/输出、是否可改变状态。
3. 默认无绑定阶段必须 O(1) 跳过。
4. 插件执行必须有超时和错误策略。
5. 内置插件优先；动态插件后置。

### 9.2 MVP 启用阶段

最终设计精简 MVP 阶段为 9 个：

```text
pre_request
post_auth
pre_prompt
pre_route
post_route
pre_provider
post_provider
pre_settlement
audit
```

### 9.3 预留扩展阶段

完整 phase enum 可以保留：

```text
pre_policy
post_prompt
stream_chunk
pre_response
post_response
post_settlement
callback
```

这些阶段默认不启用，只有绑定插件时才进入。

### 9.4 为什么移除 credential_extract phase

API key 提取和基础认证属于核心安全边界，不应作为普通插件开放替换。正确做法：

```text
CredentialExtractor 是 Authenticator 内部组件
AuthPlugin 只做增强策略，例如 IP allowlist、JWT claim 映射、tenant policy
```

### 9.5 内置插件

| 插件 | 阶段 | 职责 |
|---|---|---|
| RequestSizePlugin | pre_request | body/header/附件大小限制 |
| IPAllowlistPlugin | post_auth | 租户/key/IP allowlist |
| ModelACLPlugin | post_auth / pre_route | 模型权限校验 |
| PromptTokenLimitPlugin | pre_prompt | prompt token 上限 |
| PIIRedactionPlugin | pre_prompt / post_provider | PII 脱敏 |
| PromptGuardPlugin | pre_prompt | prompt 安全策略 |
| RouteOverridePlugin | pre_route | 指定模型/渠道/标签 |
| CostGuardPlugin | post_route | 成本控制和降级建议 |
| ResponseGuardPlugin | post_provider | 输出安全检查 |
| AuditLogPlugin | audit | 审计事件 |
| LLMMetricPlugin | audit | token/cost/latency 指标 |
| CallbackPlugin | callback | 异步任务回调 |

---

## 10. 路由系统

### 10.1 RoutePlan

```go
type RoutePlan struct {
    RequestID        string
    Strategy         RouteStrategy
    PublicModel      PublicModel
    BillingModel     PublicModel
    Candidates       []ProviderCandidate
    Fallbacks        []ProviderCandidate
    Degraded         []ProviderCandidate
    MaxRetries       int
    MaxFallbacks     int
    TimeoutPolicy    TimeoutPolicy
    FailurePolicy    FailurePolicy
    PriceQuote       PriceQuote
    Signals          RouteSignals
}
```

### 10.2 策略列表

| 策略 | 说明 | MVP |
|---|---|---|
| priority | 按优先级排序 | 是 |
| weighted_random | 按权重随机 | 是 |
| round_robin | 轮询 | 可选 |
| consistent_hash | 按 selection key 粘性 | P1 |
| least_latency | 低延迟优先 | P1 |
| least_cost | 低成本优先 | P1 |
| health_weighted | 健康分加权 | P1 |
| quota_aware | 供应商 quota 感知 | P2 |
| semantic_route | 语义路由 | 先不做 |

### 10.3 默认策略

MVP 默认：

```text
priority + weighted_random
```

生产默认：

```text
health_weighted
```

评分公式建议：

```text
score = base_weight
      * health_score
      * latency_score
      * cost_score
      * quota_score
      * policy_score
```

---

## 11. 异步任务幂等

### 11.1 Idempotency-Key

所有创建异步任务和扣费型写操作支持：

```http
Idempotency-Key: user-generated-key
```

幂等作用域：

```text
tenant_id + api_key_id + endpoint + idempotency_key
```

### 11.2 幂等记录

```go
type IdempotencyRecord struct {
    ID             string
    TenantID       string
    APIKeyID       string
    Endpoint       string
    IdempotencyKey string
    RequestHash    string
    ResourceType   string
    ResourceID     string
    Status         IdempotencyStatus
    ExpiresAt      time.Time
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

### 11.3 行为规则

| 情况 | 返回 |
|---|---|
| key 不存在 | 创建 task，并记录 key |
| key 存在且 request_hash 一致 | 返回原 task |
| key 存在但 request_hash 不一致 | 409 idempotency_conflict |
| 原 task 失败 | 返回原失败 task，不重复创建 |
| key 已过期 | 可创建新 task |

---

## 12. WebSocket / Realtime 预留

当前路线不实现长连接或完整 Realtime，但系统保留 disabled contract 和接口预留：

```text
POST /v1/realtime/sessions
GET  /v1/realtime/sessions/{session_id}
WebSocket /v1/realtime?session_id=...
```

Realtime 不走普通 `GatewayEngine.Handle`，而应走：

```go
type RealtimeEngine interface {
    CreateSession(ctx context.Context, req RealtimeSessionRequest) (*RealtimeSession, error)
    HandleConnection(ctx context.Context, conn RealtimeConnection) error
}
```

普通请求生命周期可以复用：auth、policy、route、plugin、billing，但 streaming、session memory、bidirectional audio/video 需要独立引擎。

---

## 13. 可观测性

当前路线保留基础 structured logs、metrics、trace、redaction 和审计插件接入点。生产级 Observability 扩展平台不进入当前路线，不再新增 dashboard/SLO/告警平台能力作为后续必做项。

### 13.1 Metrics

必须暴露：

```text
ai_gateway_http_requests_total
ai_gateway_http_request_duration_seconds_bucket
ai_gateway_provider_attempts_total
ai_gateway_provider_attempt_duration_seconds_bucket
ai_gateway_provider_first_token_latency_seconds_bucket
ai_gateway_retries_total
ai_gateway_fallbacks_total
ai_gateway_degradations_total
ai_gateway_rate_limit_rejections_total
ai_gateway_circuit_state
ai_gateway_tokens_total
ai_gateway_cost_micros_total
ai_gateway_settlement_failures_total
ai_gateway_failed_settlement_backlog
ai_gateway_snapshot_staleness_seconds
ai_gateway_idempotency_hits_total
```

### 13.2 Logs

结构化日志字段：

```text
request_id
trace_id
tenant_id
project_id
api_key_id_hash
protocol_mode
canonical_api
model
provider_type
channel_id
status
error_code
latency_ms
input_tokens
output_tokens
cost_micros
snapshot_version
```

禁止记录：

```text
API key 明文
provider key 明文
完整 prompt
完整 response
未经脱敏的用户隐私字段
```

### 13.3 Tracing

span：

```text
gateway.receive
gateway.classify
gateway.auth
gateway.policy
gateway.plugin.pre_prompt
gateway.route
gateway.admission
gateway.limit
gateway.provider_attempt
gateway.stream
gateway.settlement
gateway.audit
```

---

## 14. 性能预算

初始生产目标：

| 项目 | 预算 |
|---|---|
| gateway 单实例 LLM 非流式 QPS | 500+，不含 provider latency |
| gateway 单实例 streaming 并发 | 5k SSE 连接起步 |
| APIClassifier + auth + route p99 | < 20ms |
| Redis 限流 p99 | < 10ms |
| snapshot 本地读取 | < 1ms |
| 非流式额外内存/request | < 256KB，不含 body |
| 大 body 内存阈值 | 16MB，超出 spill 到 temp file |
| failed settlement replay 延迟 | p95 < 60s |
| API key revoke 生效 | < 2s |
| route/price 变更生效 | < 10s |

这些不是最终 SLA，而是工程预算。超过预算必须给出原因。

---

## 15. 灾难恢复与降级

### 15.1 Redis 故障

| 能力 | 策略 |
|---|---|
| snapshot 缓存 | 使用本地最近一次有效 snapshot |
| QPS/TPM 限流 | fail closed 或 tenant policy 决定 |
| concurrency lease | fail closed |
| revoked key blacklist | fail closed for high-risk ops |
| async task queue | 停止新任务或进入 DB-only 模式 |

### 15.2 数据库故障

| 阶段 | 策略 |
|---|---|
| 请求前需要预占 | fail closed |
| provider 已成功但 settlement 失败 | 写本地 emergency log + failed settlement outbox，恢复后 replay |
| control plane | 只读或不可用 |
| gateway 数据面 | 只处理无需新预占的低风险请求，不建议继续收费流量 |

### 15.3 RPO/RTO

| 数据 | RPO | RTO |
|---|---|---|
| ledger / settlement | 0 或接近 0，通过事务和 outbox | 30min-2h |
| config snapshot | 1 个版本 | 10min |
| audit log | 5min | 1h |
| metrics | 可丢 | 30min |
| task result metadata | 1min | 1h |

---

## 16. MVP 边界

### 16.1 MVP 必做

```text
OpenAI-compatible chat non-stream
OpenAI-compatible chat stream
Claude messages compatible
Gemini generateContent compatible
Unified images/videos async task
API key auth
model ACL
route policy priority/weighted
Redis QPS/TPM/concurrency limit
balance hold
settlement
failed settlement replay
runtime snapshot
structured logs + metrics + tracing
```

### 16.2 当前不做

```text
控制面 RBAC / 审计平台
复杂财务 / 发票闭环
对象存储
完整 Realtime WebSocket / WebRTC
生产级 Observability 扩展平台
WASM 插件
动态脚本插件
```

这些能力不进入当前路线；已有基础 admin token、结构化日志、metrics、trace、redaction、审计插件和账务流水不因此移除。

### 16.3 当前先不做

```text
semantic routing
semantic cache
跨地域多活
```

这些能力需要重新产品决策后另立路线和任务板。

### 16.4 客户接入验收收口

P9 只补齐客户接入验收资产：Portal smoke CLI、OpenAPI import preflight、RC smoke 集成和客户验收 runbook。P9 不新增产品接口，也不改变 16.2 和 16.3 的范围边界。

---

## 17. 核心验收标准

1. 新增 provider 不改核心 gateway lifecycle。
2. 新增模型不改 URI。
3. 同一个请求在 provider 成功但结算失败时可自动修复。
4. 流式请求客户端中断不会误伤 provider health。
5. API key revoke 2 秒内生效。
6. price rule 更新不会影响已经开始的请求。
7. async task 重复提交不会重复扣费。
8. 所有敏感日志有 redaction。
9. 所有核心阶段有 metrics 和 trace。
10. OpenAPI 可导入 Apifox。
