# P5 Provider 协议兼容

## 阶段目标

P5 的目标是把当前已接入的 OpenAI-compatible、Claude Messages 和 Gemini GenerateContent 从“主路径可用”推进到“客户 SDK 兼容可验证”。本阶段优先补协议行为、错误映射、usage 归一化和 contract tests，不新增控制面安全、复杂账务、对象存储或 Realtime 范围。

## 交付物

- Provider 协议兼容矩阵：列明 OpenAI、Claude、Gemini 各 endpoint 的 request/response、stream、tool calling、多模态、usage、error 和 SDK 覆盖状态。
- OpenAI-compatible 补齐：chat/responses/embeddings/moderations/images/audio 的 JSON、multipart、stream、tool calling 和 usage/error 映射。
- Claude Messages 补齐：messages、tools、stream event、multimodal content block、stop reason、usage 和错误码映射。
- Gemini GenerateContent 补齐：generateContent、streamGenerateContent、contents/tools/safetySettings、usageMetadata、finishReason 和错误码映射。
- SDK/HTTP contract tests：覆盖真实 SDK 请求形状、协议消歧、上游错误、stream close、usage 归一化和不支持能力的稳定错误。

## 兼容矩阵

| Provider family | Endpoint | request/response | stream | tool calling | multimodal | usage | error | 覆盖 |
|---|---|---|---|---|---|---|---|---|
| OpenAI compatible | `/v1/chat/completions` | supported；未知 JSON 字段透传，按路由改写 model | supported；支持 usage chunk 归一化 | supported | supported content array | prompt/completion/input/output/cache/reasoning/audio | stable `provider_*` | focused + contract |
| OpenAI compatible | `/v1/responses` | supported；未知 JSON 字段透传，按路由改写 model | supported；支持 `response.completed.response.usage` | supported | supported passthrough | input/output/cache/reasoning/audio | stable `provider_*` | focused |
| OpenAI compatible | `/v1/embeddings` | supported；input/dimensions/encoding_format 透传 | n/a | n/a | text/token passthrough | prompt/input/total | stable `provider_*` | focused |
| OpenAI compatible | `/v1/moderations` | supported；input/metadata 透传 | n/a | n/a | text passthrough | prompt/input/total if returned | stable `provider_*` | focused |
| OpenAI compatible | `/v1/images/*`, `/v1/audio/*` | supported；JSON/multipart model rewrite，文件字段保留 | speech/transcription streaming not enabled | n/a | multipart/file passthrough | provider usage if returned | stable `provider_*` | focused |
| Claude Messages | `/v1/messages` | supported；Anthropic headers 保真，按路由改写 model | supported；返回后连接不被提前 cancel，usage 增量合并 | supported | supported content block | input/output/cache create/cache read | stable `provider_*` | focused + contract |
| Gemini GenerateContent | `/v1beta/models/{model}:generateContent` | supported；`/v1beta` base URL 正确拼接，body 保真 | n/a | supported | supported parts/modality | prompt/candidate/cache/thoughts/audio/image/video | stable `provider_*` | focused + contract |
| Gemini GenerateContent | `/v1beta/models/{model}:streamGenerateContent` | supported；body 保真，URL 使用路由模型 | supported；返回后连接不被提前 cancel，usage 增量合并 | supported | supported parts/modality | prompt/candidate/cache/thoughts/audio/image/video | stable `provider_*` | focused |

## 核心实现顺序

1. 建立 provider protocol compatibility matrix，把每个 endpoint 的支持级别拆为 supported、partial、unsupported 和 not planned。
2. 先补 parser/adapter 的字段保真：未知字段透传，上游响应字段不被错误裁剪，敏感字段仍按 redaction 规则处理。
3. 补齐 stream 行为：事件格式、结束事件、usage 提取、客户端断开和 provider 错误映射与目标 SDK 行为一致。
4. 补齐 tool calling 和多模态 message 的 request/response 映射，确保路由、计费、审计只依赖归一化 usage 和 request state。
5. 建立 SDK 合同测试夹具，优先覆盖 OpenAI、Claude、Gemini 官方 SDK 的最小请求、stream 请求和错误响应。
6. 同步 OpenAPI、runbook 和任务板，明确仍未支持的 provider-specific 行为和推荐替代路径。

## 关键设计约束

- Public URI 继续表达能力，不新增供应商品牌化 public route。
- Provider adapter 只负责协议翻译、上游调用、错误映射、usage 解析和任务提交/轮询，不接管路由、策略、账务或租户决策。
- 协议兼容优先保证客户 SDK 的 wire shape、stream event 和错误码稳定；不把 provider 私有能力提升为核心 gateway 依赖。
- 不支持能力必须返回稳定错误，不能静默降级成错误模型、错误 provider 或错误计费。
- 本阶段不新增完整 Realtime、semantic routing/cache、WASM/动态插件、控制面 RBAC/审计平台、复杂 invoice 或对象存储。

## 验收标准

- OpenAI、Claude、Gemini 的兼容矩阵能直接说明每个 endpoint 的支持边界和测试覆盖。
- SDK/HTTP contract tests 覆盖 chat/messages/generateContent 的非流式、流式、tool calling、usage 和错误路径。
- 上游 400/401/403/404/429/5xx、超时和 stream 中断能映射为稳定 gateway 错误，不泄露 provider secret 或原始敏感内容。
- usage 归一化后的 input/output/cache/audio/image/video 等字段能被结算、报表和审计复用。
- `docs/design/ai_gateway_openapi.yaml` 与协议行为一致，未支持能力标注清楚。

## 执行记录

| 任务 | 状态 | 验证 |
|---|---|---|
| P5/E15-T01 provider 协议兼容矩阵 | 已完成 | 本文和 `docs/runbook/provider-protocol-compatibility.md` 固化 OpenAI、Claude、Gemini endpoint 的 request/response、stream、tool、多模态、usage、error 和测试覆盖状态 |
| P5/E15-T02 OpenAI-compatible wire shape | 已完成 | OpenAI adapter 保留未知 JSON/multipart 字段、按路由改写 model、补 usage details/Responses completed usage、稳定 provider error；`go test ./internal/provider/openai ./internal/dataplane/parser ./tests/contract` 通过 |
| P5/E15-T03 Claude Messages 兼容行为 | 已完成 | Claude adapter 保留 Anthropic header、按路由改写 model、修复 stream context 生命周期、合并 stream usage 和 cache usage；`go test ./internal/provider/claude ./tests/contract` 通过 |
| P5/E15-T04 Gemini GenerateContent 兼容行为 | 已完成 | Gemini adapter 支持 `/v1beta` base URL、tool/safety/multimodal passthrough、修复 stream context 生命周期、归一化 modality usage；`go test ./internal/provider/gemini ./tests/contract` 通过 |
| P5/E15-T05 usage/error normalization | 已完成 | `tokenusage.Actual` 扩展 cache/reasoning/audio/image/video 字段，`relay.ErrorFromStatus`/`ErrorFromRequestFailure` 统一 provider error class；`go test ./pkg/tokenusage ./internal/provider/relay ./internal/dataplane/dispatch` 通过 |
| P5/E15-T06 SDK/HTTP contract tests | 已完成 | 新增 `tests/contract/provider_protocol_contract_test.go`，覆盖 OpenAI tools、Claude multimodal/tools/header 和 Gemini tools/safety base URL wire shape |
| P5/E15-T07 OpenAPI 和运行文档同步 | 已完成 | OpenAPI 增加 P5 兼容边界、tool-calls 消息、usage detail 和 provider error codes；新增 provider protocol compatibility runbook |

## 风险与处理

| 风险 | 处理 |
|---|---|
| SDK 行为随版本变化 | 固定测试 SDK 版本，并用 HTTP fixture 覆盖 wire shape |
| 兼容字段过度侵入核心生命周期 | 字段映射限制在 parser/provider adapter 边界 |
| 不支持能力被误当作成功降级 | unsupported path 必须返回稳定错误并有测试 |
| usage 字段口径不一致 | 建立 provider usage normalization 表，结算只读归一化结果 |

## 设计来源

- [路线图](./00-roadmap.md)
- [P4 发布候选与商用上线验收](./15-p4-release-candidate-readiness.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单](../tasks.md)
