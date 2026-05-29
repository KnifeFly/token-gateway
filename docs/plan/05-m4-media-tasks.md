# M4 Unified Media Async Task

## 阶段目标

支持网剧 Agent 和多模态工作流需要的生图、生视频、音频、音乐、文件上传、异步任务、任务查询、幂等提交和 callback。

## 交付物

- `/v1/images/generations`、`/v1/images/edits`、`/v1/videos/generations`、`/v1/audio/speech`、`/v1/audio/transcriptions` 和 `/v1/music/generations`。
- `/v1/tasks/{task_id}`、`/v1/tasks/{task_id}/cancel`。
- `/v1/files/upload/base64`、`/v1/files/upload/stream`、`/v1/files/upload/url` 和文件 quota。
- Task service、File service、ProviderTaskDispatcher、ProviderTaskPoller、Result normalizer 和 CallbackOutbox。
- `Idempotency-Key` 与 request body hash 记录。
- task settlement 和 callback retry。

## 核心实现顺序

1. 实现 media request parser 和 model schema validation。
2. 统一 file asset，支持 base64、stream 和 URL 上传。
3. 校验 `Idempotency-Key`；同 key 同 body 返回原 task，同 key 不同 body 返回 `idempotency_conflict`。
4. 请求通过 price precheck 和 balance hold 后创建 internal task。
5. ProviderTaskDispatcher 提交外部任务并落库 external_task_id。
6. worker 通过 polling 或 webhook 更新任务状态。
7. Result normalizer 把 provider 结果转为统一资产。
8. 任务完成后结算并通过 callback outbox 通知客户。

## 关键设计约束

- 统一 URI 表达能力，具体模型差异放到 `model` 和 `model_params`。
- internal task 必须先于 provider task 建立，避免外部任务失联。
- external_task_id 必须落库。
- callback 失败不阻塞任务完成，必须写 outbox 重试。
- 幂等记录必须与任务和 balance hold 绑定，避免重复任务和重复扣费。
- 异步任务最终也必须进入 settlement 和 ledger。

## 验收标准

- 视频生成接口返回 task object。
- `GET /v1/tasks/{task_id}` 可查询状态。
- worker 可轮询 provider task 并更新状态。
- 任务成功后完成结算。
- callback 失败后可重试。
- 任务取消可调用 provider cancel 或进行本地取消标记。
- 重复 `Idempotency-Key` 返回同一个 task。
- 同 key 不同 body 返回 409 `idempotency_conflict`。

## 风险与处理

| 风险 | 处理 |
|---|---|
| provider task 提交成功但本地未记录 | 先建 internal task，再提交外部任务 |
| 回调失败导致客户不可见 | callback outbox 独立重试并记录状态 |
| 文件来源不统一 | 统一 FileAsset 作为素材和结果资产 |
| 客户端重试导致重复扣费 | Idempotency-Key 绑定 task、body hash 和 hold |

## 设计来源

- [实施计划 M4](../design/ai_gateway_implementation_plan.md)
- [系统设计异步媒体任务](../design/ai_gateway_system_design.md)
- [架构设计异步任务架构](../design/ai_gateway_architecture_design.md)
- [OpenAPI 合同媒体和任务接口](../design/ai_gateway_openapi.yaml)
- [任务清单 M4](../tasks.md)
