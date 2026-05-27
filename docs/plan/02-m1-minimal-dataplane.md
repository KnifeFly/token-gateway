# M1 最小数据面

## 阶段目标

跑通一个可运行的最小 AI Gateway：客户使用 `/v1/chat/completions`，经过 API key 认证、model 解析、runtime snapshot 路由、provider relay、错误归一化和基础观测。

## 交付物

- `GatewayEngine.Handle` 主流程和 `RequestState`。
- OpenAI Chat request parser、CredentialExtractor、Authenticator 和 PolicyEvaluator。
- 最小 RuntimeSnapshot、IndexedSnapshot 和 gateway snapshot cache。
- RoutePlanner、PolicyResolver、ModelResolver、CandidateResolver、PrioritySelector 和 WeightedRandomSelector。
- Provider relay types、ProviderRegistry、OpenAI-compatible adapter、provider error mapping 和 usage parser。
- ProviderDispatcher、retry/fallback 基础控制和 provider attempt 记录。
- access log、metrics 和 trace span。

## 核心实现顺序

1. 建立 RuntimeSnapshot 最小结构，覆盖 api keys、models、channels、mappings 和 route policies。
2. 实现 API key hash 认证，写入 Principal。
3. 实现 OpenAI Chat parser，解析 model、messages、stream 和 usage estimate。
4. 实现 RoutePlanner，输出候选 provider/channel 列表。
5. 实现 ProviderDispatcher 调用 OpenAI-compatible adapter。
6. 写回 OpenAI-compatible 响应和标准错误。
7. 补齐 provider attempt、metrics 和 tracing。

## 关键设计约束

- 数据面热路径只读 snapshot 和索引。
- `model` 不能当普通字符串透传，必须进入模型解析和路由。
- Provider adapter 不拥有路由、租户策略、计费或限流决策。
- 如果无路由或 provider 不可用，返回可诊断的标准错误。

## 验收标准

- 使用 curl 调用 `/v1/chat/completions` 成功。
- 无效 API key 返回 401。
- 无权限模型返回 403。
- 无路由返回 404 或 503。
- provider 5xx 被映射为标准 provider 错误。
- `weighted_random` 有流量分布测试。
- metrics 包含 provider attempt 指标。
- trace 包含 `gateway.route` 和 `gateway.provider_attempt`。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 最小链路混入计费细节 | M1 只保留 usage estimate 和 attempt 记录，正式扣费放到 M2 |
| provider 形状泄漏到核心层 | request/response 在 adapter 边界转换 |
| snapshot 过度设计 | 先做最小字段和索引，M5 再做发布治理 |

## 设计来源

- [实施计划 M1](../design/ai_gateway_implementation_plan_v2.md)
- [系统设计同步 LLM 请求生命周期](../design/ai_gateway_system_design_v2.md)
- [架构设计 dataplane/routing/provider dispatch](../design/ai_gateway_architecture_design_v2.md)
- [代码蓝图 GatewayEngine](../design/ai_gateway_code_blueprint_v2.md)
- [任务清单 Epic 3-7](../design/ai_gateway_task_list_v2.md)
