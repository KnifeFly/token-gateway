# 商用 AI Gateway 实施计划文档 v0.2

版本：v0.2  
目标：按阶段、里程碑、验收标准推进，避免一开始把系统做散。

---

## 1. 实施原则

1. **先数据面，后复杂控制面**：先保证请求可进、可路由、可计费、可观测。
2. **先账务闭环，后大规模售卖**：provider 成功后本地失败必须能补偿。
3. **先内置插件，后动态插件**：先用稳定接口承载扩展点。
4. **先统一协议，后模型私有能力**：顶层协议稳定，私有参数放 `model_params`。
5. **先同步 LLM，再异步媒体**：同步链路验证网关核心，再扩展 task。
6. **先单区域，后多区域**：先做可用，再做复杂部署。

---

## 2. 阶段总览

| 阶段 | 名称 | 目标 | 时间建议 |
|---|---|---|---|
| M0 | 基础工程与协议冻结 | 目录、配置、错误、OpenAPI、领域模型 | 1-2 周 |
| M1 | 最小数据面 | API key、OpenAI-compatible chat、路由、provider relay | 2-3 周 |
| M2 | 计费闭环 | balance hold、usage attempt、settlement、ledger、failed replay | 2-3 周 |
| M3 | 多协议兼容 | OpenAI Responses、Claude、Gemini、Embeddings、stream | 2-3 周 |
| M4 | 统一媒体任务 | image/video/audio/music async task、file、callback | 3-5 周 |
| M5 | 控制面与快照 | admin API、模型/渠道/价格/路由/限流配置、snapshot publish | 3-4 周 |
| M6 | 插件化与安全治理 | 插件链、PII、PromptGuard、ResponseGuard、Audit | 2-4 周 |
| M7 | 生产可观测与稳定性 | metrics、tracing、dashboard、压测、故障演练 | 2-4 周 |
| M8 | 商业化运营能力 | 对账、账单、利润、租户报表、模型市场 | 持续 |

---

## 3. M0：基础工程与协议冻结

### 目标

建立项目骨架，让后续所有代码有统一规范。

### 任务

```text
初始化 Go module
建立 cmd/gateway cmd/control-api cmd/worker
建立 internal/bootstrap
建立 configs/local.yaml
建立 pkg/apperr
建立日志、配置、DB、Redis、HTTP server
建立 OpenAPI YAML
建立数据库 migration 框架
建立 Makefile/CI
```

### 验收标准

```text
go test ./... 通过
make lint 通过
GET /healthz 返回 ok
GET /readyz 返回 ready 或依赖错误
GET /metrics 可访问
OpenAPI 可导入 Apifox
目录结构符合 architecture 文档
```

### 风险

| 风险 | 处理 |
|---|---|
| 目录过早复杂 | 先创建核心目录，不写空包过多 |
| OpenAPI 频繁变更 | URI 冻结，schema 可版本化 |
| 语言/框架争论 | 数据面以 Go + net/http 为默认 |

---

## 4. M1：最小数据面

### 目标

完成一个可运行的最小 AI Gateway：

```text
/v1/chat/completions
API key auth
model parse
route planning
provider relay
basic usage parse
basic metrics
```

### 交付

```text
GatewayEngine.Handle
RequestParser for OpenAI chat
APIKeyAuth
RuntimeSnapshot minimal
ProviderRegistry
OpenAI-compatible ProviderAdapter
PrioritySelector
WeightedRandomSelector
Access log / metrics / trace
```

### 关键实现顺序

1. `RuntimeSnapshot` 最小结构：api_keys、models、channels、mappings、route_policies。
2. API key hash 认证。
3. `RequestParser` 解析 chat body。
4. `RoutePlanner` 生成 candidate。
5. `ProviderDispatcher` 调用 OpenAI-compatible adapter。
6. HTTP response 原样返回。
7. Provider error 归一化。

### 验收标准

```text
使用 curl 调用 /v1/chat/completions 成功
无效 API key 返回 401
无权限模型返回 403
无路由返回 404/503
provider 5xx 返回标准错误
weighted_random 有流量分布测试
基础 metrics 有 provider_attempts_total
trace 包含 gateway.route 和 gateway.provider_attempt
```

---

## 5. M2：计费闭环

### 目标

正式支持 API 转售的最小账务闭环。

### 交付

```text
balances
balance_holds
usage_attempts
usage_records
ledger_entries
failed_settlements
AdmissionController
SettlementService
FailedSettlementReplayer worker
```

### 请求前

```text
估算 usage
估算费用
检查余额
创建 balance_hold
```

### 请求后

```text
记录 usage_attempt
解析真实 usage
创建 usage_record
写 ledger_entry
扣 balance
更新 hold 为 settled
```

### 失败场景

| 场景 | 处理 |
|---|---|
| provider 未调用前失败 | release hold |
| provider 调用失败 | record failed attempt，release hold |
| provider 成功但 settlement 失败 | keep hold，写 failed_settlement |
| stream 中途 client 断开 | 按 billability policy 决定 settle/release |
| worker replay 失败 | retry + backoff + dead letter |

### 验收标准

```text
重复 request_id 不重复扣费
provider 成功但 DB settlement 故障后会写 failed_settlement
worker 恢复后能 replay 成功
ledger sum 与 balance 一致
所有扣费都有 usage_record 和 ledger_entry
```

---

## 6. M3：多协议兼容

### 目标

对外支持 OpenAI / Claude / Gemini 主流协议。

### 交付

```text
/v1/responses
/v1/embeddings
/v1/messages
/v1beta/models/{model}:generateContent
/v1beta/models/{model}:streamGenerateContent
streaming writer
provider stream usage parser
```

### 验收标准

```text
OpenAI SDK 可用
Anthropic SDK 或兼容请求可用
Gemini generateContent 请求可用
SSE stream 可用
client disconnect 不误熔断 provider
stream close 时结算
```

---

## 7. M4：统一媒体任务

### 目标

支持网剧 Agent 所需的生图、生视频、音频、音乐、文件、异步任务。

### 交付

```text
/v1/images/generations
/v1/images/edits
/v1/videos/generations
/v1/audio/speech
/v1/audio/transcriptions
/v1/music/generations
/v1/tasks/{task_id}
/v1/tasks/{task_id}/cancel
/v1/files/upload/base64
/v1/files/upload/stream
/v1/files/upload/url
ProviderTaskDispatcher
ProviderTaskPoller worker
CallbackOutbox
```

### 异步任务关键点

```text
创建 internal task 后再调用 provider
provider external_task_id 必须落库
polling 和 webhook 都能驱动状态更新
结果资产统一 FileAsset
callback 失败写 callback_outbox
任务最终结算
```

### 验收标准

```text
视频生成返回 task
GET task 可查状态
worker 可轮询 provider task
任务成功后能结算
callback 失败可重试
任务取消可调用 provider cancel 或本地标记
```

---

## 8. M5：控制面与快照

### 目标

实现“不发版新增模型/渠道/价格/路由/限流”。

### 交付

```text
Admin tenant/project/api_key API
Model catalog API
Provider channel API
Provider credential encryption
Route policy API
Price rule API
Limit rule API
Plugin binding API
Snapshot builder
Snapshot validator
Snapshot publisher
Gateway snapshot watcher
```

### 快照发布流程

```text
保存配置到 DB
validate config
build runtime snapshot
dry-run route validation
persist snapshot
publish active version to Redis/configd
gateway watch and hot reload
```

### 验收标准

```text
新增模型无需重启 gateway
新增 channel 无需重启 gateway
修改价格后新请求生效
旧请求结算使用请求时 pinned price
发布坏 snapshot 被 validator 拒绝
snapshot version 出现在 metrics 和 response diagnostic header
```

---

## 9. M6：插件化与安全治理

### 目标

把安全、审计、脱敏、prompt guard、response guard 变成可配置插件。

### 交付

```text
Plugin interface
PluginManager
PluginBinding in snapshot
APIKeyAuthPlugin
RequestSizePlugin
PromptTokenLimitPlugin
PiiRedactionPlugin
PromptGuardPlugin
ResponseGuardPlugin
CostGuardPlugin
AuditLogPlugin
LLMMetricPlugin
CallbackPlugin
```

### 验收标准

```text
插件可按 tenant/project/model 绑定
插件按 phase 和 priority 执行
插件可 deny 请求
插件可写 audit event
PII 不进入 access log
PromptGuard 命中后返回 policy_denied
```

---

## 10. M7：生产可观测与稳定性

### 目标

达到可灰度商用的运维水平。

### 交付

```text
Prometheus dashboard
Grafana dashboard
OTel tracing
structured access logs
provider health dashboard
settlement backlog dashboard
task backlog dashboard
alert rules
failure drills
load tests
```

### 故障演练

必须覆盖：

```text
Redis 不可用
DB 短暂不可用
provider 429
provider 5xx
provider stream 中断
client stream 断开
settlement 写库失败
callback 失败
snapshot 发布失败
worker 重启
```

### 验收标准

```text
压测达到目标 QPS / 并发 stream
失败 settlement backlog 有告警
provider 429 触发 fallback
client disconnect 不触发 provider circuit penalty
Redis 限流故障时 fail closed 或按配置 fail open
```

---

## 11. M8：商业化运营能力

### 目标

支持正式商业运营。

### 交付

```text
usage report
cost report
profit report
tenant dashboard
充值/扣费流水
发票/账单导出
渠道成本配置
模型市场配置
Agent workflow metadata report
```

### 验收标准

```text
客户可查余额和用量
运营可查渠道利润
财务可对账
失败账务可追踪
模型维度成本可分析
Agent 场景/镜头维度可分析
```

---

## 12. 版本发布建议

```text
v0.1 internal alpha: M0 + M1
v0.2 billing alpha: M2
v0.3 protocol beta: M3
v0.4 media beta: M4
v0.5 control plane beta: M5
v0.6 security beta: M6
v0.7 production candidate: M7
v1.0 commercial launch: M8 核心能力
```

---

## 13. 每个阶段的通用 Definition of Done

每个功能必须满足：

```text
代码完成
单元测试完成
集成测试完成
错误码规范完成
metrics 完成
trace span 完成
access log 字段完成
审计要求完成
配置项完成
OpenAPI 更新
文档更新
故障场景说明
```
