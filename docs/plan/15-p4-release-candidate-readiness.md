# P4 发布候选与商用上线验收

## 阶段目标

在 M0-M9、P0-P3 已经补齐主体能力和生产语义后，P4 的目标不是继续扩大产品面，而是把当前实现推进到可发布候选状态：在干净依赖环境和真实 provider 条件下验证四平面进程、账务、限流、snapshot、worker、异步任务、回调、OpenAPI 合同、监控告警和回滚能力。

P4 继续保持完整 Realtime 不进入当前路线。Realtime 只维护已有 disabled contract、session 预留和 WebSocket stub；任何完整 WebSocket/WebRTC、realtime provider adapter 或 realtime billing 都需要单独产品决策。

## 交付物

- 一套可重复的干净环境 release candidate smoke：MySQL/Redis compose 或 staging 依赖、migration、gateway、control-api、configd、worker、health/ready、snapshot publish/watch/rollback 和基础请求链路。
- Worker 生产运营 job 补齐：balance hold reaper、reconciliation scheduled job，并与 failed settlement replay、provider task poller、callback dispatcher 形成可观测闭环。
- 至少一到两个真实 Unified Media provider adapter 映射，覆盖 image/video/audio/music 中最关键商业路径；OpenAI/Claude/Gemini 真实 channel 也要完成回归。
- Configd 生产分发链路决策和实现：Redis active snapshot key/pubsub 或明确保留 DB polling 的 fallback 机制，要求 gateway reload、stale policy 和诊断可复现。
- OpenAPI 管理面合同补齐：admin tenant/project/api-key/model/channel/route/price/limit/plugin/snapshot/emergency endpoint 与实现一致。
- 发布级 observability 和安全验收：dashboard、alert、redaction audit、provider attempt、billing backlog、snapshot stale、worker job 和 Redis latency 指标均可验证。
- Staging/灰度上线 runbook：包含环境准备、密钥注入、迁移、配置发布、真实请求、压测、故障演练、回滚、数据核对和发布记录模板。

## 核心实现顺序

1. 清理或隔离本地 Docker 数据卷，建立干净 compose/staging 依赖环境，先跑通 migration、gateway、control-api、configd 和 worker 四进程启动。
2. 用 control-api 写入 tenant、project、API key、model、channel、route、price、limit、plugin，发布 snapshot，验证 gateway watch、rollback、diagnostics 和 stale policy。
3. 补齐 worker 生产运营 job：balance hold reaper 和 reconciliation scheduled job，并加入 job metrics、lease、失败重试和 focused tests。
4. 为真实媒体供应商实现 provider-specific `ProviderTaskAdapter`，明确 submit、poll、cancel、usage、error、result URL 和 callback 字段映射。
5. 使用真实 OpenAI-compatible、Claude、Gemini 和至少一个媒体 provider channel 跑通端到端请求，覆盖同步、流式、异步、账务、限流、回调和 failed settlement replay。
6. 完成 configd 生产分发模式：优先使用 Redis durable active key + pubsub 通知，保留 DB active snapshot polling fallback；补分发失败、重启、版本回退和 checksum 验证。
7. 补齐 OpenAPI 管理面合同，并用 contract smoke 确认公开 API、管理 API、错误码和鉴权行为一致。
8. 跑完整 release gate：`go test ./...`、`make lint`、`make build`、race、Redis integration、compose smoke、load test、failure drills、backup/restore drill 和人工调账/对账校验。
9. 固化 staging rollout runbook，记录每次 RC 的 commit、migration version、snapshot version、provider channel、SLO、告警、回滚结果和已知风险。

## 关键设计约束

- P4 不新增完整 Realtime，不改变现有 URI 体系，不把供应商名暴露为 public route。
- Gateway 热路径仍然只读 runtime snapshot、Redis hot data 和内存索引，不查控制面管理表。
- Provider adapter 仍只负责协议转换、上游调用、错误映射、usage/task 状态解析，不拥有路由、策略、账务或租户决策。
- 真实 provider 验收必须使用可撤销的测试 key、低额度账号和脱敏日志；任何 provider secret 不得写入仓库、日志、metrics label 或 trace attribute。
- 发布候选以可重复验证为准，不能只依赖本机 seed snapshot 或 mock upstream。
- 任何 compose/staging 环境清理必须避免删除用户已有生产或长期共享数据卷；需要破坏性清理时单独确认。

## 验收标准

- 干净环境中 `make compose-up`、migration、gateway、control-api、configd、worker 启动成功，`/healthz`、`/readyz`、`/metrics` 均可访问。
- 通过 control-api 发布配置后，gateway 新请求使用新 snapshot；rollback 后新请求回到上一版本；configd/gateway 重启后保持最近可用 snapshot 和明确 stale policy。
- Worker 至少覆盖 failed settlement replay、provider task poller、callback dispatcher、balance hold reaper 和 reconciliation scheduled job，且 job lease、metrics 和失败重试可验证。
- 至少一个真实媒体 provider task 从 submit 到 poll succeeded、result 回写、callback、settlement 全链路通过；OpenAI/Claude/Gemini 真实 channel 有最小请求回归。
- OpenAPI 覆盖当前实际公开 API 和管理 API，导入 Apifox 或等价工具无结构错误，关键 endpoint 有示例请求/响应。
- Load test 输出 QPS、p95/p99、stream concurrency、Redis latency；failure drills 覆盖 provider、billing、Redis、DB、snapshot、worker restart。
- Backup/restore drill 在一次 disposable 环境通过，并记录 snapshot version、reconciliation result 和 failed-settlement backlog。
- `docs/tasks.md`、runbook、OpenAPI、ADR 和部署说明与当前实现一致，不声明未验证能力。

## 执行记录

| 任务 | 状态 | 验证 |
|---|---|---|
| P4/E14-T01 干净依赖环境 RC smoke | 已完成 | `tests/rc/clean_env_smoke.sh` 使用独立 Docker compose project/volume 和自动避让端口完成 MySQL、Redis、migration、gateway、control-api、configd、worker、snapshot publish/watch/rollback、gateway chat 和 metrics 验证，输出 `rc_smoke=passed` |

## 风险与处理

| 风险 | 处理 |
|---|---|
| 本地已有 Docker volume 与 compose 镜像版本不兼容 | P4 使用独立 project name 或一次性测试 volume，避免破坏用户既有 volume |
| 真实 provider 字段差异拖慢发布 | 先选择商业优先级最高的 1-2 个供应商做专属 adapter，其余保留 generic HTTP contract |
| Configd 分发从 polling 升级到 pubsub 引入一致性风险 | 使用 durable active key + pubsub 通知 + polling fallback，并保留 checksum/rollback 验证 |
| Worker 定时 job 误扣费或重复执行 | 所有 job 使用 lease、幂等 key、账务 ledger 约束和 replay 测试 |
| OpenAPI 管理面补齐后与实现漂移 | 每次 endpoint 变更同步 handler test 和 OpenAPI 示例 |
| 发布验收被 mock upstream 掩盖问题 | P4 release gate 必须包含真实 provider channel，mock 只用于单元测试和故障注入 |

## 设计来源

- [路线图](./00-roadmap.md)
- [P3 生产语义补齐与商用硬化](./14-p3-production-hardening.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
- [ADR](../design/ai_gateway_ADR.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单](../tasks.md)
