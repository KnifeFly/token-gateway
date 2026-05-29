# M1 最小非流式数据面

## 阶段目标

跑通一个可运行的最小非流式 AI Gateway：客户使用 `/v1/chat/completions` non-stream，经过 APIClassifier、API key 认证、model 解析、runtime snapshot 路由、provider relay、错误归一化和基础观测。

## 交付物

- `GatewayEngine.Handle` 主流程和 `RequestState`。
- APIClassifier、OpenAI Chat request parser、CredentialExtractor、Authenticator 和 PolicyEvaluator。
- 最小 RuntimeSnapshot、IndexedSnapshot 和 gateway snapshot cache。
- RoutePlanner、PolicyResolver、ModelResolver、CandidateResolver、PrioritySelector 和 WeightedRandomSelector。
- Provider relay types、ProviderRegistry、OpenAI-compatible adapter、provider error mapping 和 usage parser。
- ProviderDispatcher、retry/fallback 基础控制和 provider attempt 记录。
- basic settlement mock，为 M2 正式扣费预留接口。
- access log、metrics 和 trace span。

## 核心实现顺序

1. 建立 RuntimeSnapshot 最小结构，覆盖 api keys、models、channels、mappings 和 route policies。
2. 实现 APIClassifier，M1 至少能识别 OpenAI Chat canonical API。
3. 实现 API key hash 认证，写入 Principal。
4. 实现 OpenAI Chat parser，解析 model、messages、stream flag 和 usage estimate；M1 对 stream 返回明确未支持或交给 M3。
5. 实现 RoutePlanner，输出候选 provider/channel 列表。
6. 实现 ProviderDispatcher 调用 OpenAI-compatible adapter。
7. 写回 OpenAI-compatible 响应和标准错误。
8. 补齐 provider attempt、metrics 和 tracing。

## 关键设计约束

- 数据面热路径只读 snapshot 和索引。
- `model` 不能当普通字符串透传，必须进入模型解析和路由。
- Provider adapter 不拥有路由、租户策略、计费或限流决策。
- M1 不实现 streaming；已经请求 stream 时必须显式拒绝或走受控 feature flag。
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
- stream 请求不会误走非流式实现。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 最小链路混入计费细节 | M1 只保留 usage estimate、attempt 记录和 settlement mock，正式扣费放到 M2 |
| provider 形状泄漏到核心层 | request/response 在 adapter 边界转换 |
| snapshot 过度设计 | 先做最小字段和索引，M5 再做发布治理 |

## 设计来源

- [实施计划 M1](../design/ai_gateway_implementation_plan.md)
- [系统设计同步 LLM 请求生命周期](../design/ai_gateway_system_design.md)
- [架构设计 dataplane/routing/provider dispatch](../design/ai_gateway_architecture_design.md)
- [代码蓝图 GatewayEngine](../design/ai_gateway_code_blueprint.md)
- [任务清单 M1](../tasks.md)
