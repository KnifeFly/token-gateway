# Staging Rollout Runbook

## 目标

把当前 P4 release candidate 推到 staging，并用真实 provider、真实 MySQL/Redis、configd snapshot、worker job、观测告警和回滚流程验证商用上线前的最小闭环。

## 发布前准备

- 记录 release commit、migration 版本、镜像 tag、配置仓库版本和 operator。
- 准备 disposable tenant/project/API key，生产客户 key 不参与 staging 演练。
- 准备低额度 provider 测试 key：OpenAI-compatible、Claude、Gemini、Replicate。Provider secret 只进入 secret manager 或 control-api 加密写入，不写入仓库文件。
- 确认 MySQL/Redis 是 staging 专用资源，Redis key prefix 使用本次环境名，例如 `token-gateway-staging-rc1`。
- 确认 dashboard、alert、日志、trace exporter、告警接收人和静默窗口已配置。

## 部署顺序

1. 部署 MySQL、Redis，并确认网络、TLS、账号和连接池配置。
2. 执行 migration：

```bash
go run ./cmd/migrate -config configs/staging.yaml -direction up
```

3. 启动 control-api、configd、worker、gateway。
4. 逐个检查 `/healthz`、`/readyz`、`/metrics`。
5. 通过 control-api 写入 tenant、project、API key、model、channel、route、price、limit、plugin。
6. 发布 snapshot：

```bash
curl -fsS -X POST -H "X-Admin-Token: ${ADMIN_TOKEN}" \
  "${CONFIGD_URL}/configd/snapshots/publish"
```

7. 验证 Redis active snapshot key：

```bash
redis-cli GET "${REDIS_KEY_PREFIX}:snapshot:active" | jq '.version,.checksum'
```

8. 验证 gateway 新请求响应头 `X-Gateway-Snapshot-Version` 已切到最新版本。

## 真实请求验收

- OpenAI-compatible：`/v1/chat/completions` 非流式和流式各 3 次。
- Claude：`/v1/messages` 非流式和流式各 3 次。
- Gemini：`/v1beta/models/{model}:generateContent` 和 `:streamGenerateContent` 各 3 次。
- Replicate Unified Media：创建 image task，确认 task running；worker poll 后 succeeded；callback delivered；settlement 写入 usage/ledger；failed settlement backlog 为 0。
- Public API：`/v1/models`、`/v1/models/{model}/schema`、`/v1/credits`、task get/cancel、file quota。
- Control API：随机抽样 tenant/project/api-key/model/channel/route/price/limit/plugin/snapshot/emergency endpoint。

## Release Gate

```bash
tests/failure/release_gate.sh

TOKEN_GATEWAY_RELEASE_GATE_REDIS_ADDR="${REDIS_ADDR}" \
  tests/failure/release_gate.sh

TOKEN_GATEWAY_RELEASE_GATE_LIVE=true \
GATEWAY_URL="${GATEWAY_URL}" \
API_KEY="${API_KEY}" \
MODEL="${MODEL}" \
  tests/failure/release_gate.sh
```

通过标准：

- `release_gate=passed`
- `drills=passed`
- loadtest 输出 `latency_p95_ms`、`latency_p99_ms`
- Redis latency 输出 `redis_latency_p95_ms`
- worker metrics 有 `provider_task_poller`、`failed_settlement_replayer`、`callback_dispatcher`、`balance_hold_reaper`、`reconciliation`
- alert rules 没有 firing page 级告警

## 故障演练

- Provider 5xx/429：临时禁用 provider/channel，确认新请求不再选中该 channel。
- Redis 不可用：确认 gateway 保留 last-known-good snapshot，hard stale policy 生效。
- Configd 重启：确认 Redis active key 和 DB active snapshot 均可恢复。
- Worker 重启：确认 lease 生效，任务 poll、callback、settlement 不重复推进。
- Billing settlement 失败：注入失败记录，确认 failed settlement replay 后 backlog 回到 0。
- Snapshot rollback：发布两版 snapshot，rollback 后新请求回到上一版。

## 回滚

优先回滚 snapshot，不先回滚数据库：

```bash
curl -fsS -X POST -H "X-Admin-Token: ${ADMIN_TOKEN}" \
  "${CONFIGD_URL}/configd/snapshots/rollback"
```

回滚后检查：

- `X-Gateway-Snapshot-Version` 回到上一版。
- Redis active key version 与 configd diagnostics active version 一致。
- provider attempt error rate 恢复。
- failed settlement backlog 没有新增。

代码或镜像回滚只在 snapshot rollback 无法恢复时执行。Migration down 仅限明确可逆、无数据破坏且经 owner 批准的场景。

## 数据核对

- Tenant balance available + held 与 ledger expected total 一致。
- Usage records 与 provider attempts 可按 request_id 对上。
- Task terminal state、callback outbox、settlement ledger 无遗漏。
- Reconciliation report 无 issue，或 issue 已记录 owner、影响范围和修复计划。
- Backup/restore drill 的 snapshot version、migration version、reconciliation result 已记录。

## 发布记录模板

| 字段 | 值 |
|---|---|
| Release commit |  |
| Branch / PR |  |
| Image tag |  |
| Migration latest |  |
| Snapshot version |  |
| Redis key prefix |  |
| Provider channels |  |
| Release gate result |  |
| Loadtest p95 / p99 |  |
| Redis p95 |  |
| Alerts firing |  |
| Rollback tested |  |
| Reconciliation result |  |
| Known risks |  |
| Owner / time |  |
