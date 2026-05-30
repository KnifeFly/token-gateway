# P7 非存储媒体转发生态

## 阶段目标

P7 的目标是把 Unified Media 能力明确收敛为“非存储输入资产转发 + provider task 生命周期治理”。系统不做对象存储，不承诺下载、生命周期管理或存储 SLA；图片、视频、音频等输入只作为请求资产被验证、归一化并转发给上游 provider。

## 交付物

- 非存储媒体语义说明：URL、base64、multipart 只用于 provider 转发和请求归一化，gateway 不持久化对象内容。
- Media provider adapter contract：统一 submit、poll、cancel、result URL、usage、error、callback 和 provider metadata 映射。
- 真实媒体 provider 映射：优先覆盖 image/video 的关键商业路径，明确每个 provider 的必填字段、结果字段和失败状态。
- Task/result URL 透传：provider 返回的 result URL、asset metadata 和 usage 进入 task response、callback 和 settlement。
- 媒体任务 contract tests：覆盖 submit、poll running/succeeded/failed、cancel、callback、usage 和计费衔接。

## 核心实现顺序

1. 更新设计和 OpenAPI，把 `/v1/files/*` 定义为 transient/non-storage input asset，不再使用 upload/storage 语义承诺。
2. 梳理 Unified Media parser 和 task bridge，确保 URL/base64/multipart 输入只落必要 metadata、hash、size 和 source，不保存对象内容。
3. 抽象 media provider adapter 的字段映射表，按 image、video、audio、music 能力描述 submit/poll/cancel 和结果格式。
4. 选择最关键的 image/video provider 做真实映射，补齐 provider-specific error、usage、result URL 和 callback 字段。
5. 确保 task settlement 只依赖 provider 结果和归一化 usage，不依赖 gateway 本地存储对象。
6. 增加 media contract tests 和 provider fixture，覆盖非存储语义、结果透传和错误状态。

## 关键设计约束

- Gateway 不是对象存储服务，不提供文件持久化、下载加速、生命周期管理、病毒扫描或存储 SLA。
- 对 base64/multipart 输入只能做请求处理所需的大小限制、hash、MIME 探测和临时转发；不得把本地 file URL 当作客户可长期访问地址。
- Provider 返回的结果 URL 由 provider 或客户配置的下游存储承载，gateway 只透传和记录必要 metadata。
- 媒体任务必须继续支持 Idempotency-Key，避免重复任务和重复扣费。
- 本阶段不引入完整开发者门户、复杂 invoice、Realtime 或 semantic cache。

## 验收标准

- OpenAPI 和系统设计明确 `/v1/files/*` 是 transient/non-storage input asset，不再承诺对象存储。
- 至少一个 image 或 video provider 的 submit、poll、result URL、callback 和 settlement 全链路有测试。
- base64、URL、multipart 输入在超限、MIME 不明、provider 拒绝和重复 idempotency key 时行为稳定。
- Task response 和 callback 能返回 provider result URL、usage、provider task id 和错误原因。
- 本地或测试 fixture 不生成持久可下载对象地址，也不要求 gateway 托管媒体内容。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 客户误解 gateway 托管文件 | OpenAPI、系统设计和 portal 文案统一使用 transient/non-storage |
| provider 要求先上传文件 | 优先使用 provider 支持的 URL/base64 入参；必须中转时只做临时请求级处理 |
| 结果 URL 过期导致客户无法取结果 | 在 task/callback 中透传 provider expiry metadata，并建议客户自有存储落盘 |
| 不同 provider 状态机不一致 | Adapter contract 固定 normalized task status 和 provider metadata |

## 设计来源

- [路线图](./00-roadmap.md)
- [P6 Provider 可靠性治理](./17-p6-provider-reliability.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单](../tasks.md)
