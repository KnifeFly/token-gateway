# NewAPI Lean Console Parity 设计

本文档定义 token-gateway 在 Portal/Admin Console 基础上，对标 NewAPI 后仍需要补齐的精简产品能力。目标不是复制 NewAPI 管理端，而是保留 AI Gateway 商用运营必需的能力，并明确裁掉用户分组、模型分组、倍率体系、复杂系统设置、订阅套餐、兑换码、支付管理和模型部署服务等不进入当前路线的功能。

P24 只承载 Lean Console Parity：渠道管理、模型管理、用户/客户账户管理、令牌管理、使用日志、任务日志、操练场和最小额度运营。它必须沿用 P19-P23 已建立的 Human Console Plane、BFF、session、RBAC、audit、OpenAPI split 和 frontend monorepo 边界。

## 1. 设计结论

- NewAPI 里可参考的核心能力是：渠道管理、模型管理、用户管理、令牌管理、使用日志、任务日志、数据看板、操练场和客户充值/额度视图。
- token-gateway 不采用 NewAPI 的用户分组、模型分组、渠道分组或倍率体系。需要表达范围时使用 tenant/project/API key/model ACL/tag/category，而不是 group。
- token-gateway 不采用 NewAPI 的大而全系统设置页。需要配置的能力必须落到有 owner 的 typed configuration、control-plane config 或 feature flag。
- token-gateway 不在 P24 做订阅套餐、兑换码、第三方支付配置、邀请返利、模型部署服务或复杂首页/菜单配置。
- 渠道管理必须服务于 provider/channel lifecycle、credential rotation、model binding、health test、sync preview、route policy input 和 snapshot publish，而不是单纯维护 NewAPI 风格的代理通道。
- 模型管理必须服务于 model catalog、category、modality、schema、capability、pricing summary、channel coverage 和 Portal 模型广场展示，而不是维护模型分组和倍率。
- 用户管理必须服务于 tenant/project/customer account/API key/credits/status/audit，不做 NewAPI 的 group-based user plan。
- 页面设计必须与 NewAPI 不同：采用 operation workbench、detail drawer、safe diff、reason + idempotency + audit 的交互，不复制 NewAPI 的菜单、表格和抽屉布局。

## 2. NewAPI 观察摘要

本次参考的测试环境菜单包含：

| 区域 | NewAPI 模块 | 可参考点 | P24 决策 |
|---|---|---|---|
| 控制台 | 数据看板 | 余额、历史消耗、请求次数、模型分布、趋势排行 | 保留为 Admin/Portal summary，但按 tenant/project/channel/model 维度重做 |
| 控制台 | 令牌管理 | token 列表、剩余额度、可用模型、IP 限制、过期时间、禁用/编辑/删除 | 保留，映射到 token-gateway API key 管理 |
| 控制台 | 使用日志 | 时间、用户、令牌、模型、渠道、用时、输入/输出、花费、request_id | 保留，映射到 usage_records/provider_attempts/ledger read model |
| 控制台 | 任务日志 | task id、渠道、用户、平台、类型、状态、进度、耗时 | 保留，映射到 async task lifecycle |
| 个人中心 | 钱包管理 | 余额、充值、兑换码、邀请奖励 | P24 只做余额/ledger/人工调整视图，不做支付、兑换码和邀请 |
| 管理员 | 渠道管理 | 类型、名称、密钥、组织、API 地址、模型、模型重定向、自动禁用、测试、同步 | 保留核心，裁掉复杂透传和 group/rate 语义 |
| 管理员 | 模型管理 | 模型名称、匹配类型、描述、标签、供应商、端点、同步状态、绑定渠道 | 保留核心，模型目录与真实调用路由分离 |
| 管理员 | 用户管理 | 用户名、状态、额度、分组、角色、邀请、禁用/编辑/提升/降级 | 保留账户、状态、额度、角色；不做分组和邀请 |
| 管理员 | 订阅管理 | 套餐、价格、有效期、重置、升级分组 | 不进入 P24 |
| 管理员 | 兑换码管理 | 兑换码生成、复制、清理、兑换人 | 不进入 P24 |
| 管理员 | 支付管理 | 充值订单、支付配置、回调地址、第三方支付 | 不进入 P24 |
| 管理员 | 模型部署 | io.net 部署服务 | 不进入 P24 |
| 管理员 | 系统设置 | 运营、聊天、绘图、分组定价、限速、性能、系统、其他设置 | 不复制；只拆成 typed owner configs |
| 聊天 | 操练场 | 分组、模型、图片、多参数、流式、调试、导入导出 | 保留为 playground，但按 token-gateway protocol/schema 驱动 |

## 3. 裁剪原则

### 3.1 不做 NewAPI group

NewAPI 页面中大量使用 `分组` 作为用户、令牌、渠道和模型的共同维度。token-gateway 不引入这一层。

替代方案：

- 租户边界使用 `tenant_id`。
- 项目边界使用 `project_id`。
- API key 范围使用 `allowed_models`、IP allowlist、expires_at 和 explicit limit rules。
- 模型展示筛选使用 `category`、`modalities`、`capabilities`、`tags`、`status` 和 `sort_order`。
- 渠道展示筛选使用 `provider_type`、`capabilities`、`health_status`、`tags` 和 `enabled`。
- 路由策略继续使用 route policy、priority/weighted/health/cost/latency/quota aware signals，不通过 NewAPI group 做隐式分流。

### 3.2 不做倍率

NewAPI 的倍率适合单一额度单位的轻量计费。token-gateway 已经有 component price、currency + micros、provider cost、settlement、ledger 和 reconciliation，因此 P24 不引入 model ratio、group ratio 或 channel ratio。

替代方案：

- 客户售价继续使用 price rule + price components。
- 渠道成本继续使用 provider cost components。
- Portal/Admin 展示人可读价格摘要，但不允许在 UI 中输入倍率。
- 所有账务金额继续由报价器、hold、settlement 和 ledger 计算。

### 3.3 不复制复杂系统设置

NewAPI 系统设置页把菜单、聊天、绘图、模型、分组定价、限速、性能、日志、监控、签到等能力集中到一个页面。token-gateway 不采用这种模式。

替代方案：

- 安全、限流、路由、价格、snapshot、worker、callback、日志保留各自 owner。
- Admin UI 只暴露必要 typed form，并且必须有 OpenAPI schema、RBAC、reason、audit 和 validation。
- 复杂平台运营功能暂不做，避免绕过已有 control-plane 和 runtime snapshot 边界。

## 4. 信息架构

P24 后的 Admin Console 不使用 NewAPI 菜单分组，推荐信息架构：

```text
Admin Console
  Workbench
    Overview
    Incidents
    Release Readiness

  Catalog
    Models
    Model Availability
    Model Marketplace Preview

  Routing
    Channels
    Channel Health
    Route Policies
    Snapshot Preview

  Accounts
    Customer Accounts
    Projects
    API Keys
    Credit Operations

  Activity
    Usage Logs
    Task Logs
    Provider Attempts
    Audit Events

  Tools
    Playground
    Channel Test
    Model Sync Preview

  Settings
    Typed Console Settings
```

Portal Console 推荐信息架构：

```text
Portal Console
  Dashboard
  Models
  Playground
  API Keys
  Usage
  Tasks
  Credits
  Project Settings
```

## 5. 页面设计原则

P24 页面不能复制 NewAPI 的视觉结构。建议采用以下模式：

- 列表页左侧或顶部放 query builder：status、provider、category、capability、tenant/project、time range、request_id。
- 主区域使用 compact data grid，但行内只放低风险快捷动作，例如 view、test、disable request。
- 详情使用右侧 drawer 或 full page detail，不在主表上堆大量 inline editing。
- 高风险 mutation 使用独立 workflow：preview -> reason -> confirm -> submit -> audit result。
- 所有新增/编辑表单分为固定 tabs：Basics、Bindings、Policy、Health、Security、Audit。
- 密钥、credential、raw request、raw response、raw provider error 默认不可见。
- request_id、snapshot_version、audit_event_id、idempotency_key 必须在结果页可复制。
- 失败状态必须显示 safe error code 和 request_id，而不是 DB/Redis/provider 原始错误。

## 6. 渠道管理设计

### 6.1 目标

渠道管理用于运营 provider/channel 接入与健康，不承担 customer pricing、user grouping 或 model grouping。

P24 渠道管理需要支持：

- 创建、编辑、启用、禁用 channel。
- provider type、base URL、credential reference、organization/project 等连接信息。
- channel 支持的 public models 与 upstream models 绑定。
- 单模型、全渠道和批量 channel test。
- channel health status、latency、success rate、429/5xx/timeout、last error class。
- channel quota/cost config status，只展示安全摘要。
- upstream model sync preview，先 preview，再选择 apply。
- model rewrite/mapping，但必须限定为 public model -> upstream model，不支持任意请求改写绕过 schema。
- request header/parameter override 只允许 allowlist 字段，不支持任意 JSON 覆盖。
- priority/weight 仅作为 route policy 输入展示，不作为 NewAPI group/rate 替代。
- credential rotation，不显示已保存密钥明文。

### 6.2 字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `channel_id` | server ID | 服务端生成 |
| `name` | string | 运营可读名称 |
| `provider_type` | enum | openai、claude、gemini、replicate、dashscope、doubao 等 |
| `protocol` | enum | openai-compatible、claude-compatible、gemini-compatible、native media 等 |
| `base_url` | string | 经过 egressguard 和 URL policy 校验 |
| `credential_ref` | reference | 加密存储引用，不返回明文 |
| `credential_fingerprint` | string | 只展示 fingerprint、last_rotated_at |
| `organization` | string | 可选上游 organization/project，不进入日志 |
| `enabled` | bool | 手动启停 |
| `health_status` | enum | unknown、healthy、degraded、disabled、error |
| `supported_models` | list | public model 与 upstream model 映射 |
| `capabilities` | list | chat、responses、embedding、image、audio、tool、stream 等 |
| `route_policy_hint` | object | priority、weight 或 route strategy 输入 |
| `cost_config_status` | enum | configured、missing、partial |
| `last_tested_at` | time | 最近测试时间 |
| `last_error_class` | enum | safe error class |
| `tags` | list | 展示和筛选，不参与计费和权限 |
| `notes` | string | 管理员备注，不能含 secret |

### 6.3 页面

**Channels Workbench**

- 顶部 summary：healthy、degraded、disabled、missing cost、missing models、snapshot pending。
- Filter：provider、protocol、health、capability、enabled、cost status、model keyword、tag。
- Table columns：Channel、Provider、Protocol、Models、Health、Latency、Cost Status、Route Hint、Snapshot State、Last Test。
- Row actions：View、Test、Disable/Enable、Rotate Secret、Sync Preview。

**Channel Detail**

- Basics：name、provider type、protocol、base URL、organization、tags、notes。
- Credentials：fingerprint、last rotated、rotate workflow。
- Model Coverage：public model、upstream model、capability、schema compatibility、sync status。
- Health：test history、latency、error class、circuit breaker state、retry budget observation。
- Policy：route policy hint、manual disable, emergency state。
- Audit：last mutation, actor, reason, audit_event_id。

### 6.4 不进入 P24 的渠道能力

- 任意 request body passthrough。
- 任意 request header passthrough。
- 用户分组/渠道分组绑定。
- 通过倍率表达价格或成本。
- 代理地址由 UI 任意配置并进入生产。
- 系统提示词注入、思考内容转换、任意状态码复写。
- 自动把未知上游模型直接发布给客户；只能 preview -> approve -> snapshot。

## 7. 模型管理设计

### 7.1 目标

模型管理用于维护 public model catalog 和客户可见模型广场，同时连接真实路由、schema 和 pricing summary。它不能改变真实调用路径；真实调用仍由 runtime snapshot、model mapping、route policy 和 channel binding 决定。

P24 模型管理需要支持：

- 新增/编辑 public model。
- 精确/前缀/后缀/包含匹配可以作为 catalog match rule，但不能绕过 classifier。
- category、modalities、capabilities、tags、metadata。
- 模型展示名称、描述、图标、排序、状态。
- schema preview 和 endpoint capability 展示。
- channel coverage：哪些 channel 支持该模型，健康状态如何。
- pricing summary：按 component price 展示，不显示倍率。
- 官方/外部 catalog sync preview，可选择同步 metadata，不自动覆盖人工锁定字段。
- Portal 模型广场展示字段与 Admin 模型字段一致。

### 7.2 字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `model_id` | string | public model ID，请求中的 `model` 入口 |
| `display_name` | string | 展示名称 |
| `match_rule` | enum | exact、prefix、suffix、contains，仅用于 metadata 匹配 |
| `category` | enum | chat、embedding、rerank、image、video、audio_speech、audio_transcription、music、moderation、realtime_reserved |
| `modalities` | list | text、image、audio、video 等 |
| `capabilities` | list | stream、tools、json_schema、vision、batch、async_task |
| `status` | enum | draft、active、deprecated、disabled |
| `tags` | list | 展示筛选 |
| `description` | markdown | Portal 安全展示文本 |
| `schema_ref` | string | request/response schema reference |
| `endpoint_refs` | list | 支持的 API endpoint |
| `pricing_summary` | object | component price 摘要 |
| `provider_cost_status` | enum | configured、missing、partial |
| `channel_coverage` | object | 支持渠道数、健康渠道数、缺成本渠道数 |
| `sync_locked_fields` | list | 不被 sync 覆盖的字段 |
| `sort_order` | int | Portal 展示顺序 |

### 7.3 页面

**Models Catalog**

- 顶部 summary：active、deprecated、missing price、missing provider cost、no healthy channel、schema missing。
- Filter：category、capability、status、provider coverage、tag、keyword。
- Table columns：Model、Category、Capabilities、Price Summary、Channel Coverage、Schema、Portal Status、Updated。
- Row actions：View、Edit Metadata、Preview Schema、View Channels、Disable/Deprecate。

**Model Detail**

- Overview：display name、category、description、tags、status。
- Schema：request schema、response schema、supported endpoints。
- Pricing：customer price summary、provider cost status、quote sample。
- Channels：channel bindings、upstream model、health、last tested、cost status。
- Portal Preview：客户视角模型卡片，不复制 NewAPI 模型广场样式。
- Audit：mutation history。

### 7.4 不进入 P24 的模型能力

- 模型分组。
- 模型倍率。
- 通过模型页直接配置真实路由策略。
- 自动发布未知模型给客户。
- 用 UI 写任意 endpoint JSON 绕过 OpenAPI/schema。

## 8. 用户/客户账户管理设计

### 8.1 目标

NewAPI 的用户管理是站点用户维度。token-gateway 需要转译为客户账户运营：tenant、project、customer account、API key、credits、usage 和状态。P24 不引入用户分组，不做邀请返利，不做套餐升级分组。

如果 Portal 仍处于 API key login 模式，P24 第一版可以把 customer account 表达为 tenant/project contact + API key set；后续再扩展 Portal username/password 或 OIDC login。

### 8.2 能力

- 创建 customer account，并绑定 tenant/project。
- 管理账号状态：active、disabled、closed。
- 查看余额、总额度、历史消耗、active holds、failed settlements。
- 查看 API keys、allowed models、IP allowlist、expires_at。
- 创建/禁用/重置 customer API key。
- 手动额度调整：必须 reason、idempotency、audit，进入 ledger/manual adjustment。
- 重置 Portal session 或强制 logout。
- 管理联系人信息和管理员备注。
- 查看用户 usage、task、callback、settlement 摘要。

### 8.3 字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `customer_account_id` | server ID | 服务端生成 |
| `tenant_id` | ID | 租户边界 |
| `project_id` | ID | 项目边界 |
| `username` | string | 可选 Portal 登录名 |
| `display_name` | string | 展示名称 |
| `email` | string | 可选联系邮箱 |
| `status` | enum | active、disabled、closed |
| `role` | enum | owner、developer、viewer，仅 project scope |
| `balance` | money | 余额展示 |
| `total_granted` | money | 累计授信或充值，不等于套餐 |
| `active_api_key_count` | int | 活跃 key 数 |
| `allowed_models_summary` | object | 模型 ACL 摘要 |
| `notes` | string | 管理员备注 |
| `last_seen_at` | time | Portal/session 最近活动 |

### 8.4 页面

**Customer Accounts**

- Filter：status、tenant、project、role、balance range、model access、keyword。
- Table columns：Account、Tenant/Project、Status、Balance、API Keys、Models、Recent Usage、Last Seen。
- Row actions：View、Disable、Adjust Credit、Create API Key、Reset Session。

**Customer Detail**

- Overview：account、tenant/project、status、contact、notes。
- Credits：balance、ledger entries、manual adjustment workflow、active holds。
- API Keys：safe key metadata、allowed models、IP allowlist、expiry、disable.
- Usage：recent usage logs filtered to this account。
- Tasks：recent async tasks。
- Audit：operator actions。

### 8.5 不进入 P24 的用户能力

- 用户分组。
- 邀请奖励和返利。
- 订阅套餐升级分组。
- 客户自己选择上游渠道。
- 客户查看 provider cost、provider credential、raw provider error。

## 9. 令牌/API Key 管理设计

P24 需要把 NewAPI 令牌管理转译为 token-gateway API key management。

能力：

- Portal customer 自助创建 derived API key。
- Admin 创建 customer API key。
- disable/enable/rotate/revoke。
- allowed models 子集校验，不可扩大 project 权限。
- IP allowlist、expires_at、budget/limit rule binding。
- plaintext key 只在创建响应显示一次。
- key fingerprint、last_used_at、usage summary、status。
- 批量复制/批量删除不作为 P24 必需能力；批量 disable 可以进入后续。

表格字段：

```text
Name
Status
Tenant/Project
Fingerprint
Allowed Models
IP Allowlist
Expires At
Last Used At
Usage Summary
Created At
```

## 10. 使用日志与任务日志设计

### 10.1 使用日志

P24 使用日志需要聚合：

- usage_records
- provider_attempts
- ledger_entries
- billing settlement status
- request observe metadata

筛选项：

```text
time range
request_id
client_request_id
tenant/project
customer account
api key fingerprint/name
model
channel
provider
protocol
status/error class
stream/non-stream
```

表格字段：

```text
Time
Tenant/Project
API Key
Model
Protocol
Channel
Latency / First Token
Input Units
Output Units
Customer Cost
Provider Cost Summary
Settlement Status
Request ID
```

详情页只展示 safe metadata。raw prompt、raw response、provider secret、raw repair payload 默认不展示。

### 10.2 任务日志

P24 任务日志需要聚合：

- task lifecycle
- provider task attempt
- callback delivery
- task settlement
- idempotency key state

筛选项：

```text
time range
task_id
tenant/project
customer account
model
channel
provider
task status
callback status
settlement status
```

表格字段：

```text
Submit Time
Finish Time
Duration
Tenant/Project
Model
Provider/Channel
Task Type
Task ID
Status
Progress
Callback
Settlement
Request ID
```

## 11. 操练场与测试工具设计

P24 可以保留 NewAPI 操练场的产品价值，但实现必须按 token-gateway 协议和 schema 驱动。

能力：

- Portal Playground：客户使用自己的 session/project/API key 范围测试模型。
- Admin Playground：运营人员按 channel/model 做受控测试，不写入客户账务，或明确标记 internal test usage。
- 支持 chat、responses、embeddings、images/audio task 的 schema-driven form。
- 支持 stream 开关、tool calling、multimodal URL/base64 输入边界提示。
- 支持导入/导出测试 payload，但不导出 secret。
- 支持调试视图：request_id、route decision、channel、latency、usage、safe error。
- Channel Test 与 Playground 共用测试执行器，避免重复逻辑。

不进入 P24：

- 任意 raw JSON body 直接绕过 parser/schema。
- 将客户 raw prompt/response 存入 Admin 日志。
- 通过 Playground 修改模型、渠道或价格配置。

## 12. 最小额度运营设计

P24 不做支付、兑换码、订阅套餐和邀请返利，但需要提供最小商业运营视图：

- Account credits summary。
- Ledger entries 查询。
- Manual credit adjustment。
- Failed settlement/repair status 链接。
- Active holds 和 protected task holds。
- Export usage/ledger report。

所有额度调整必须：

```text
RBAC permission
reason
Idempotency-Key
audit event
ledger entry
safe response
```

## 13. Admin BFF API Scope

P24 在 P21/P22 的 Admin BFF 基础上扩展或细化以下 API。具体 path 可按已有 OpenAPI 命名调整，但浏览器只走 `/api/admin/v1/*`。

```text
GET    /api/admin/v1/channels
POST   /api/admin/v1/channels
GET    /api/admin/v1/channels/{channel_id}
PATCH  /api/admin/v1/channels/{channel_id}
POST   /api/admin/v1/channels/{channel_id}/disable
POST   /api/admin/v1/channels/{channel_id}/enable
POST   /api/admin/v1/channels/{channel_id}/rotate-credential
POST   /api/admin/v1/channels/{channel_id}/test
POST   /api/admin/v1/channels/{channel_id}/sync-preview
POST   /api/admin/v1/channels/{channel_id}/sync-apply
GET    /api/admin/v1/channels/{channel_id}/health-events

GET    /api/admin/v1/models
POST   /api/admin/v1/models
GET    /api/admin/v1/models/{model_id}
PATCH  /api/admin/v1/models/{model_id}
POST   /api/admin/v1/models/{model_id}/disable
POST   /api/admin/v1/models/{model_id}/deprecate
GET    /api/admin/v1/models/{model_id}/channels
GET    /api/admin/v1/models/{model_id}/schema-preview
POST   /api/admin/v1/models/sync-preview

GET    /api/admin/v1/customer-accounts
POST   /api/admin/v1/customer-accounts
GET    /api/admin/v1/customer-accounts/{account_id}
PATCH  /api/admin/v1/customer-accounts/{account_id}
POST   /api/admin/v1/customer-accounts/{account_id}/disable
POST   /api/admin/v1/customer-accounts/{account_id}/adjust-credit
POST   /api/admin/v1/customer-accounts/{account_id}/reset-session

GET    /api/admin/v1/api-keys
POST   /api/admin/v1/api-keys
POST   /api/admin/v1/api-keys/{key_id}/disable
POST   /api/admin/v1/api-keys/{key_id}/rotate

GET    /api/admin/v1/usage-logs
GET    /api/admin/v1/usage-logs/{request_id}
GET    /api/admin/v1/task-logs
GET    /api/admin/v1/task-logs/{task_id}

POST   /api/admin/v1/playground/run
POST   /api/admin/v1/playground/export
POST   /api/admin/v1/playground/import-preview
```

## 14. Portal BFF API Scope

P24 对 Portal BFF 的补强：

```text
GET    /api/portal/v1/models
GET    /api/portal/v1/models/{model_id}
GET    /api/portal/v1/models/{model_id}/schema
POST   /api/portal/v1/playground/run

GET    /api/portal/v1/api-keys
POST   /api/portal/v1/api-keys
POST   /api/portal/v1/api-keys/{key_id}/disable

GET    /api/portal/v1/usage
GET    /api/portal/v1/tasks
GET    /api/portal/v1/credits
GET    /api/portal/v1/ledger
```

Portal 仍不能访问 channel、provider cost、route policy、snapshot、admin audit 或 operator management。

## 15. Repository 与 Owner 边界

P24 不新增全局 service/repository。

```text
internal/app/admin/service
internal/app/admin/repository
internal/app/portal/service
internal/app/portal/repository
internal/controlplane/configadmin
internal/controlplane/snapshot
internal/billing
internal/task
internal/dataplane
```

Owner 规则：

- Channel/model/route/price/limit 写入继续委托 `internal/controlplane/configadmin`。
- Snapshot validate/publish/rollback 继续委托 snapshot owner。
- Ledger、manual adjustment、failed settlement 继续委托 billing owner。
- Task cancel/retry/callback 继续委托 task/worker owner。
- Admin repository 只做 safe read model、session、operator、audit 和 UI query，不直接写 owner table。
- Portal repository 只做 customer-scoped read model 和 API key self-service，不写 admin/config/snapshot 表。

## 16. RBAC 与审计

P24 继续使用 P21 角色模型，但补充权限：

| Permission | 说明 |
|---|---|
| `channel:read` | 查看 channel safe DTO |
| `channel:write` | 创建/编辑 channel |
| `channel:secret_rotate` | 轮换 credential |
| `channel:test` | 执行 channel test |
| `channel:sync` | 执行 sync preview/apply |
| `model:read` | 查看 model catalog |
| `model:write` | 编辑 model metadata |
| `customer:read` | 查看 customer account |
| `customer:write` | 创建/编辑/禁用 customer account |
| `customer:credit_adjust` | 人工额度调整 |
| `api_key:write` | 创建/禁用/轮换 API key |
| `usage:read` | 查看 usage logs |
| `task:read` | 查看 task logs |
| `playground:run` | 执行 Admin Playground |

所有 mutation 必须写 audit：

```text
actor
permission
resource
resource_id
request_id
idempotency_key
reason
redacted_before
redacted_after
status
safe_error_code
```

## 17. 安全与脱敏

- credential 永远不返回明文。
- API key plaintext 只在创建时显示一次。
- usage/task detail 不显示 raw prompt、raw response、raw provider error、repair payload。
- channel test 不记录完整 prompt；仅记录 safe request metadata 和 result。
- egressguard 校验 base URL、callback URL 和 file URL。
- UI 不允许 browser 直接调用 `/admin/*`。
- 所有 dangerous action 需要 reason、confirm、idempotency 和 audit。
- OpenAPI schema 使用 safe DTO，contract tests 检查 denylist 字段不出现。

## 18. OpenAPI 与前端

P24 必须更新：

- `api/openapi/admin-bff.yaml`
- `api/openapi/portal-bff.yaml`
- generated TypeScript client
- frontend query hooks
- contract tests

前端包规则不变：

```text
web/apps/admin -> web/packages/api-client
web/apps/admin -> web/packages/ui
web/apps/admin -> web/packages/auth
web/apps/admin -> web/packages/format

web/apps/portal -> web/packages/api-client
web/apps/portal -> web/packages/ui
web/apps/portal -> web/packages/auth
web/apps/portal -> web/packages/format
```

`web/packages/ui` 只放纯 UI，不放 ChannelTable、ModelEditor、CustomerAccountDetail 等业务组件。

## 19. 与 P22/P23 的关系

- P22 负责完整 Admin UI、E2E smoke、static asset、安全头和 production release gate。
- P23 负责目录结构、handler/service/repository/frontend package 拆分。
- P24 负责基于 NewAPI 观察后的 lean product parity scope 和实现细化。

执行上可以分两种情况：

1. 如果 P22/P23 未完成，P24 先作为设计和任务边界输入 P22/P23，避免 Admin UI 做错功能范围。
2. 如果 P22/P23 已完成，P24 作为独立产品增强阶段，按 channel -> model -> customer account -> logs -> playground 顺序实现。

## 20. 验收标准

- 渠道管理支持 safe CRUD、credential rotation、test、sync preview/apply、health read model，并且不泄漏 credential。
- 模型管理支持 catalog metadata、category、schema、pricing summary、channel coverage 和 Portal preview，并且不引入倍率。
- 用户/客户账户管理支持 tenant/project scoped account、API keys、status、credit adjustment 和 audit，并且不引入用户分组。
- API key 管理支持 create/disable/rotate/allowed models/IP/expires，并且 plaintext key 只显示一次。
- 使用日志和任务日志支持关键筛选、safe detail、request_id/task_id drilldown 和 export。
- Playground 支持 schema-driven test、safe debug 和 Admin/Portal scope 隔离。
- Admin/Portal frontend 不直接拼裸 fetch，不访问 `/admin/*`。
- OpenAPI、generated client、contract tests 和 focused Go tests 一致。
- P24 明确裁剪的模块不会以半实现形式进入 UI 菜单。

## 21. 风险与处理

| 风险 | 处理 |
|---|---|
| 对标 NewAPI 变成复制 NewAPI | 每个模块都记录 keep/cut decision，UI 信息架构重新设计 |
| 分组和倍率绕回实现 | OpenAPI、UI 文案和任务板明确禁止 group/ratio 输入 |
| Channel UI 绕过 owner service | Admin service 只 orchestration，config 写入委托 `configadmin` |
| Channel test 泄漏 prompt 或 secret | 使用 safe test payload、credential ref 和 redacted result |
| Model sync 自动发布未知模型 | 必须 preview -> approve -> snapshot，不自动 customer visible |
| 用户管理变成复杂 IAM | P24 只做 customer account/project role，不接企业 SSO/OIDC/group mapping |
| 日志详情泄漏 raw payload | safe DTO + denylist contract tests |
| 支付/订阅/兑换码被顺手加入 | P24 non-goals 明确禁止，后续需要单独商业支付阶段 |

## 22. 设计来源

- [Portal/Admin Console Monorepo 设计](./ai_gateway_console_monorepo_design.md)
- [系统设计](./ai_gateway_system_design.md)
- [架构设计](./ai_gateway_architecture_design.md)
- [代码蓝图](./ai_gateway_code_blueprint.md)
- [OpenAPI 合同](./ai_gateway_openapi.yaml)
- [路线图](../plan/00-roadmap.md)
- [任务清单](../tasks.md)
