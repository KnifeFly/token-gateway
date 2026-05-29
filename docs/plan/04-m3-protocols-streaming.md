# M3 Streaming + Native Compatible

## 阶段目标

在最小 OpenAI Chat 链路基础上扩展 OpenAI stream、Responses、Embeddings、Claude Messages、Gemini GenerateContent 和主流 stream 能力，并覆盖 Native Compatible 与 Unified Media 共享 URI 的协议消歧测试。

## 交付物

- `/v1/responses`、`/v1/embeddings`、`/v1/messages`、`/v1beta/models/{model}:generateContent` 和 `streamGenerateContent`。
- Claude、Gemini 和 OpenAI Responses adapter。
- ProviderStream interface、stream writer、StreamFinalizer 和 downstream error report。
- stream usage parser、first token latency 和 close-time settlement。
- 多协议错误映射、`ambiguous_protocol` 映射和 conformance tests。

## 核心实现顺序

1. 扩展 APIClassifier 和 RequestParser，识别 OpenAI、Claude、Gemini canonical API，并支持 `X-Gateway-Protocol`、model registry 和 body schema 消歧。
2. 为不同协议实现 adapter，核心 engine 继续使用统一 relay request/response。
3. 实现 ProviderStream 和 gateway stream writer。
4. StreamFinalizer 包装上游 stream，记录 chunk、first token、usage 和 downstream error。
5. stream close 时调用 settlement 并释放 limit/hold。
6. 补齐 SDK 或兼容请求的 e2e 测试。

## 关键设计约束

- 已经向客户端写出 token 后，不允许透明 retry 到另一个 provider。
- 客户端断开但已收到部分 token 时，按 billability policy 决定 settle 或 release。
- Provider stream 失败但客户端已收到可用内容时，记录 provider failure 和 billable outcome。
- 协议差异留在 transport 和 adapter 边界，不穿透核心 engine。
- 无法判断协议族时返回标准 `ambiguous_protocol`，不能猜测执行。

## 验收标准

- OpenAI SDK 可调用 Responses 和 Embeddings。
- Anthropic SDK 或兼容请求可调用 `/v1/messages`。
- Gemini generateContent 和 streamGenerateContent 可用。
- SSE stream 可用。
- client disconnect 不误触 provider circuit penalty。
- stream close 时完成最终 accounting。
- URI 冲突场景有 classifier/e2e 覆盖。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 多协议模型差异污染核心层 | 统一 ParsedRequest 和 Provider relay 类型 |
| stream 中途失败账务不一致 | StreamFinalizer 成为唯一 close-time accounting 入口 |
| usage 缺失 | adapter parser 和 fallback estimate 明确标记来源 |
| URI 冲突误路由 | classifier 使用 header、model registry、body schema 并覆盖 ambiguous case |

## 设计来源

- [实施计划 M3](../design/ai_gateway_implementation_plan.md)
- [系统设计流式请求](../design/ai_gateway_system_design.md)
- [架构设计 Provider Dispatch](../design/ai_gateway_architecture_design.md)
- [代码蓝图 StreamFinalizer](../design/ai_gateway_code_blueprint.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
