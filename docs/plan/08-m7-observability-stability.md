# M7 Observability + Performance

## 阶段目标

让系统达到可灰度商用的运维水平。请求、provider、账务、任务、callback、snapshot、插件和性能预算都必须能被定位、监控、告警和演练。

## 交付物

- Prometheus metrics、Grafana dashboard、OTel tracing 和 structured access logs。
- provider health、settlement backlog、task backlog、callback retry 和 snapshot staleness dashboard。
- alert rules、load test scripts 和 failure drills。
- 性能预算和压测报告。
- Redis、DB、provider、stream、settlement、callback、snapshot 和 worker 重启演练。

## 核心实现顺序

1. 固化 metrics 命名和 label 规范。
2. 补齐 GatewayEngine 每个关键阶段的 trace span。
3. 建立 access log、provider attempt log、settlement log、task lifecycle log、admin audit log 和 security event log。
4. 建立 Grafana dashboard 和 Prometheus alert rules。
5. 编写 chat、video、stream 压测脚本，并输出 QPS、stream concurrency、Redis 延迟和 provider latency 报告。
6. 编写 failure drills 并纳入发布前检查。

## 关键设计约束

- 所有日志必须结构化，并带 request_id、trace_id、tenant_id、project_id。
- metrics label 和 trace attribute 不能包含敏感值、高基数字段或原始 prompt/response。
- client disconnect 不应被误判为 provider 熔断惩罚。
- Redis 限流故障必须按配置 fail closed 或 fail open，不能隐式放行。
- 性能预算必须覆盖 gateway hot path、Redis Lua、provider attempt 和 stream close finalization。

## 验收标准

- 压测达到目标 QPS 和并发 stream。
- failed settlement backlog 有告警。
- provider 429 触发 fallback。
- client disconnect 不触发 provider circuit penalty。
- Redis 限流故障按配置 fail closed 或 fail open。
- dashboard 能定位 provider、task、billing 和 snapshot 问题。
- 压测报告覆盖 QPS、stream concurrency、Redis 延迟和 provider latency。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 指标过多但不可用 | 围绕请求、provider、账务、任务和 snapshot 五条主线建 dashboard |
| 高基数 label 压垮 metrics | request_id、prompt、response、API key 不进 label |
| 故障演练只停留文档 | failure drills 提供可运行脚本和验收输出 |
| 性能目标不可验证 | 每个核心路径都有预算和压测输出 |

## 设计来源

- [实施计划 M7](../design/ai_gateway_implementation_plan.md)
- [系统设计可观测性](../design/ai_gateway_system_design.md)
- [架构设计商用设计底线](../design/ai_gateway_architecture_design.md)
- [任务清单 M7](../tasks.md)
