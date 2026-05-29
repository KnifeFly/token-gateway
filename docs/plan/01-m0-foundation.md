# M0 基础工程与文档归一

## 阶段目标

建立可持续开发的 Go 工程骨架，并把最终设计包、OpenAPI、ADR、阶段计划和任务看板归一到稳定路径。后续数据面、控制面、worker 和账务能力必须共享统一目录、配置、错误、日志、观测和测试入口。

## 交付物

- Go module、`cmd/gateway`、`cmd/control-api`、`cmd/worker` 和必要的 `internal/`、`pkg/` 目录。
- `configs/local.yaml`、配置加载、ENV override、Normalize 和 Validate。
- `pkg/apperr` 统一错误包。
- 结构化日志、OpenTelemetry tracing、Prometheus metrics。
- HTTP server 基础中间件和 `/healthz`、`/readyz`、`/metrics`。
- DB、Redis、migration 框架、Makefile 和 CI。
- OpenAPI 最终合同纳入工程流程。
- `docs/design/ai_gateway_ADR.md` 作为 ADR 入口。
- `docs/tasks.md` 作为唯一任务看板。

## 核心实现顺序

1. 初始化 Go module 和最小目录，保证 `go test ./...` 可运行。
2. 建立配置、错误、日志和 HTTP server 基础设施。
3. 加入 metrics、tracing、DB、Redis 和 migration 框架。
4. 补齐 Makefile 的 test、lint、race、build、run 入口。
5. 建立 CI，保证 PR 至少执行 test、race 和 lint。
6. 校验 OpenAPI 可导入并作为后续 API 变更来源。
7. 确认最终设计包、阶段计划和任务看板没有旧 v2/v3 引用。

## 关键设计约束

- 数据面默认以 Go + `net/http` 为基础，路由器可选 chi 或项目已确定的轻量方案。
- `internal/bootstrap` 只做依赖装配，不写业务逻辑。
- `domain` 包不能依赖 HTTP、SQL、Redis、provider SDK 或 metrics client。
- 不要为了完整目录树创建大量空包。

## 验收标准

- `go test ./...` 通过。
- `make lint` 通过。
- `GET /healthz` 返回 ok。
- `GET /readyz` 返回 ready 或明确依赖错误。
- `GET /metrics` 输出 Prometheus 格式。
- 目录结构符合架构设计的分层原则。
- `docs/tasks.md` 是唯一任务入口。
- OpenAPI 可导入 Apifox。
- 至少 5 条 ADR 已纳入最终 ADR 文档。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 目录过早复杂 | 只创建 M0 和 M1 确实要用的包 |
| OpenAPI 频繁变更 | URI 冻结，schema 后续版本化 |
| 配置项混乱 | 统一 Default、Load、ApplyEnv、Normalize、Validate 流程 |
| 设计文档再次分叉 | 只维护不带版本号的最终设计包 |

## 设计来源

- [实施计划 M0](../design/ai_gateway_implementation_plan.md)
- [架构设计 cmd/bootstrap/transport](../design/ai_gateway_architecture_design.md)
- [代码蓝图最小可编译目录](../design/ai_gateway_code_blueprint.md)
- [任务清单 M0](../tasks.md)
