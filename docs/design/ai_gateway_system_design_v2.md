# 商用 AI Gateway 系统设计文档 v0.2

版本：v0.2  
目标：面向 **网剧 Agent / 多模态内容生成 / API 转售 / Token 与积分计费** 的商用 AI Gateway。

本文是产品和系统层设计。代码落地细节见：

- `ai_gateway_architecture_design_v2.md`
- `ai_gateway_code_blueprint_v2.md`
- `ai_gateway_implementation_plan_v2.md`
- `ai_gateway_task_list_v2.md`

---

## 1. 产品定位

系统不是简单的 OpenAI Proxy，而是一个面向商业化的 AI 能力运营平台。

核心价值：

```text
统一协议入口
多供应商模型接入
多模态任务编排
模型路由与降级
Token / 积分 / 成本计费
租户隔离
安全治理
可观测性
失败补偿
商用运营后台
```

目标用户：

| 用户类型 | 使用方式 | 核心需求 |
|---|---|---|
| 网剧 Agent | 统一媒体 API + 异步任务 | 生图、生视频、配音、音乐、镜头素材、任务状态、callback |
| API 转售客户 | OpenAI / Claude / Gemini 兼容协议 | 改 base_url、稳定转发、余额查询、用量账单 |
| 内部运营 | Admin / Control API | 渠道配置、价格配置、路由配置、限流配置、审计 |
| 财务/商务 | Billing / Ledger / Invoice | 余额、充值、扣费、对账、成本利润 |
| 运维 | Metrics / Tracing / Logs | 故障定位、供应商健康、成本异常、任务积压 |

---

## 2. 对外协议策略

### 2.1 两类出口

系统对外暴露两类协议。

#### A. Native Compatible API

目标：让现有客户尽量只改 `base_url` 和 `api_key`。

```text
POST /v1/chat/completions                         OpenAI Chat Completions
POST /v1/responses                                OpenAI Responses
POST /v1/embeddings                               OpenAI Embeddings
POST /v1/images/generations                       OpenAI Images 兼容入口，可复用为统一生图入口
POST /v1/audio/speech                             OpenAI Speech 兼容入口
POST /v1/audio/transcriptions                     OpenAI Transcription 兼容入口
POST /v1/messages                                 Claude Messages
POST /v1beta/models/{model}:generateContent       Gemini GenerateContent
POST /v1beta/models/{model}:streamGenerateContent Gemini StreamGenerateContent
```

#### B. Unified Media / Agent API

目标：面向网剧 Agent 和多模态工作流，使用统一 URI，通过 `model` 选择具体模型。

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
GET  /v1/usage
```

统一 URI 的原则：

```text
URI 表达能力，不表达供应商。
model 表达具体模型、模型族或供应商映射。
model_params 承载模型私有参数。
metadata 承载 Agent / workflow / scene / shot 等业务信息。
```

---

## 3. 统一媒体接口设计原则

### 3.1 顶层字段稳定

视频生成请求建议统一为：

```json
{
  "model": "seedance-2.0-reference-to-video",
  "prompt": "参考人物设定图，生成一个夜晚街头追逐镜头",
  "negative_prompt": "低清晰度，畸形，人物变脸",
  "image_urls": ["https://cdn.example.com/character.png"],
  "video_urls": ["https://cdn.example.com/camera_ref.mp4"],
  "audio_urls": ["https://cdn.example.com/bgm.mp3"],
  "duration": 8,
  "quality": "1080p",
  "aspect_ratio": "16:9",
  "seed": 12345,
  "callback_url": "https://customer.example.com/webhooks/ai-task",
  "metadata": {
    "agent_id": "drama-agent-001",
    "workflow_id": "wf_20260525_001",
    "scene_id": "scene_08",
    "shot_id": "shot_08_03",
    "character_id": "char_luna"
  },
  "model_params": {
    "camera_movement": "push_in",
    "motion_strength": 0.65,
    "character_consistency": true,
    "style_reference_weight": 0.8
  }
}
```

字段分层：

| 字段 | 说明 | 是否稳定 |
|---|---|---|
| `model` | 对外模型名，路由入口 | 稳定 |
| `prompt` | 主提示词 | 稳定 |
| `negative_prompt` | 负向提示词 | 稳定 |
| `image_urls` | 图像素材 | 稳定 |
| `video_urls` | 视频素材 | 稳定 |
| `audio_urls` | 音频素材 | 稳定 |
| `duration` | 目标时长 | 稳定 |
| `quality` | 清晰度档位 | 稳定 |
| `aspect_ratio` | 画幅 | 稳定 |
| `callback_url` | 异步回调地址 | 稳定 |
| `metadata` | 客户业务元数据 | 稳定 |
| `model_params` | 模型私有参数 | 非稳定，由模型 schema 定义 |

### 3.2 `model` 背后的含义

`model` 不能只是字符串，它必须映射到运行时模型配置：

```text
PublicModel
ModelAlias
ModelCapability
ModelSchema
ProviderChannel
ProviderModelMapping
RoutePolicy
PriceRule
LimitRule
PluginBinding
```

例如：

```text
model = seedance-2.0-text-to-video
  capability = text_to_video
  task_mode = async
  output_type = video
  billing_unit = video_second
  schema = seedance_2_0_text_to_video_schema
  route_policy = video_default_policy
  price_rule = CNY 0.08 / second for 720p
  providers = volcengine_seedance_channel_a, volcengine_seedance_channel_b
```

---

## 4. 核心业务能力

### 4.1 模型与供应商接入

必须支持：

```text
OpenAI
Azure OpenAI
Anthropic Claude
Google Gemini
OpenAI-compatible providers
国内多模态供应商
自研模型供应商
```

供应商接入不应该影响对外协议。新增供应商只做：

```text
Provider module
Provider adapter
Provider descriptor
Provider credential schema
Provider error mapping
Provider usage parser
Provider task polling adapter
```

### 4.2 路由能力

MVP 必须支持：

```text
priority
weighted_random
round_robin
consistent_hash
```

商用版本必须支持：

```text
health_weighted
least_latency
least_cost
quota_aware
tenant_affinity
region_affinity
capability_match
semantic_route
```

路由必须返回候选计划，而不是只返回一个 channel：

```text
RoutePlan
  public_model
  billing_model
  candidates[]
  strategy
  max_retries
  max_fallbacks
  timeout_policy
  cost_policy
  failure_policy
```

### 4.3 限流与准入

维度：

```text
tenant
account
project
api_key
user
ip
model
route
provider
channel
```

类型：

```text
QPS
RPM
TPM
TPD
concurrency
cost_per_minute
cost_per_day
request_body_bytes
prompt_tokens
output_tokens
```

准入顺序：

```text
鉴权 -> 权限 -> 价格预估 -> 余额预占 -> 限流 -> provider dispatch
```

### 4.4 计费能力

计费对象：

```text
balance_account
balance_hold
usage_attempt
usage_record
ledger_entry
failed_settlement
reconciliation_report
invoice_item
```

计费单位：

| 能力 | 计费单位 |
|---|---|
| 文本模型 | input_tokens / output_tokens / cached_tokens / reasoning_tokens |
| Embeddings | input_tokens |
| 生图 | image_count / resolution / quality |
| 图像编辑 | image_count / input_image_count / resolution |
| 生视频 | duration_seconds / quality / resolution / reference_count |
| 语音合成 | characters / seconds |
| 语音转写 | audio_seconds |
| 音乐生成 | duration_seconds |
| 工具调用 | request_count / tool_call_count |

所有账务写入必须幂等。

### 4.5 异步任务能力

媒体生成默认异步。

任务状态：

```text
queued
running
succeeded
failed
canceled
expired
```

任务生命周期：

```text
CreateTask
  -> AdmitAndReserve
  -> DispatchProviderTask
  -> PollProviderTask / ReceiveWebhook
  -> NormalizeResult
  -> SettleUsage
  -> CallbackCustomer
  -> ExpireAsset
```

任务表必须保留：

```text
task_id
external_task_id
provider_type
channel_id
model
status
progress
input_payload
normalized_payload
result_assets
usage
error_code
error_message
callback_url
metadata
created_at
updated_at
expires_at
```

---

## 5. 多租户设计

租户层级：

```text
Tenant
  └── Account
      └── Project
          └── APIKey
              └── User / Agent / Workflow metadata
```

隔离策略：

| 资源 | 隔离方式 |
|---|---|
| API Key | project/account scoped |
| 模型权限 | API key / project / account scoped |
| 价格 | tenant/account/project scoped |
| 路由 | tenant/account/project/model scoped |
| 限流 | tenant/account/project/api_key scoped |
| 审计 | tenant scoped |
| 文件 | tenant + project scoped |
| 账务 | account scoped |

---

## 6. 请求生命周期

### 6.1 同步 LLM 请求

```text
1. HTTP receive
2. request_id / trace_id
3. API classification
4. API key extraction
5. API key auth
6. model and capability parse
7. plugin: pre_auth / post_auth
8. model authorization
9. prompt inspection
10. route planning
11. price estimate
12. balance hold
13. rate limit / concurrency limit
14. provider attempt
15. retry / fallback if needed
16. usage parse
17. settlement
18. response write
19. access log / metrics / trace / audit
```

### 6.2 流式请求

```text
1. 前置流程同同步请求
2. provider stream opened
3. response headers written
4. stream chunks copied
5. downstream write metrics
6. provider usage collected while streaming
7. stream close
8. close-time settlement
9. release concurrency and balance hold
```

关键规则：

```text
如果已经向客户端写出 token，不允许透明 retry 到另一个 provider。
如果客户端断开，但已经收到部分 token，需要按策略判断是否计费。
如果 provider stream 失败但客户端已收到可用内容，需要记录 provider failure 和 billable outcome。
```

### 6.3 异步媒体任务

```text
1. request receive
2. parse model and media inputs
3. validate model schema
4. upload / validate input files
5. price precheck
6. balance hold
7. create internal task
8. dispatch provider task
9. return task object
10. worker poll or webhook receive
11. normalize provider result
12. settle final cost
13. callback customer
14. expose GET /v1/tasks/{task_id}
```

---

## 7. 插件体系

插件阶段：

```text
pre_request
credential_extract
authenticated
pre_policy
pre_prompt
post_prompt
pre_route
post_route
pre_admission
post_admission
pre_provider
post_provider
stream_chunk
pre_response
post_response
pre_settlement
post_settlement
audit
```

内置插件：

| 插件 | 阶段 | 作用 |
|---|---|---|
| APIKeyAuthPlugin | credential_extract / authenticated | 提取和认证 API key |
| IPAllowlistPlugin | pre_policy | IP 白名单 |
| ModelACLPlugin | pre_policy | 模型权限 |
| RequestSizePlugin | pre_request | 请求大小限制 |
| PromptTokenLimitPlugin | pre_prompt | prompt token 限制 |
| PiiRedactionPlugin | pre_prompt / pre_response / audit | 脱敏 |
| PromptGuardPlugin | pre_prompt | prompt 安全 |
| ResponseGuardPlugin | pre_response | response 安全 |
| RateLimitPlugin | pre_admission | 限流 |
| CostGuardPlugin | pre_admission | 成本预算限制 |
| AuditLogPlugin | audit | 审计 |
| LLMMetricPlugin | post_provider / post_settlement | LLM 指标 |
| CallbackPlugin | post_settlement | 异步任务回调 |

插件默认是配置驱动，不做动态代码执行。

---

## 8. 安全设计

### 8.1 API Key

```text
只展示一次明文
数据库只存 hash
key_prefix 用于查找
支持过期时间
支持禁用
支持模型白名单
支持 IP allowlist
支持 project/account scope
```

### 8.2 Provider Credential

```text
KMS / envelope encryption
key version
rotation job
decrypt audit
不进入日志
不进入 metrics label
不直接进入 snapshot 明文
```

### 8.3 Prompt 和 Response 数据

默认不存完整 prompt / response。

如果业务必须存：

```text
显式开启
字段级脱敏
采样
加密存储
保留期
访问审计
```

---

## 9. 可观测性

### 9.1 Metrics

必须有：

```text
http_requests_total
http_request_duration_seconds_bucket
gateway_requests_total
gateway_request_duration_seconds_bucket
gateway_rate_limited_total
gateway_provider_attempts_total
gateway_provider_latency_seconds_bucket
gateway_provider_first_token_latency_seconds_bucket
gateway_retries_total
gateway_fallbacks_total
gateway_circuit_state
gateway_tokens_total
gateway_cost_micros_total
gateway_balance_hold_total
gateway_settlement_total
gateway_failed_settlement_backlog
gateway_task_total
gateway_task_duration_seconds_bucket
gateway_callback_total
gateway_snapshot_version
gateway_snapshot_staleness_seconds
```

### 9.2 Logs

日志分类：

```text
access log
provider attempt log
settlement log
task lifecycle log
admin audit log
security event log
```

日志要求：

```text
结构化
必须带 request_id / trace_id / tenant_id / project_id
不得带 API key / provider key / 原始 prompt / 原始 response
错误信息要经过 redactor
```

### 9.3 Tracing

核心 span：

```text
gateway.receive
gateway.auth
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

## 10. 部署形态

### 10.1 进程

```text
cmd/gateway       数据面
cmd/control-api   控制面 API
cmd/worker        异步任务、账务修复、provider polling
cmd/configd       配置快照发布和订阅，可并入 control-api 起步
cmd/admin-cli     管理命令行，可选
```

### 10.2 依赖

MVP：

```text
PostgreSQL/MySQL
Redis
Object Storage
OpenTelemetry Collector
Prometheus
```

规模化后：

```text
Kafka / Redpanda / NATS JetStream
ClickHouse / BigQuery for analytics
Vector DB for semantic routing/cache
KMS
```

---

## 11. MVP 边界

第一版必须做：

```text
API key auth
OpenAI chat/responses/embeddings
Claude messages
Gemini generateContent
Unified image/video generation
async task
provider adapter
routing priority/weighted
limit QPS/TPM/concurrency
balance hold
settlement
failed settlement replay
metrics/tracing/logs/audit
```

第一版不要做：

```text
动态 WASM 插件
复杂 semantic cache
完整 invoice 税务系统
多地域 active-active
高度复杂 Agent workflow DSL
```

---

## 12. 核心验收标准

系统上线前必须能回答：

```text
谁调用了？
调用了哪个模型？
为什么路由到这个 provider/channel？
用了多少 token 或生成了多少秒视频？
应该扣多少钱？
实际扣了多少钱？
provider 成本是多少？
失败后是否补偿？
客户是否收到响应？
敏感数据有没有进日志？
这个请求在哪个 trace 里？
```
