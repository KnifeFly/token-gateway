# Provider Release Channel Runbook

## 目标

用真实 provider 低额度测试 key 验证 release channel，而不是只依赖 mock upstream。同步 chat provider 继续覆盖 OpenAI-compatible、Claude、Gemini；异步媒体 provider 以 Replicate prediction adapter 作为第一条真实 Unified Media release channel。

## 准备

- 使用 disposable tenant/project/API key 和低额度 provider account。
- Provider secret 只通过 control-api `api_key` 字段注入并由控制面加密保存，不写入仓库、日志、metrics label 或 trace attribute。
- 参考 `configs/provider-release-channels.example.json` 准备模型、channel 和 route payload；所有 `[REDACTED_SECRET]`、`[REPLICATE_VERSION_ID]` 必须在本地或 staging secret 管理里替换。
- Replicate channel 的 `base_url` 使用 `https://api.replicate.com`，`upstream_model` 使用精确 version id。Adapter 会调用 `POST /v1/predictions`、`GET /v1/predictions/{id}` 和 `POST /v1/predictions/{id}/cancel`。

## 配置发布

```bash
export ADMIN_TOKEN=local-admin-token
export CONTROL=http://127.0.0.1:9502
export CONFIGD=http://127.0.0.1:9504

curl -fsS -X POST -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" \
  -d @/path/to/model.json "${CONTROL}/admin/models"

curl -fsS -X POST -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" \
  -d @/path/to/channel.json "${CONTROL}/admin/channels"

curl -fsS -X POST -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" \
  -d @/path/to/route.json "${CONTROL}/admin/routes"

curl -fsS -X POST -H "X-Admin-Token: ${ADMIN_TOKEN}" \
  "${CONFIGD}/configd/snapshots/publish"
```

## 最小回归

- OpenAI-compatible：`/v1/chat/completions` 非流式和流式各 1 次。
- Claude：`/v1/messages` 非流式和流式各 1 次。
- Gemini：`/v1beta/models/{model}:generateContent` 和 `:streamGenerateContent` 各 1 次。
- Replicate Unified Media：创建 image task，确认内部 task 返回 running；worker poll 后 task 变为 succeeded，result 包含 `provider=replicate`、prediction `id`、`output`、`urls`、`metrics`；callback outbox delivered；billing settlement 或 failed settlement replay 没有 backlog。

## 失败处理

- 401/403：确认 channel secret 注入和 credential key 一致，不在日志里打印 secret。
- 429/5xx：保留 provider attempt、worker job metrics 和 task error；降低 batch size 或切回灰度 route。
- prediction 长时间 processing：先取消内部 task，再调用 task cancel 路径确认 Replicate cancel endpoint 返回 2xx。
- result 缺失：保留 prediction id 和 provider request id，禁用该 channel 后 rollback snapshot。

## 来源

- Replicate prediction 创建、轮询和取消接口参考官方文档：https://replicate.com/docs/topics/predictions/create-a-prediction
