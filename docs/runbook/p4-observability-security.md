# P4 Observability And Security Release Gate

## Gate

`tests/failure/release_gate.sh` 是 P4 发布级观测与安全验收入口。默认模式不要求本地 gateway 正在运行，会完成：

- Dashboard JSON 解析。
- Alert/dashboard 合同检查：provider attempt、billing backlog、snapshot stale、snapshot publish error、worker job、callback retry。
- 敏感串扫描：常见 provider token、云 access key、GitHub token 和 Slack token 不允许出现在 `configs`、`deployments`、`docs`、`tests`。
- Focused tests：`pkg/redaction`、`internal/infra/telemetry`、`internal/dataplane/observe`、`internal/worker`。
- Loadtest Redis latency 输出合同检查；提供 Redis 地址时会真实采样 `redis_latency_p95_ms`。

## Commands

```bash
tests/failure/release_gate.sh

TOKEN_GATEWAY_RELEASE_GATE_REDIS_ADDR=127.0.0.1:6379 \
  tests/failure/release_gate.sh

TOKEN_GATEWAY_RELEASE_GATE_LIVE=true \
GATEWAY_URL=http://127.0.0.1:9501 \
API_KEY=tg-local-dev-key \
MODEL=gpt-4o-mini \
  tests/failure/release_gate.sh
```

## Acceptance

- `release_gate=passed` 必须出现。
- Live gate 需要同时看到 `drills=passed`、loadtest `latency_p95_ms` 和 Redis `redis_latency_p95_ms`。
- Alert rules 至少覆盖 failed settlement backlog、provider 5xx/429、snapshot stale/publish error、worker job failure 和 callback retries。
- Dashboard 至少覆盖 HTTP QPS/latency、provider attempts/latency、billing repair backlog、task/callback health、worker job outcomes、snapshot staleness 和 fallback/degradation。
- 所有 provider secret 只允许通过本地环境变量或 control-api 加密写入，不允许进入仓库文件、日志、metrics label 或 trace attribute。
