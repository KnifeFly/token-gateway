# Configd Snapshot Distribution

## 模式

P4 起，configd 在 DB active snapshot 成功切换后，同步写入 Redis durable active key，并向 Redis pubsub channel 发布轻量事件。Gateway watcher 优先读取 Redis active key；Redis 不可用、key 缺失或 payload 校验失败时，回落到 DB active snapshot polling。定时 polling 仍保留，用于 pubsub 丢失、gateway 重启和 Redis 订阅中断后的自愈。

## Redis Key

- Active key：`{gateway.limits.key_prefix}:snapshot:active`
- Event channel：`{gateway.limits.key_prefix}:snapshot:events`

Active key 保存 envelope，包含 `version`、`checksum`、`created_at`、`published_at` 和完整 runtime snapshot。Gateway 读取后会重新执行 runtime snapshot validation 和 checksum/version 一致性检查。

## 验证

```bash
export ADMIN_TOKEN=local-admin-token
export CONFIGD=http://127.0.0.1:9504
export REDIS_KEY_PREFIX=token-gateway-local

curl -fsS -X POST -H "X-Admin-Token: ${ADMIN_TOKEN}" \
  "${CONFIGD}/configd/snapshots/publish"

redis-cli GET "${REDIS_KEY_PREFIX}:snapshot:active" | jq '.version,.checksum'
redis-cli SUBSCRIBE "${REDIS_KEY_PREFIX}:snapshot:events"
```

Gateway 侧用 `/v1/chat/completions` 响应头 `X-Gateway-Snapshot-Version` 验证新请求已切换到最新版本；rollback 后重复请求确认版本回退。

## 回退

- Redis 读失败：gateway watcher 自动回退 DB active snapshot，并继续按 `control.snapshot_poll_interval` 轮询。
- Redis 写失败：configd publish 返回错误，DB active snapshot 可能已切换；此时先检查 `/configd/snapshots/diagnostics` 和 Redis 连接，再重新 publish。
- payload 校验失败：gateway 拒绝使用 Redis payload 并回退 DB；需要检查 active key 是否被非 configd 进程写入。
