# M7 Performance Budget

Initial gray-release budget:

| Path | Budget |
|---|---:|
| Gateway non-stream hot path, excluding provider latency | 500 QPS per instance |
| API classify + auth + route p99 | < 20 ms |
| Redis limit acquire p95 | < 5 ms |
| Provider attempt p95 | tracked by upstream/provider SLO |
| Stream close finalization p99 | < 50 ms excluding settlement storage latency |
| SSE stream concurrency | 5k concurrent streams per instance target |

Load test:

```bash
go run ./tools/loadtest -url http://127.0.0.1:8080/v1/chat/completions -api-key tg-local-dev-key -requests 1000 -concurrency 100
go run ./tools/loadtest -stream -requests 500 -concurrency 100
go run ./tools/loadtest -redis 127.0.0.1:6379 -redis-samples 100
```

Release evidence should include QPS, p95/p99 latency, stream max concurrency,
Redis latency p50/p95, provider latency p95, and the failure drill output.
