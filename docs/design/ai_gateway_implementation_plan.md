# 商用 AI Gateway 实施计划文档

## 1. 实施原则

1. 先写最小闭环，不先堆 provider。
2. 所有阶段都必须可测试、可回滚。
3. 计费链路优先于协议扩展。
4. 数据面不直接查管理表。
5. `docs/tasks.md` 是唯一任务看板。
6. 每个阶段结束必须补齐 ADR、OpenAPI、测试、迁移脚本。

---

## 2. 阶段总览

| 阶段 | 名称 | 目标 |
|---|---|---|
| M0 | 基础工程与文档归一 | repo、CI、docs、ADR、OpenAPI 位置规范 |
| M1 | 最小非流式数据面 | OpenAI-compatible chat non-stream 跑通 |
| M2 | 账务闭环 | balance hold、settlement、ledger、failed replay |
| M3 | Streaming + Native Compatible | OpenAI stream、Claude、Gemini |
| M4 | Unified Media Async Task | 图像/视频异步任务、Idempotency-Key、callback |
| M5 | Control Plane + Snapshot | 控制面配置、snapshot publish、key revoke blacklist |
| M6 | Plugins + Security | 内置插件、prompt/response guard、redaction |
| M7 | Observability + Performance | 指标、trace、性能预算、压测、故障演练 |
| M8 | Realtime Reserved Extension | session 接口和 WebSocket 架构预留 |
| M9 | Commercial Operations | 对账、报表、租户运营、灾备流程 |

---

## 3. M0：基础工程与文档归一

### 目标

建立 Go 工程骨架和文档规范。

### 任务

```text
- 初始化 cmd/internal/pkg 目录
- 建立 config 加载
- 建立 slog/otel/prometheus 基础设施
- 建立 Makefile
- 建立 CI: test/vet/lint/race
- 将 OpenAPI 归一到 `docs/design/ai_gateway_openapi.yaml`
- 将 ADR 归一到 `docs/design/ai_gateway_ADR.md`
- 将最终任务看板落地为 `docs/tasks.md`
- 删除或标记旧 design task list 为只读
```

### 验收标准

```text
make test 通过
make lint 通过
OpenAPI 可导入 Apifox
`docs/tasks.md` 是唯一任务入口
至少 5 条 ADR 已创建
```

---

## 4. M1：最小非流式数据面

### 目标

跑通：

```text
POST /v1/chat/completions non-stream
```

### 说明

M1 不做 streaming。M1 的认证、路由、provider 调用可以先硬编码或 seed 配置，但结构必须与后续 snapshot 兼容。

### 交付

```text
APIClassifier
RequestParser
API key auth
mock snapshot provider
priority route planner
OpenAI-compatible provider adapter
basic settlement mock
structured logs
```

### 验收标准

```text
curl /v1/chat/completions 成功
无效 API key 返回 401
无权限模型返回 403
不存在模型返回 404
provider 失败返回标准错误
```

---

## 5. M2：计费闭环

### 目标

让系统具备可商用的扣费基础。

### 交付

```text
balances
balance_holds
usage_attempts
usage_records
ledger_entries
failed_settlements
settlement service
failed_settlement_replayer worker
```

### 流程

```text
quote price
reserve balance hold
provider attempt
record usage attempt
settle usage
append ledger
release hold
```

### 验收标准

```text
余额不足拒绝 provider 调用
provider 成功后余额减少
结算失败写 failed_settlement
worker 可重放 failed_settlement
重复 replay 不重复扣费
```

---

## 6. M3：Streaming + Native Compatible

### 目标

支持：

```text
OpenAI-compatible stream
Claude /v1/messages
Gemini generateContent
```

### 交付

```text
SSE writer
AccountingStream
close-time settlement
Claude adapter
Gemini adapter
Protocol classifier conflict tests
```

### 验收标准

```text
stream 正常输出
client disconnect 不惩罚 provider health
已经写出 token 后不透明 fallback
stream close 后完成 settlement
/v1/messages 可用
/v1beta/models/{model}:generateContent 可用
```

---

## 7. M4：Unified Media Async Task

### 目标

支持网剧 Agent 场景。

### 交付

```text
/v1/images/generations
/v1/videos/generations
/v1/tasks/{task_id}
/v1/tasks/{task_id}/cancel
Idempotency-Key
provider_task_poller
callback_dispatcher
file upload
```

### 验收标准

```text
重复 Idempotency-Key 返回同一个 task
同 key 不同 body 返回 409
视频任务可从 queued -> running -> succeeded
callback 可重试
任务成功后完成 settlement
```

---

## 8. M5：Control Plane + Snapshot

### 目标

新增模型、渠道、价格、路由、限流无需发版。

### 交付

```text
control-api CRUD
snapshot builder
snapshot validator
snapshot publisher
configd/gateway subscriber
atomic snapshot store
key revoke blacklist
```

### 验收标准

```text
发布 snapshot 后 gateway 自动切换
错误 snapshot 不生效
API key revoke <2s 生效
price update 不影响已开始请求
route update 对新请求生效
```

---

## 9. M6：Plugins + Security

### 目标

实现配置驱动安全和治理能力。

### 交付

```text
PluginManager
PluginBinding snapshot
RequestSizePlugin
IPAllowlistPlugin
ModelACLPlugin
PromptTokenLimitPlugin
PIIRedactionPlugin
PromptGuardPlugin
ResponseGuardPlugin
CostGuardPlugin
AuditLogPlugin
LLMMetricPlugin
```

### 验收标准

```text
无插件绑定阶段 O(1) skip
prompt guard 可拒绝请求
PII redaction 生效
CostGuard 可拒绝或降级
审计日志不含敏感明文
```

---

## 10. M7：Observability + Performance

### 目标

达到可生产运维。

### 交付

```text
Prometheus histogram
OpenTelemetry traces
structured logs
redaction
dashboard
alerts
load tests
failure drills
```

### 验收标准

```text
压测报告覆盖 QPS、stream concurrency、Redis 延迟
provider 429/5xx 有 retry/fallback 指标
settlement backlog 有指标
snapshot staleness 有指标
所有 P0 故障演练通过
```

---

## 11. M8：Realtime Reserved Extension

### 目标

不完整实现 Realtime，但预留协议和架构。

### 交付

```text
POST /v1/realtime/sessions
GET /v1/realtime/sessions/{session_id}
RealtimeEngine interface
WebSocket handler stub
```

### 验收标准

```text
OpenAPI 有 session 接口
未启用时返回 501
鉴权、审计、metrics 已接入
```

---

## 12. M9：Commercial Operations

### 目标

支持正式商业运营。

### 交付

```text
reconciliation report
tenant usage dashboard
cost report
credit package
manual adjustment audit
backup/restore runbook
disaster recovery drill
```

### 验收标准

```text
每日对账可发现差异
ledger 可追溯
人工调账有审计
备份恢复演练通过
```

---

## 13. 每阶段 Definition of Done

```text
代码合并
单元测试
集成测试
必要 e2e
OpenAPI 更新
ADR 更新
迁移脚本
配置示例
监控指标
日志字段
故障用例
文档更新
```
