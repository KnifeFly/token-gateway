# 商用 AI Gateway 执行任务清单

本文是最终设计包的唯一执行看板，综合了 v0.2 的细粒度商用任务和 v0.3 的架构修订。实现时只维护本文件的任务状态，不再在 `docs/design` 下维护第二份 task list。

## 当前状态

- 已完成：v2/v3 设计包归一为最终版文档；M0 Go 工程骨架、配置、错误、日志、HTTP server、metrics、DB/Redis client、migration、Makefile、compose 和 CI 已落地；M1 `/v1/chat/completions` 非流式数据面已跑通；M2 账务闭环已落地 balance account/hold、usage attempt、usage record、ledger、failed settlement replay、Redis limit 和 reconciliation；M3 已扩展 OpenAI stream/Responses/Embeddings、Claude Messages、Gemini GenerateContent/streamGenerateContent、ProviderStream/AccountingStream、SSE writer、StreamFinalizer 和 downstream disconnect 分类；M4 已落地 Unified Media async task、Task/File domain、Idempotency-Key、TaskBridge、provider task poller、callback outbox、task settlement 和 file service；M5 已落地 control API、credential encryption、snapshot build/validate/publish/watch/rollback、request pinned price、Redis revocation 和 snapshot metrics/header；M6 已落地 snapshot 驱动 plugin chain、9 个 MVP phase、内置安全/审计/指标插件和 `policy_denied` 映射；M7 已落地 metrics/tracing/redaction/load test/failure drills/dashboard/alert rules/性能预算；M8 已落地 Realtime reserved extension，未启用时稳定返回 501/`feature_not_enabled`，完整 Realtime 不进入当前路线；M9 已落地商用运营报表、对账、人工调账、模型市场配置、Agent metadata report、OpenAPI 管理接口和 backup/restore runbook；P0 已补齐 worker、异步任务、公开 API、snapshot stale policy、emergency disable 和 focused tests；P1 已补齐多维 limit rule runtime index、Redis Lua 多维限流、local deny cache、routing strategy registry、RouteSignals、显式 policy decision stage、model catalog/schema/alias/provider mapping 和 focused tests；P2 已补齐独立 configd、snapshot publish/rollback/diagnostics、IP allowlist、Model ACL、RouteOverride、Callback、CostGuard decision、classifier registry hint/body schema inference、`ambiguous_protocol` 测试和 Realtime disabled contract 边界；P3 已补齐 Redis token bucket/TPM 预扣、统一 billability policy、Native OpenAI images/audio adapter、Unified Media provider adapter contract、Redis RouteSignals、configd 分发 smoke 和生产验收文档；P4 已补齐干净依赖环境 RC smoke、worker 运营 job、真实 provider release channel、configd Redis active snapshot 分发、OpenAPI 管理面合同、发布级观测安全 release gate 和 staging 灰度上线 runbook；P5 已补齐 OpenAI/Claude/Gemini 协议兼容矩阵、SDK-compatible HTTP wire shape、stream 生命周期、tool/multimodal passthrough、usage/error normalization、contract tests 和 OpenAPI/runbook 同步；P6 已补齐 provider/channel 健康信号、熔断、retry budget、fallback 限制、provider attempt 追踪和 failure drills；P7 已补齐非存储媒体输入资产语义、media provider result asset contract、Replicate fixture 映射、callback/settlement result URL 衔接和媒体 contract tests；P8 已补齐 Portal 模型/schema、credits、usage、API key 自助管理、task 查询、权限边界测试、OpenAPI contract 和 runbook；P9 已补齐客户接入验收收口、Portal smoke CLI、RC smoke 集成、OpenAPI import preflight 和 customer acceptance runbook；P10 已补齐发布交接收口、release handoff CLI、PR 模板、发布证据字段、回滚说明和 runbook；P11 已补齐模型分类、组件化客户售价、provider cost、统一报价器、模型目录字段、渠道模型 metadata 和同步 preview；P12 已收敛 review P0 正确性，覆盖 stream lease close ownership、async idempotency replay hold release、terminal submit settlement 复用、zero-price/no-hold settlement 和 focused regression tests；P13 已收敛 review P1 商业账务与安全边界，覆盖 async price snapshot、budget admission 语义、URL/base64 transient asset 边界、egressguard、callback 签名/重试/dead-letter、classifier 顺序和 async submit fallback；P14 已收敛 review P2 工程 readiness，覆盖 API key HMAC hash、control plane 静态 token 安全基线、stream close timeout、SSE event-boundary usage parser、nil-safe metrics、trusted proxy client IP 和 README/runbook；P15 已收敛 follow-up review 生产阻塞，覆盖 internal/client request ID 分离、stream lease renewal、task-aware hold reaper、provider webhook 不直达 customer callback URL、egress fail-closed 和 focused regression tests；P16 已收敛 follow-up review 账务审计一致性，覆盖 async idempotency duplicate replay、async submit attempt durable audit、sync final failed attempt durable update、failed settlement row claim 和 daily admission budget 语义；P17 已收敛 follow-up review worker/callback 稳定性，覆盖 worker lease heartbeat、job MaxConcurrency、provider poller 单任务错误隔离、callback durable claim、delivery 观测和 failure drills；P18 已收敛 follow-up review 文件资产边界，覆盖 transient metadata registry、expired quota、expired metadata cleanup、URL/base64 边界、stream disabled contract 和客户文档；已补齐 Portal/Admin Console monorepo 设计、CLAUDE 指引、roadmap、P19-P22 详细计划文档、P23 结构治理和 P24 NewAPI Lean Console Parity 规划文档。
- P19 进展：Console monorepo foundation 已落地，覆盖 `cmd/console`、console route skeleton、OpenAPI split、frontend workspace、generated API client 入口、Makefile/CI 和本地开发 runbook。
- P20 进展：Portal Web BFF 已落地，覆盖 `internal/app/portal` session/use case wrapper、`/api/portal/v1/*` browser BFF、HttpOnly session + CSRF、dashboard/onboarding 聚合、Portal UI、OpenAPI/generated client、focused Go tests 和 `tools/portal-web-smoke`。
- P21 进展：Admin Web BFF 已落地，覆盖 `internal/app/admin` operator/session/RBAC/audit、`/api/admin/v1/*` browser BFF、CSRF/idempotency/reason guard、owner-service config writes、snapshot/operations read models、migration、OpenAPI/generated client 和 focused Go tests。
- P11 进展：模型价格目录增强已落地，覆盖 `internal/domain/pricing` category/template/component quoter、control-plane price/model/channel metadata、runtime snapshot/index、admission/settlement/async task customer quote、provider cost components、OpenAPI 和 focused tests。
- P23 进展：Console directory structure alignment 已收口，覆盖 `internal/controlplane/configadmin` rename、`internal/portal` 全量迁移到 `internal/app/portal`、Admin/Portal service/repository/transport 拆分、configadmin repository 聚合拆分、API scripts/examples、import boundary check、Portal/Admin frontend app 拆分、shared frontend package 拆分和最终全量验收。
- P24 进展：NewAPI Lean Console Parity 设计与计划已落地，已完成 Admin/Portal P24 信息架构、route plan、中文展示文案入口和 i18n 预留；已完成 Channel Management lean workflows，覆盖 channel safe CRUD、credential rotation、test、sync preview/apply、health read model、model coverage、route policy hint、audit 和 Admin 中文渠道面板；只补齐渠道、模型、用户/客户账户、令牌、日志、任务、操练场和最小额度运营，用户分组、模型分组、渠道分组、倍率、订阅、兑换码、支付、邀请返利、模型部署和复杂系统设置不进入 P24。
- 待执行：P22 Admin frontend、Console E2E smoke、静态资产/security headers、发布/回滚 runbook 和 console 生产化仍待执行；P24 除 T01-T03 外的产品实现仍待执行。复杂财务/发票闭环、对象存储、完整 Realtime、生产级 Observability 扩展、WASM/动态插件、semantic routing/cache 和多地域 active-active 先不做。
- 阻塞：无。
- 下一步建议：如目标是完整 console 生产发布，进入 P22 Console Frontend Production；如目标是继续 P24 精简运营产品面，下一步进入 `P24/E34-T04/P0` Model Management 和 Portal 模型预览。对象存储能力仍需另立独立阶段。
- 本次规划验证：2026-06-02 P24 NewAPI Lean Console Parity 设计、计划、roadmap、tasks、CLAUDE 和 design index 落库后，`git diff --check` 与 markdown trailing-whitespace scan 通过；本次为 docs-only 变更，未运行代码测试。2026-06-02 P23 Console Directory Structure Alignment 计划文档落库并接入 roadmap/tasks/design/CLAUDE 后，`git diff --check` 与 markdown trailing-whitespace scan 通过；本次为 docs-only 变更，未运行代码测试。2026-06-02 P23 Portal 迁移目标调整为全量迁移并删除 `internal/portal` 运行包后，`git diff --check` 与 markdown trailing-whitespace scan 通过；本次为 docs-only 变更，未运行代码测试。2026-06-01 Portal/Admin Console 设计与 P19-P22 计划文档落库后，`git diff --check` 与 markdown trailing-whitespace scan 通过；当次为 docs-only 变更，未运行代码测试。
- 本次开发验证：2026-06-02 P24 Admin/Portal route plan、中文展示文案和 i18n 预留 `pnpm typecheck`、`pnpm build`、`pnpm test`、`pnpm lint`、`git diff --check` 和 docs/frontend trailing-whitespace scan 通过；Browser Admin route plan smoke、Portal 本地开发密钥登录后导航 smoke、Portal 操练场入口 smoke 通过，未登录 `/auth/me` 401 为当前会话恢复路径的预期未认证响应。
- 本次开发验证：2026-06-02 P24 Channel Management lean workflows `go test ./internal/app/admin/... ./internal/transport/adminhttp ./internal/controlplane/configadmin ./tests/contract`、`pnpm typecheck`、`pnpm build`、`pnpm test`、`pnpm lint`、`pnpm generate:api`、`api/scripts/lint_openapi.sh` 和 cut-scope scan 通过；提交后需重跑 `make api-check` 确认 generated client 无 drift。
- 本次开发验证：2026-06-02 P23 后端结构治理核心项 `go test ./...`、`go vet ./...`、`go build ./cmd/...`、`make boundary-check`、`make api-check`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build`、`git diff --check` 和 docs/API trailing-whitespace scan 通过；focused slices `go test ./internal/controlplane/configadmin ./internal/controlplane/snapshot ./internal/transport/controlhttp ./internal/app/admin/... ./internal/transport/adminhttp ./internal/bootstrap` 与 `go test ./internal/app/portal/... ./internal/transport/portalhttp ./internal/transport/portalwebhttp` 通过。
- 本次开发验证：2026-06-02 P23 frontend app/shared package 拆分与最终验收 `go test ./internal/controlplane/configadmin ./internal/app/admin/... ./internal/app/portal/... ./internal/transport/adminhttp ./internal/transport/portalhttp ./internal/transport/portalwebhttp`、`go test ./...`、`go vet ./...`、`go build ./cmd/...`、`pnpm generate:api`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build`、`make boundary-check`、`make api-check`、`go run ./tools/portal-web-smoke -console-url http://127.0.0.1:9505 -api-key tg-local-dev-key -create-derived-key` 和 Browser Portal/Admin desktop/mobile smoke 通过；console smoke 使用本地 seed snapshot，DB/Redis/billing/limits 关闭。
- 本次开发验证：2026-06-01 P19 `go test ./...`、`go vet ./...`、`go build ./cmd/...`、`pnpm install --frozen-lockfile`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build`、`make api-check`、`git diff --check` 通过；`cmd/console` 在关闭本地 DB/Redis 依赖后完成 health/ready/BFF skeleton smoke，Portal/Admin Vite 页面经 Playwright 打开并截图检查。
- 本次开发验证：2026-06-01 P20 `go test ./...`、`go vet ./...`、`go build ./cmd/...`、`pnpm generate:api`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build`、`git diff --check` 通过；`cmd/console` 在关闭本地 DB/Redis/Billing/Limit 依赖后用 `tg-local-dev-key` 跑通 `go run ./tools/portal-web-smoke -console-url http://127.0.0.1:9505 -api-key tg-local-dev-key -create-derived-key`；Portal 生产构建页面经 Browser 验证 desktop/mobile 登录 dashboard 渲染且无 console error。
- 本次开发验证：2026-06-01 P11/P12 `go test ./...`、`go vet ./...`、`go build ./cmd/...`、`pnpm generate:api`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build`、`make api-check`、`git diff --check` 通过；focused slice `go test ./internal/domain/pricing ./internal/controlplane/admin ./internal/controlplane/snapshot ./internal/dataplane/admission ./internal/dataplane/snapshot ./internal/billing ./internal/billing/reporting ./internal/task ./internal/portal ./internal/transport/controlhttp` 通过；P12 stream/async/zero-price correctness 在全量 Go 测试中复验通过。
- 本次开发验证：2026-06-01 P21 `go test ./...`、`go vet ./...`、`go build ./cmd/...`、`pnpm generate:api`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build`、`git diff --check` 通过；focused slice `go test ./internal/app/admin/... ./internal/transport/adminhttp ./internal/transport/consolehttp ./internal/controlplane/admin ./internal/transport/controlhttp` 通过。
- 最近验证：2026-05-31 P18 `go test ./...`、`go vet ./...`、`git diff --check`、`tests/failure/file_asset_boundary_drills.sh`、`bash -n tests/failure/file_asset_boundary_drills.sh`、`bash -n tests/failure/release_gate.sh` 和 Grafana dashboard JSON parse 通过；focused slice `go test ./internal/task ./internal/dataplane/parser ./internal/worker/jobs ./internal/bootstrap ./tests/contract` 通过。2026-05-31 P17 `go test ./...`、`go vet ./...`、`git diff --check`、`tests/failure/worker_callback_drills.sh`、`bash -n tests/failure/worker_callback_drills.sh`、`bash -n tests/failure/release_gate.sh` 和 Grafana dashboard JSON parse 通过；focused slice `go test ./internal/worker ./internal/worker/jobs ./internal/task ./internal/bootstrap` 通过。2026-05-31 P16 `go test ./...`、`go vet ./...` 和 `git diff --check` 通过；focused slice `go test ./internal/task ./internal/dataplane/dispatch ./internal/billing ./internal/billing/reporting ./internal/bootstrap ./internal/dataplane/limit ./internal/controlplane/admin ./internal/transport/controlhttp` 通过。2026-05-31 P15 `go test ./...`、`go vet ./...` 和 `git diff --check` 通过；focused slice `go test ./internal/dataplane/engine ./internal/transport/httpserver ./internal/transport/realtimehttp`、`go test ./internal/dataplane/stream ./internal/dataplane/limit`、`go test ./internal/billing ./internal/provider/replicate ./internal/task ./internal/bootstrap` 通过；新增 Redis/MySQL integration 测试分别在 `TOKEN_GATEWAY_REDIS_ADDR`、`TOKEN_GATEWAY_MYSQL_DSN` 未设置时按现有约定跳过。2026-05-31 P14 `go test ./...`、`go vet ./...`、`git diff --check` 和 README/docs trailing-whitespace scan 通过；focused slice `go test ./internal/dataplane/auth ./internal/controlplane/admin ./internal/bootstrap ./internal/transport/controlhttp ./internal/transport/configdhttp ./internal/dataplane/stream ./internal/provider/openai ./internal/transport/httpserver ./internal/task ./internal/billing` 通过。2026-05-31 P13 `go test ./...`、`go vet ./...`、`git diff --check` 通过；focused slice `go test ./pkg/egressguard`、`go test ./internal/task`、`go test ./internal/worker/jobs`、`go test ./internal/dataplane/classifier ./internal/dataplane/parser ./internal/dataplane/limit` 通过。2026-05-31 P12 `go test ./...`、`go vet ./...` 和 `git diff --check` 通过；focused slice `go test ./internal/dataplane/engine ./internal/dataplane/stream ./internal/task ./internal/billing ./internal/dataplane/limit ./internal/worker/jobs` 通过；新增 Redis/MySQL integration 测试分别在 `TOKEN_GATEWAY_REDIS_ADDR`、`TOKEN_GATEWAY_MYSQL_DSN` 未设置时按现有约定跳过。2026-05-31 review remediation 计划文档落库后，`git diff --check` 与 markdown trailing-whitespace scan 通过；本次为 docs-only 变更，未运行代码测试。2026-05-30 P11 计划文档落库后，`git diff --check` 与 markdown trailing-whitespace scan 通过；本次为 docs-only 变更，未运行代码测试；2026-05-30 review hardening 修复 Portal API key 快照刷新、异步任务终态结算、控制面 `enabled=false`、admin token fail-closed 和 fallback per-candidate 限流后，`go test ./...` 与 `go vet ./...` 通过；2026-05-30 P10 `go run ./tools/release-handoff -run-checks -output /tmp/token-gateway-p10-release-handoff.md` 通过，覆盖 `go test ./...`、`go vet ./...`、`go build ./cmd/...`、`go test ./tools/portal-smoke ./tools/release-handoff ./tests/contract`、`bash -n tests/rc/clean_env_smoke.sh` 和 `tests/failure/release_gate.sh`；`git diff --check` 通过；此前 P8 `go test ./internal/portal ./internal/transport/portalhttp ./internal/task ./internal/bootstrap` 通过；此前 P7 `go test ./internal/dataplane/parser ./internal/task ./internal/provider/replicate ./internal/worker/jobs ./tests/contract` 通过；此前 `go test ./internal/dataplane/observe ./internal/dataplane/router ./internal/dataplane/dispatch ./internal/billing ./internal/infra/redis` 通过；`bash -n tests/failure/provider_reliability_drills.sh` 和 `bash -n tests/failure/release_gate.sh` 通过；非 live `tests/failure/release_gate.sh` 通过；此前 `bash tests/rc/clean_env_smoke.sh` 使用独立 Docker compose project/volume 和自动避让端口跑通 MySQL、Redis、migration、gateway、control-api、configd、worker、health/ready、Redis active snapshot key、snapshot publish/watch/rollback、gateway chat 和 metrics，输出 `rc_smoke=passed`；`make lint`、`make build`、Redis 集成、failure drills 和 load test 均通过。

## 使用规则

- M0-M9 任务 ID 采用 `M{milestone}/E{epic}-T{number}/P{priority}` 格式。
- P0-P4 设计差距补齐、商用硬化和发布验收任务 ID 采用 `P{phase}/E{epic}-T{number}/P{priority}` 格式。
- P5-P11 剩余产品能力、验收、发布交接和模型价格目录增强任务 ID 采用 `P{phase}/E{epic}-T{number}/P{priority}` 格式，执行顺序固定为 P5、P6、P7、P8、P9、P10、P11。
- P12-P14 review remediation 任务 ID 采用 `P{phase}/E{epic}-T{number}/P{priority}` 格式，来源于 2026-05-31 静态 review 的 P0/P1/P2 风险分层，执行顺序固定为 P12、P13、P14。
- P15-P18 follow-up review remediation 任务 ID 采用 `P{phase}/E{epic}-T{number}/P{priority}` 格式，来源于 P12-P14 后续静态 review 暴露的生产生命周期、身份边界、账务审计、worker/callback 和文件资产边界问题，执行顺序固定为 P15、P16、P17、P18。
- P19-P24 console full-stack、结构治理与 NewAPI 精简对标任务 ID 采用 `P{phase}/E{epic}-T{number}/P{priority}` 格式，P19-P22 是产品/生产化阶段，P23 是行为保持型目录结构对齐阶段，P24 是 NewAPI lean parity 产品补齐阶段。
- 第一轮优先完成 M0-M3 的 P0 任务，形成最小商用内核。
- M4-M9 只拆到可执行粒度，避免早期过度展开控制面、插件和运营后台。
- P0-P4 是 M0-M9 之后的设计差距补齐、商用硬化和发布候选验收阶段，执行顺序固定为 P0、P1、P2、P3、P4。
- P16-P18 未验收前，暂停继续新增 P11 这类功能扩展，优先保证账务审计、worker/callback、文件资产边界和 outbound 安全不变量成立。
- P19-P22 只在 P15-P18 可靠性收敛完成后进入；它们新增完整 Portal/Admin 前后端，但不允许改变 `/v1/*`、`/v1/portal/*` 和 `/admin/*` 的既有职责。
- Browser Portal/Admin 固定走 `/api/portal/v1/*` 和 `/api/admin/v1/*`；Admin UI 不直接调用 `/admin/*`，也不使用 control admin token 作为 browser auth。
- Portal/Admin 后端新增能力按 `internal/app/portal` 和 `internal/app/admin` 拆分，service/repository 继续按 use case/read model 拆文件，不新增全局 service/repository 目录；control/config owner 当前为 `internal/controlplane/configadmin`。
- P23 只做结构治理和模块拆分，不新增产品能力、不改变 API 行为、不移动 core domain ownership；`configadmin` rename 是去歧义的 rename-only 调整，不代表把 control owner 合并进 `app/admin`；如果 P22 实施前单文件继续膨胀，可以先执行 P23 的拆分子集。
- P24 只做 NewAPI 精简对标：渠道、模型、用户/客户账户、令牌、使用日志、任务日志、操练场和最小额度运营进入范围；用户/模型/渠道分组、倍率、订阅、兑换码、支付、邀请返利、模型部署和复杂系统设置不进入范围。
- 完整 Realtime 不进入当前路线；M8/P2 只维护 disabled contract、session 预留和 WebSocket stub。
- 文件能力按非存储输入资产处理；gateway 不做对象存储，不承诺媒体对象持久化、下载、生命周期或存储 SLA。
- 每个任务完成时必须按影响范围更新代码、测试、迁移、OpenAPI、ADR、metrics、trace、日志、审计、配置、计划和故障说明。

## M0 基础工程与文档归一

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M0/E0-T01/P0 | P0 | 初始化 Go module 和最小目录 | repo root, `cmd/`, `internal/`, `pkg/` | `go test ./...` 可运行 |
| [x] | M0/E0-T02/P0 | P0 | 建立 Makefile | `Makefile` | test、lint、race、build、run 目标可用 |
| [x] | M0/E0-T03/P0 | P0 | 建立配置加载 | `internal/bootstrap/config.go` | default、yaml、env、normalize、validate 支持 |
| [x] | M0/E0-T04/P0 | P0 | 建立统一错误包 | `pkg/apperr` | code、status、type、retryable、safe 可表达 |
| [x] | M0/E0-T05/P0 | P0 | 建立结构化日志 | `internal/infra/log` | request_id、trace_id 可输出 |
| [x] | M0/E0-T06/P0 | P0 | 建立 HTTP server 基础 | `internal/transport/httpserver` | `/healthz`、`/readyz`、`/metrics` 可访问 |
| [x] | M0/E0-T07/P0 | P0 | 初始化 Prometheus 和 OTel | `internal/infra/telemetry` | metrics 和 trace exporter 可配置 |
| [x] | M0/E0-T08/P0 | P0 | 初始化 DB、Redis 和 migration | `internal/infra/db`, `internal/infra/redis`, `migrations/` | ping、close、migrate up/down 可用 |
| [x] | M0/E0-T09/P1 | P1 | 建立 CI | `.github/workflows` | PR 触发 test、vet/lint、race |
| [x] | M0/E0-T10/P1 | P1 | 固化文档入口 | `docs/design`, `docs/plan`, `docs/tasks.md` | 无旧 v2/v3 设计入口，OpenAPI 可导入 |

## M1 最小非流式数据面

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M1/E1-T01/P0 | P0 | 定义 RequestState | `internal/dataplane/engine/state.go` | 覆盖 protocol、principal、snapshot、route、billing、observe |
| [x] | M1/E1-T02/P0 | P0 | 实现 APIClassifier | `internal/dataplane/classifier` | OpenAI chat 可分类，冲突路径有测试骨架 |
| [x] | M1/E1-T03/P0 | P0 | 实现 BodyStore 和 OpenAI Chat Parser | `internal/dataplane/parser` | model、messages、stream flag、usage estimate 可解析 |
| [x] | M1/E1-T04/P0 | P0 | 实现 API key extractor 和 hash 校验 | `internal/dataplane/auth` | Bearer/x-api-key 支持，明文 key 不落日志 |
| [x] | M1/E1-T05/P0 | P0 | 定义最小 RuntimeSnapshot 和 IndexedSnapshot | `internal/controlplane/snapshot`, `internal/dataplane/snapshot` | api key、model、channel、route 索引可用 |
| [x] | M1/E1-T06/P0 | P0 | 实现 RoutePlanner 和 priority selector | `internal/dataplane/router` | 可选中 mock channel，无路由返回标准错误 |
| [x] | M1/E1-T07/P0 | P0 | 实现 Provider relay types 和 registry | `internal/provider/relay`, `internal/provider` | adapter 能注册和按 capability 获取 |
| [x] | M1/E1-T08/P0 | P0 | 实现 OpenAI-compatible adapter | `internal/provider/openai` | 非流式 chat 可调用 mock/真实上游 |
| [x] | M1/E1-T09/P0 | P0 | 实现 ProviderDispatcher 和 attempt 记录 | `internal/dataplane/dispatch` | provider 5xx、429、401 分类正确 |
| [x] | M1/E1-T10/P0 | P0 | 实现 GatewayEngine.Handle 非流式主链路 | `internal/dataplane/engine` | curl `/v1/chat/completions` 成功 |
| [x] | M1/E1-T11/P1 | P1 | 接入基础观测 | `internal/dataplane/observe` | access log、provider attempt metrics、route span 可见 |

## M2 账务闭环

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M2/E2-T01/P0 | P0 | 建立账务表 migration | `migrations/` | balances、holds、attempts、records、ledger、failed_settlements 可迁移 |
| [x] | M2/E2-T02/P0 | P0 | 实现金额和价格类型 | `pkg/money`, `internal/domain/pricing` | 金额使用 micros，不用 float |
| [x] | M2/E2-T03/P0 | P0 | 实现 PriceQuoter / PriceEstimator | `internal/dataplane/admission` | input/output token 可报价 |
| [x] | M2/E2-T04/P0 | P0 | 实现 Balance service 和 hold | `internal/billing` | 余额不足不调用 provider |
| [x] | M2/E2-T05/P0 | P0 | 实现 AdmissionController.Reserve | `internal/dataplane/admission` | request_id 幂等创建 hold |
| [x] | M2/E2-T06/P0 | P0 | 实现 usage attempt writer | `internal/billing` | 每次 provider attempt 有记录 |
| [x] | M2/E2-T07/P0 | P0 | 实现 Settlement planner/executor | `internal/billing` | provider 成功后扣费并 release hold |
| [x] | M2/E2-T08/P0 | P0 | 实现 Ledger service | `internal/billing` | ledger entry 幂等且可对账 |
| [x] | M2/E2-T09/P0 | P0 | 实现 failed settlement replay worker | `internal/worker/jobs` | 结算失败可重放且不重复扣费 |
| [x] | M2/E2-T10/P1 | P1 | 实现 Redis token bucket 和 concurrency lease | `internal/dataplane/limit` | 多副本 QPS/TPM/concurrency 准确 |
| [x] | M2/E2-T11/P1 | P1 | 实现初版 reconciliation query | `internal/billing/reconciliation.go` | 可发现 ledger 与 balance 差异 |

## M3 Streaming + Native Compatible

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M3/E3-T01/P0 | P0 | 实现 SSE writer | `internal/transport/httpserver/sse_writer.go` | OpenAI stream 可输出 |
| [x] | M3/E3-T02/P0 | P0 | 定义 ProviderStream / AccountingStream | `internal/provider/relay`, `internal/dataplane/stream` | close-time settlement 入口唯一 |
| [x] | M3/E3-T03/P0 | P0 | 实现 StreamFinalizer | `internal/dataplane/stream/finalizer.go` | stream close 后完成 settlement |
| [x] | M3/E3-T04/P0 | P0 | 分类 client disconnect | `internal/dataplane/stream` | 不误罚 provider health |
| [x] | M3/E3-T05/P0 | P0 | 实现 Claude Messages parser/adapter | `internal/provider/claude` | `/v1/messages` 可用 |
| [x] | M3/E3-T06/P0 | P0 | 实现 Gemini GenerateContent parser/adapter | `internal/provider/gemini` | `/v1beta/...` 可用 |
| [x] | M3/E3-T07/P1 | P1 | 实现 OpenAI Responses / Embeddings | `internal/provider/openai` | OpenAI SDK 可调用 |
| [x] | M3/E3-T08/P1 | P1 | 补齐协议消歧测试 | `internal/dataplane/classifier`, `internal/dataplane/engine` | `X-Gateway-Protocol`、model registry、body schema 和 `ambiguous_protocol` 覆盖 |

## M4 Unified Media Async Task

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M4/E4-T01/P0 | P0 | 实现 Task domain 和状态机 | `internal/task` | queued、running、succeeded、failed、canceled 有效 |
| [x] | M4/E4-T02/P0 | P0 | 实现 IdempotencyStore | `internal/task/idempotency.go` | 同 key 同 body 返回同 task，同 key 不同 body 返回 409 |
| [x] | M4/E4-T03/P0 | P0 | 实现 Unified media parser | `internal/dataplane/parser/unified_media_parser.go` | image、video、audio、music 和 `model_params` 可解析 |
| [x] | M4/E4-T04/P0 | P0 | 实现非存储 input asset service | `internal/task/file_service.go` | base64、url、stream 输入可用于请求归一化和 provider 转发，不承诺对象存储 |
| [x] | M4/E4-T05/P0 | P0 | 实现 TaskBridge | `internal/task/bridge.go`, `internal/dataplane/engine` | 创建 internal task 并返回 task object |
| [x] | M4/E4-T06/P0 | P0 | 实现 ProviderTaskDispatcher | `internal/task/provider_dispatcher.go` | external_task_id 落库 |
| [x] | M4/E4-T07/P0 | P0 | 实现 provider task poller | `internal/worker/jobs/provider_task_poller.go` | task 状态推进 |
| [x] | M4/E4-T08/P0 | P0 | 实现 callback outbox/dispatcher | `internal/worker/jobs/callback_dispatcher.go` | callback 失败可重试 |
| [x] | M4/E4-T09/P1 | P1 | 实现 task settlement | `internal/task/settlement.go` | 任务成功后最终扣费 |

## M5 Control Plane + Snapshot

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M5/E5-T01/P0 | P0 | 实现 admin auth | `internal/transport/controlhttp` | admin token 或 RBAC 可用 |
| [x] | M5/E5-T02/P0 | P0 | 实现 tenant/project/api key CRUD | `internal/controlplane/admin` | key create、disable、list 可用 |
| [x] | M5/E5-T03/P0 | P0 | 实现 model/schema/alias CRUD | `internal/controlplane/admin` | 新增模型无需重启 gateway |
| [x] | M5/E5-T04/P0 | P0 | 实现 provider/channel/credential CRUD | `internal/controlplane/admin` | credential 加密，不进 snapshot 明文 |
| [x] | M5/E5-T05/P0 | P0 | 实现 route/price/limit CRUD | `internal/controlplane/admin` | route、price、limit 可配置 |
| [x] | M5/E5-T06/P0 | P0 | 实现 snapshot builder/validator | `internal/controlplane/snapshot` | 坏配置拒绝发布 |
| [x] | M5/E5-T07/P0 | P0 | 实现 snapshot publisher/watcher | `internal/controlplane/snapshot`, `internal/dataplane/snapshot` | gateway 原子切换 snapshot |
| [x] | M5/E5-T08/P0 | P0 | 实现 request-level pinning | `internal/dataplane/engine` | 已开始请求 pin 住 snapshot、price、route |
| [x] | M5/E5-T09/P0 | P0 | 实现 API key revocation blacklist | `internal/infra/redis/revocation.go` | revoke 在目标 SLA 内生效 |
| [x] | M5/E5-T10/P1 | P1 | 实现 rollback 和 staleness metrics | `internal/controlplane/snapshot` | snapshot version/staleness 可观测 |

## M6 Plugins + Security

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M6/E6-T01/P0 | P0 | 定义 MVP 9 phase enum | `internal/dataplane/plugin/phase.go` | phase 与 ADR 一致 |
| [x] | M6/E6-T02/P0 | P0 | 实现 PluginManager | `internal/dataplane/plugin/manager.go` | 无绑定 phase O(1) skip |
| [x] | M6/E6-T03/P0 | P0 | 实现 PluginBinding resolver | `internal/dataplane/plugin/binding_resolver.go` | scope specificity、priority、name 排序正确 |
| [x] | M6/E6-T04/P0 | P0 | 实现 RequestSizePlugin | `plugin/builtin/request_size.go` | 超限拒绝 |
| [x] | M6/E6-T05/P0 | P0 | 实现 PromptTokenLimitPlugin | `plugin/builtin/prompt_token_limit.go` | prompt token 超限拒绝 |
| [x] | M6/E6-T06/P0 | P0 | 实现 PIIRedactionPlugin | `plugin/builtin/pii_redaction.go` | 日志和审计脱敏 |
| [x] | M6/E6-T07/P0 | P0 | 实现 PromptGuardPlugin | `plugin/builtin/prompt_guard.go` | 命中返回 `policy_denied` |
| [x] | M6/E6-T08/P1 | P1 | 实现 ResponseGuardPlugin / CostGuardPlugin | `plugin/builtin` | 支持 deny、degrade、audit |
| [x] | M6/E6-T09/P1 | P1 | 实现 AuditLogPlugin / LLMMetricPlugin | `plugin/builtin` | 审计和指标不含敏感明文 |

## M7 Observability + Performance

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M7/E7-T01/P0 | P0 | 固化 metrics 命名和 label 规范 | `internal/infra/telemetry/metrics.go` | provider、billing、task、snapshot 指标齐全 |
| [x] | M7/E7-T02/P0 | P0 | 补齐 OpenTelemetry spans | `internal/dataplane/observe` | 每个关键阶段 span 可见 |
| [x] | M7/E7-T03/P0 | P0 | 实现统一 redactor | `pkg/redaction` | api key、provider key、prompt、response 脱敏 |
| [x] | M7/E7-T04/P0 | P0 | 编写 load test | `tools/loadtest` | QPS、stream concurrency、Redis 延迟报告 |
| [x] | M7/E7-T05/P0 | P0 | 编写 failure drills | `tests/failure` | provider、billing、redis、db、snapshot 场景 |
| [x] | M7/E7-T06/P1 | P1 | 建立 dashboard 和 alert rules | `deployments/observability` | failed settlement、snapshot stale、provider 429/5xx 有告警 |

## M8 Realtime Reserved Extension

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M8/E8-T01/P2 | P2 | 定义 RealtimeSession domain | `internal/dataplane/realtime/session.go` | session 类型、状态、过期时间可表达 |
| [x] | M8/E8-T02/P2 | P2 | 实现 create/get session API | `internal/transport/realtimehttp` | 未启用时返回 501/feature_not_enabled |
| [x] | M8/E8-T03/P2 | P2 | 定义 RealtimeEngine interface | `internal/dataplane/realtime/engine.go` | 不绑定具体 provider |
| [x] | M8/E8-T04/P2 | P2 | 实现 WebSocket handler stub | `internal/transport/realtimehttp` | 可编译，有鉴权、审计、metrics 接入点 |

## M9 Commercial Operations

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | M9/E9-T01/P1 | P1 | 实现客户余额和用量报表 | `internal/controlplane/admin` | 客户可查余额、用量和扣费流水 |
| [x] | M9/E9-T02/P1 | P1 | 实现 provider cost 和利润报表 | `internal/billing/reporting` | 运营可查渠道成本和模型利润 |
| [x] | M9/E9-T03/P1 | P1 | 实现 reconciliation report | `internal/billing/reconciliation.go` | 每日对账能发现差异 |
| [x] | M9/E9-T04/P1 | P1 | 实现 manual adjustment | `internal/billing` | 人工调账幂等且有强审计 |
| [x] | M9/E9-T05/P2 | P2 | 实现模型市场配置 | control API | 租户可见模型可配置 |
| [x] | M9/E9-T06/P2 | P2 | 实现 Agent metadata 报表 | reporting 或 analytics | workflow、scene、shot 维度可分析 |
| [x] | M9/E9-T07/P2 | P2 | 建立 backup/restore runbook | `docs/runbook` | 恢复演练通过 |

## P0 Production Closure

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P0/E10-T01/P0 | P0 | 实现可运行 worker 进程入口 | `cmd/worker`, `internal/bootstrap` | worker 可启动、优雅退出，并接入 config、DB、Redis、logger、metrics、tracing |
| [x] | P0/E10-T02/P0 | P0 | 实现通用 job runner | `internal/worker` | 支持 lease、并发控制、防重复执行、retry/backoff、panic recovery 和 shutdown |
| [x] | P0/E10-T03/P0 | P0 | 接入 provider task polling | `internal/worker/jobs`, `internal/task` | 多 worker 下同一 provider task 不重复推进，状态可从 queued/running 到终态 |
| [x] | P0/E10-T04/P0 | P0 | 接入 failed settlement replay | `internal/worker/jobs`, `internal/billing` | 失败结算可自动重放，且 replay 幂等、不重复扣费 |
| [x] | P0/E10-T05/P0 | P0 | 接入 callback dispatcher | `internal/worker/jobs`, `internal/task` | callback outbox 可重试，失败原因和最终状态可追踪 |
| [x] | P0/E10-T06/P0 | P0 | 实现真实异步 provider task adapter | `internal/provider`, `internal/task` | submit、poll、cancel 可调用真实 provider；mock 仅用于测试 |
| [x] | P0/E10-T07/P0 | P0 | 打通 async media task 生产闭环 | `internal/dataplane/engine`, `internal/task`, `internal/billing` | task 从 API 创建到 provider 轮询、结果回写、callback、settlement 全链路通过 |
| [x] | P0/E10-T08/P0 | P0 | 补齐 `/v1/models` 和模型 schema API | `internal/transport/httpserver`, model catalog 读模型 | `/v1/models`、`/v1/models/{model}/schema` 与 OpenAPI 对齐并有 HTTP 测试 |
| [x] | P0/E10-T09/P0 | P0 | 补齐 `/v1/credits` API | `internal/transport/httpserver`, `internal/billing` | 客户可查询余额/credits，响应不泄露内部账务实现 |
| [x] | P0/E10-T10/P0 | P0 | 补齐 `/v1/moderations` API | `internal/transport/httpserver`, `internal/dataplane` | moderation API 与 OpenAPI 对齐，鉴权、审计、错误映射可用 |
| [x] | P0/E10-T11/P0 | P0 | 实现 snapshot stale policy | `internal/dataplane/snapshot`, `internal/dataplane/engine` | soft stale 告警，hard stale fail-close 或按明确策略拒绝请求 |
| [x] | P0/E10-T12/P0 | P0 | 实现 emergency provider/channel disable | `internal/infra/redis`, `internal/dataplane/router`, `internal/dataplane/dispatch` | 禁用后无需重启 gateway，新请求不再选中对应 provider/channel |
| [x] | P0/E10-T13/P1 | P1 | 补齐 P0 进程级和集成测试 | `tests/`, focused package tests | worker、async task、公开 API、snapshot stale、emergency disable 测试通过 |

## P1 Design Capabilities

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P1/E11-T01/P0 | P0 | 设计并实现 limit rule 运行时索引 | `internal/controlplane/snapshot`, `internal/dataplane/limit` | tenant、project、api key、model、provider、channel 六个维度可组合读取 |
| [x] | P1/E11-T02/P0 | P0 | 实现 Redis Lua 多维限流 | `internal/dataplane/limit`, `internal/infra/redis` | RPM/QPS、TPM、concurrency、daily budget、cost-per-minute 原子判断 |
| [x] | P1/E11-T03/P0 | P0 | 实现 local deny cache | `internal/dataplane/limit` | 拒绝结果可短期缓存，命中时不访问 Redis，过期后恢复 Redis 判断 |
| [x] | P1/E11-T04/P0 | P0 | 抽象 routing strategy registry | `internal/dataplane/router` | priority/weighted 作为默认策略保留兼容 |
| [x] | P1/E11-T05/P0 | P0 | 引入 `RouteSignals` | `internal/dataplane/router`, `internal/dataplane/observe` | 路由可统一读取健康、延迟、成本、额度、模型兼容性和禁用状态 |
| [x] | P1/E11-T06/P0 | P0 | 实现 health/cost/latency/quota 策略 | `internal/dataplane/router` | health weighted、least cost、least latency、quota aware 均有确定性测试 |
| [x] | P1/E11-T07/P0 | P0 | 在主链路加入显式 policy stage | `internal/dataplane/engine`, `internal/dataplane/policy` | GatewayEngine 清晰表达 auth、classify、policy、route、dispatch、settlement |
| [x] | P1/E11-T08/P0 | P0 | 实现 policy decision 输出 | `internal/dataplane/policy` | 支持 allow、deny、degrade、route override，并被主链路消费 |
| [x] | P1/E11-T09/P0 | P0 | 建立 model catalog 统一事实源 | `internal/controlplane/admin`, `internal/controlplane/snapshot` | 模型列表、schema、alias、ACL、provider mapping 由同一数据源发布 |
| [x] | P1/E11-T10/P1 | P1 | 将公开模型 API 切到 model catalog | `internal/transport/httpserver` | `/v1/models` 和 `/v1/models/{model}/schema` 返回不再来自散落配置 |
| [x] | P1/E11-T11/P1 | P1 | 补齐 P1 行为测试 | focused package tests, Redis integration tests | Redis、routing、policy、model catalog 测试通过 |

## P2 Architecture Advanced

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P2/E12-T01/P1 | P1 | 实现独立 `cmd/configd` | `cmd/configd`, `internal/bootstrap` | configd 可独立启动并接入 DB、Redis、logger、metrics、tracing |
| [x] | P2/E12-T02/P1 | P1 | 迁移 snapshot build/validate/publish 职责 | `internal/controlplane/snapshot`, `cmd/configd` | configd 负责发布 active snapshot，坏配置不切换版本 |
| [x] | P2/E12-T03/P1 | P1 | 实现 snapshot watch/rollback/diagnostics | `cmd/configd`, `internal/dataplane/snapshot` | gateway 可热更新，rollback 和 staleness 可观测 |
| [x] | P2/E12-T04/P1 | P1 | 补齐 IP allowlist plugin | `internal/dataplane/plugin/builtin` | 命中允许/拒绝路径有测试，敏感 IP 不进入不安全 label |
| [x] | P2/E12-T05/P1 | P1 | 补齐 Model ACL plugin | `internal/dataplane/plugin/builtin` | 模型权限可通过插件链阻断，且不绕过核心 policy |
| [x] | P2/E12-T06/P1 | P1 | 补齐 RouteOverride plugin | `internal/dataplane/plugin/builtin`, `internal/dataplane/router` | 插件可输出受约束 route decision，不绕过 billing 或 ACL |
| [x] | P2/E12-T07/P1 | P1 | 补齐 Callback plugin | `internal/dataplane/plugin/builtin`, `internal/task` | callback 行为可按 scope 配置并进入 outbox |
| [x] | P2/E12-T08/P1 | P1 | 升级 CostGuard decision | `internal/dataplane/plugin/builtin`, `internal/dataplane/policy` | CostGuard 可触发 deny、degrade 或 route decision |
| [x] | P2/E12-T09/P1 | P1 | 增强 classifier registry hint | `internal/dataplane/classifier` | 协议判断可读取 model registry hint |
| [x] | P2/E12-T10/P1 | P1 | 增强 classifier body schema inference | `internal/dataplane/classifier`, `internal/dataplane/parser` | native/unified 重叠请求可通过 schema 消歧 |
| [x] | P2/E12-T11/P1 | P1 | 补齐 `ambiguous_protocol` 场景测试 | `internal/dataplane/classifier`, `internal/dataplane/engine` | 无法判断协议时稳定返回 `ambiguous_protocol` |
| [x] | P2/E12-T12/P2 | P2 | 固化 Realtime disabled contract | `internal/dataplane/realtime`, `internal/transport/realtimehttp` | 未启用时 session API 和 WebSocket stub 稳定返回 501/feature_not_enabled |
| [x] | P2/E12-T13/P2 | P2 | 固化完整 Realtime 不进入当前路线 | `docs/plan`, `docs/tasks.md` | 保留 disabled contract、session 预留和 WebSocket stub，不再规划 WebSocket/WebRTC、session memory、双向音视频、provider realtime adapter 或 realtime billing |

## P3 Production Hardening

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P3/E13-T01/P0 | P0 | 修正 Redis 限流算法语义 | `internal/dataplane/limit`, `internal/infra/redis` | QPS/RPM token bucket、TPM estimated pre-charge、concurrency lease、daily budget 和 cost-per-minute 在 Lua 中原子判断 |
| [x] | P3/E13-T02/P0 | P0 | 引入统一 billability policy | `internal/billing`, `internal/dataplane/stream`, `internal/task` | 无有效输出不计费，部分输出、客户端断开、provider error、任务取消和任务成功都有确定性计费判定 |
| [x] | P3/E13-T03/P0 | P0 | 补齐 Native OpenAI media adapter | `internal/provider/openai`, `internal/dataplane/parser`, `internal/transport/httpserver` | `/v1/images/*`、`/v1/audio/*` native compatible 路径有转发、模型改写、错误映射和 SDK/HTTP 测试 |
| [x] | P3/E13-T04/P1 | P1 | 建立 Unified Media provider adapter contract | `internal/provider`, `internal/task` | image/video/audio/music submit、poll、cancel 有供应商字段映射，mock 只作为测试辅助 |
| [x] | P3/E13-T05/P1 | P1 | 接入真实 RouteSignals 数据源 | `internal/dataplane/router`, `internal/dataplane/observe`, `internal/infra/redis` | health/cost/latency/quota 策略读取真实信号，信号缺失时安全退回 priority/weighted |
| [x] | P3/E13-T06/P1 | P1 | 强化 configd snapshot 分发验证 | `cmd/configd`, `internal/controlplane/snapshot`, `internal/dataplane/snapshot` | publish、watch、rollback、diagnostics、configd 重启和 gateway stale policy 有进程级 smoke |
| [x] | P3/E13-T07/P1 | P1 | 完成生产集成验收与文档校准 | `docs/design`, `docs/runbook`, `tests/`, `deployments/` | OpenAPI/runbook/ADR 与实现一致，Redis integration、compose smoke、load test 和 failure drills 可复现 |

## P4 Release Candidate Readiness

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P4/E14-T01/P0 | P0 | 跑通干净依赖环境 RC smoke | `tests/rc/clean_env_smoke.sh`, `configs`, `cmd/*`, `migrations`, `docs/runbook` | 独立 project/volume 或 staging 下 MySQL、Redis、gateway、control-api、configd、worker、migration、health/ready、snapshot publish/watch/rollback 全部通过 |
| [x] | P4/E14-T02/P0 | P0 | 补齐 worker 生产运营 job | `internal/worker/jobs`, `internal/billing`, `internal/task`, `internal/bootstrap/worker.go` | failed settlement replay、provider task poller、callback dispatcher、balance hold reaper、reconciliation scheduled job 均有 lease、metrics、重试和 focused tests |
| [x] | P4/E14-T03/P0 | P0 | 接入真实 provider release channel | `internal/provider`, `internal/task`, `configs`, `docs/runbook` | OpenAI/Claude/Gemini 真实 channel 最小回归通过，至少一个真实媒体 provider task 完成 submit、poll、result、callback、settlement |
| [x] | P4/E14-T04/P1 | P1 | 生产化 configd snapshot 分发 | `cmd/configd`, `internal/controlplane/snapshot`, `internal/dataplane/snapshot`, `internal/snapshotdist` | Redis active snapshot key/pubsub 或明确 fallback 模式可复现，gateway reload、checksum、rollback、stale policy 和 diagnostics 覆盖 |
| [x] | P4/E14-T05/P1 | P1 | 补齐 OpenAPI 管理面合同 | `docs/design/ai_gateway_openapi.yaml`, `internal/transport/controlhttp`, contract tests | tenant/project/api-key/model/channel/route/price/limit/plugin/snapshot/emergency 管理接口与实现一致，关键请求/响应示例完整 |
| [x] | P4/E14-T06/P1 | P1 | 建立发布级观测与安全验收 | `deployments/observability`, `pkg/redaction`, `tests/failure`, `tools/loadtest` | dashboard、alert、redaction audit、provider attempt、billing backlog、snapshot stale、worker job、Redis latency 和 SLO 预算均可验证 |
| [x] | P4/E14-T07/P1 | P1 | 固化 staging 灰度上线 runbook | `docs/runbook`, `docs/plan`, `docs/tasks.md` | 发布 checklist 覆盖环境准备、密钥注入、迁移、配置发布、真实请求、压测、故障演练、回滚、数据核对和风险记录 |

## P5 Provider Protocol Compatibility

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P5/E15-T01/P0 | P0 | 建立 provider 协议兼容矩阵 | `docs/plan/16-p5-provider-protocol-compatibility.md`, `docs/runbook` | OpenAI、Claude、Gemini endpoint 的 request/response、stream、tool、multimodal、usage、error 和 SDK 覆盖状态可追踪 |
| [x] | P5/E15-T02/P0 | P0 | 补齐 OpenAI-compatible SDK wire shape | `internal/dataplane/parser`, `internal/provider/openai` | chat、responses、embeddings、moderations、images、audio 的 JSON/multipart/stream/tool/usage/error 行为与 OpenAI SDK 合同一致 |
| [x] | P5/E15-T03/P0 | P0 | 补齐 Claude Messages 兼容行为 | `internal/dataplane/parser`, `internal/provider/claude` | messages、tools、stream event、multimodal content block、stop reason、usage 和错误映射有合同测试 |
| [x] | P5/E15-T04/P0 | P0 | 补齐 Gemini GenerateContent 兼容行为 | `internal/dataplane/parser`, `internal/provider/gemini` | contents、tools、safetySettings、usageMetadata、finishReason、stream event 和错误映射有合同测试 |
| [x] | P5/E15-T05/P0 | P0 | 统一 provider usage/error normalization | `internal/provider`, `internal/dataplane/dispatch`, `internal/billing` | 归一化 usage 可被结算和报表复用，上游 400/401/403/404/429/5xx/timeout 映射稳定 |
| [x] | P5/E15-T06/P1 | P1 | 增加 SDK/HTTP contract tests | `tests/contract`, provider focused tests | 官方 SDK 最小请求、stream、tool calling、错误响应和协议消歧均有可重复测试 |
| [x] | P5/E15-T07/P1 | P1 | 同步协议兼容 OpenAPI 和运行文档 | `docs/design/ai_gateway_openapi.yaml`, `docs/runbook`, `docs/plan` | OpenAPI 与实际协议行为一致，未支持能力有明确 stable error 或 not planned 标注 |

## P6 Provider Reliability

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P6/E16-T01/P0 | P0 | 建立 provider/channel 健康信号模型 | `internal/dataplane/router`, `internal/dataplane/observe`, `internal/infra/redis` | 成功率、错误率、429/5xx、超时、延迟、stream 中断和手动禁用状态可统一读取 |
| [x] | P6/E16-T02/P0 | P0 | 实现 circuit breaker 状态机 | `internal/dataplane/router`, `internal/provider` | closed、open、half_open 状态可按 provider/channel/model capability 维度进入和恢复 |
| [x] | P6/E16-T03/P0 | P0 | 实现 retry budget 和 retry eligibility | `internal/dataplane/dispatch` | 同一请求的重试次数、时间预算、错误类型和 provider 范围受控，且可观测 |
| [x] | P6/E16-T04/P0 | P0 | 固化 fallback 限制规则 | `internal/dataplane/router`, `internal/dataplane/dispatch`, `internal/dataplane/stream` | 请求可重放、未输出、错误可重试且预算未耗尽时才允许 fallback；stream 已输出后禁止透明 fallback |
| [x] | P6/E16-T05/P1 | P1 | 增强 provider attempt 可追踪记录 | `internal/billing`, `internal/dataplane/observe` | attempt 记录包含原 channel、目标 channel、错误类别、预算消耗、熔断状态和最终结果 |
| [x] | P6/E16-T06/P1 | P1 | 增加 provider failure drills | `tests/failure`, focused integration tests | 429/5xx、timeout、慢响应、坏 JSON、stream 中断、熔断恢复和 emergency disable 组合场景可复现 |

## P7 Media Forwarding Providers

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P7/E17-T01/P0 | P0 | 固化非存储媒体输入语义 | `docs/design`, `docs/plan/18-p7-media-forwarding-providers.md`, OpenAPI | `/v1/files/*` 被定义为 transient/non-storage input asset，不承诺对象存储、下载或生命周期 SLA |
| [x] | P7/E17-T02/P0 | P0 | 调整文件和媒体 parser 的 transient metadata 语义 | `internal/dataplane/parser`, `internal/task` | URL、base64、multipart 只落必要 metadata、hash、size 和 source，不生成长期可下载对象承诺 |
| [x] | P7/E17-T03/P0 | P0 | 建立 media provider adapter contract | `internal/provider`, `internal/task` | image/video/audio/music 的 submit、poll、cancel、result URL、usage、error 和 metadata 映射统一 |
| [x] | P7/E17-T04/P0 | P0 | 补齐关键真实媒体 provider 映射 | `internal/provider`, `configs`, `docs/runbook` | 至少一个 image 或 video provider 的 submit、poll、cancel、result 和错误状态有真实或 fixture 测试 |
| [x] | P7/E17-T05/P1 | P1 | 打通 result URL、callback 和 settlement 衔接 | `internal/task`, `internal/billing`, `internal/worker/jobs` | provider result URL 和 usage 进入 task response、callback 和最终结算 |
| [x] | P7/E17-T06/P1 | P1 | 增加媒体任务 contract tests | `tests/contract`, provider focused tests | submit、poll running/succeeded/failed、cancel、callback、usage、idempotency 和计费衔接可验证 |

## P8 Portal API

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P8/E18-T01/P0 | P0 | 补齐 `/v1/portal/*` OpenAPI 合同 | `docs/design/ai_gateway_openapi.yaml` | Portal tag、models、schema、credits、usage、api-keys、tasks paths、schemas、security 和错误响应完整 |
| [x] | P8/E18-T02/P0 | P0 | 设计 Portal handler/service 和 API key 鉴权复用 | `internal/transport/portalhttp`, `internal/portal` | Portal 复用现有 API key principal，不引入 RBAC，不暴露 admin/control 配置能力 |
| [x] | P8/E18-T03/P0 | P0 | 实现 portal 模型、schema、credits 和 usage 查询 | `internal/portal`, `internal/billing/reporting` | 返回结果按当前 tenant/project/api key scope 过滤，不暴露 provider cost 或 repair internals |
| [x] | P8/E18-T04/P0 | P0 | 实现 portal API key 自助管理 | `internal/portal`, `internal/controlplane/admin` | 派生 key 只能继承当前 tenant/project 和 allowed_models 子集，不能查看历史 plaintext key |
| [x] | P8/E18-T05/P1 | P1 | 实现 portal task 列表和详情查询 | `internal/portal`, `internal/task` | 只能查询当前 tenant/project 下 task，隐藏 provider secret、内部错误和敏感 metadata |
| [x] | P8/E18-T06/P1 | P1 | 增加 portal 权限边界和 contract tests | `internal/transport/portalhttp`, `tests/contract` | 跨 tenant/project、扩大模型权限、禁用 key、revoked key 和标准错误响应均有测试 |

## P9 Customer Acceptance Closure

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P9/E19-T01/P0 | P0 | 固化 P9 客户接入验收范围 | `docs/plan/20-p9-customer-acceptance.md`, `docs/design` | 明确 P9 只做验收收口，不新增 RBAC、invoice、对象存储、Realtime、动态插件或多地域能力 |
| [x] | P9/E19-T02/P0 | P0 | 实现 Portal customer smoke CLI | `tools/portal-smoke`, `Makefile` | 使用 customer Bearer API key 验证 models、schema、credits、usage、api-keys 和 tasks，默认不依赖 admin token |
| [x] | P9/E19-T03/P0 | P0 | 接入 RC Portal customer acceptance | `tests/rc/clean_env_smoke.sh` | 干净依赖环境 snapshot 发布后自动执行 `portal_smoke=passed`，并覆盖派生 key 创建和 disable 生命周期 |
| [x] | P9/E19-T04/P1 | P1 | 增加 OpenAPI import preflight | `tests/contract/openapi_import_contract_test.go` | OpenAPI YAML 可解析、本地 `$ref` 可解析、operationId 唯一、Portal 继承或声明 `bearerAuth` |
| [x] | P9/E19-T05/P1 | P1 | 编写客户接入验收 runbook | `docs/runbook/customer-acceptance.md`, `docs/runbook/portal-api.md` | runbook 给出本地、staging/RC、OpenAPI preflight 和证据收口命令 |

## P10 Release Handoff Closure

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P10/E20-T01/P0 | P0 | 固化 P10 发布交接范围 | `docs/plan/21-p10-release-handoff.md`, `docs/design` | 明确 P10 只做 release/PR 收口，不新增 public API、RBAC、invoice、对象存储、Realtime、动态插件或多地域能力 |
| [x] | P10/E20-T02/P0 | P0 | 实现 release handoff CLI | `tools/release-handoff`, `Makefile` | 可生成当前 branch、commit、migration、验证命令、客户验收、发布字段和 rollback 的 Markdown 交接文档 |
| [x] | P10/E20-T03/P0 | P0 | 增加 release handoff check | `tools/release-handoff` | `-run-checks` 可执行本地 release verification 并对输出做 secret redaction |
| [x] | P10/E20-T04/P1 | P1 | 增加 PR 模板 | `.github/pull_request_template.md` | PR 模板覆盖 scope、validation、customer acceptance、release notes 和范围边界 |
| [x] | P10/E20-T05/P1 | P1 | 编写发布交接 runbook | `docs/runbook/release-handoff.md`, `docs/runbook/staging-rollout.md` | runbook 固定 handoff 生成、RC/staging 证据、PR 填写和 rollback 检查 |

## P11 Model Pricing Catalog

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P11/E21-T01/P0 | P0 | 定义模型 category 枚举和默认模板 | `internal/controlplane/admin`, `internal/controlplane/snapshot`, `docs/plan/22-p11-model-pricing-catalog.md` | chat、embedding、rerank、image、video、audio_speech、audio_transcription、music、moderation、realtime_reserved 可表达，并能进入 snapshot |
| [x] | P11/E21-T02/P0 | P0 | 实现分类价格模板和单位校验 | `internal/domain/pricing`, `internal/controlplane/admin` | 每个 category 只允许配置受支持的价格单位，非法单位发布前被拒绝 |
| [x] | P11/E21-T03/P0 | P0 | 实现客户售价组件化并兼容旧 token 字段 | `internal/controlplane/admin`, `internal/controlplane/snapshot`, `internal/domain/pricing` | 旧 input/output token 字段可读写，新逻辑统一规范化为 price components |
| [x] | P11/E21-T04/P0 | P0 | 将 hold、settlement、failed replay、async task settlement 切到统一报价器 | `internal/dataplane/admission`, `internal/billing`, `internal/task` | chat、image、video、audio task 的 hold、ledger、settlement 和 failed replay 金额一致 |
| [x] | P11/E21-T05/P1 | P1 | 实现 provider cost 组件化并保持与客户售价隔离 | `internal/billing/reporting`, migrations, reporting tests | provider cost 使用同构组件结构，但不参与客户扣费，利润报表可按 provider/channel/model 聚合 |
| [x] | P11/E21-T06/P1 | P1 | 增强模型目录展示字段和 tags/metadata | `internal/controlplane/admin`, `internal/controlplane/snapshot`, `internal/portal` | 模型 category、tags、modalities、capabilities、status、sort order 和 metadata 可发布并被 Portal/模型列表读取 |
| [x] | P11/E21-T07/P1 | P1 | 增强渠道模型 metadata、测试状态和成本配置状态 | `internal/controlplane/admin`, `internal/controlplane/snapshot` | public model 到 upstream model 映射保留，渠道模型能力覆盖、测试状态和成本状态可追踪 |
| [x] | P11/E21-T08/P1 | P1 | 增加渠道测试和上游模型同步 preview 的后台能力 | control-plane service/CLI, provider adapters | 单渠道、单模型、批量测试和上游模型列表 preview 可输出新增、删除、变更、未知 category 和缺价格/成本配置 |
| [x] | P11/E21-T09/P1 | P1 | 补齐 OpenAPI、runbook、focused tests 和兼容测试 | `docs/design/ai_gateway_openapi.yaml`, `docs/runbook`, focused tests | 复杂价格、模型分类、渠道成本、目录展示、同步 preview 和旧价格字段兼容均可验证 |

## P12 Review P0 Correctness

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P12/E22-T01/P0 | P0 | 修正 streaming concurrency lease 释放 ownership | `internal/dataplane/engine`, `internal/dataplane/dispatch`, `internal/dataplane/stream` | stream response 返回后 request/provider concurrency lease 仍保留，`AccountingStream.Close()` 或等价 finalizer 后幂等释放 |
| [x] | P12/E22-T02/P0 | P0 | 增加 stream lease 回归测试 | `internal/dataplane/stream`, `internal/dataplane/limit`, Redis integration tests | 慢速/未结束 stream close 前 Redis lease 存在，close、read error、client disconnect 后 lease 消失 |
| [x] | P12/E22-T03/P0 | P0 | 修正 async idempotency replay 的 hold 泄漏 | `internal/dataplane/engine`, `internal/task`, `internal/dataplane/admission` | 并发同 `Idempotency-Key` replay 不新增 provider request，不遗留第二个 active hold |
| [x] | P12/E22-T04/P0 | P0 | 补齐 async terminal submit 结算路径 | `internal/task`, `internal/billing`, `internal/worker/jobs` | provider submit 直接返回 succeeded/failed/canceled 时，先 settlement 或 failed settlement repair，再推进任务终态 |
| [x] | P12/E22-T05/P0 | P0 | 明确 zero-price/no-hold settlement 语义 | `internal/billing`, `internal/dataplane/admission`, migrations if needed | 免费模型、0 价格规则、non-billable policy 不因空 hold 结算失败，并保留 usage audit 或 0 金额 ledger |
| [x] | P12/E22-T06/P0 | P0 | 补齐账务不变量真实依赖测试 | `tests/`, `internal/billing`, `internal/task` | 所有 billable request 的 hold 最终 settled、released 或进入 repair，MySQL/Redis 集成测试可重复执行 |

## P13 Review P1 Commercial Hardening

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P13/E23-T01/P0 | P0 | 为 async task 持久化 price snapshot | `internal/task`, `internal/controlplane/snapshot`, migrations | task 创建后保存 currency、price rule/rate、estimated output 和 route snapshot version，后续价格变更不影响结算 |
| [x] | P13/E23-T02/P0 | P0 | 拆分 rate limit 与 spend budget 语义 | `internal/dataplane/limit`, `internal/billing`, `docs/runbook` | Redis 预扣作为 admission guard，真实 spend budget 以 settlement/ledger/reconciliation 可解释 |
| [x] | P13/E23-T03/P0 | P0 | 收敛文件和媒体输入资产策略 | `internal/task`, `internal/dataplane/parser`, `docs/design`, OpenAPI | upload 若支持则真实 streaming spool/reference；若不支持则明确 URL passthrough/inline 边界并拒绝托管语义 |
| [x] | P13/E23-T04/P0 | P0 | 实现统一 egressguard | `pkg/egressguard` 或 `internal/infra/egress`, callback/file/provider clients | file URL、callback URL 和 provider base URL 禁止访问 private/reserved/link-local/loopback/multicast IP 与 metadata service |
| [x] | P13/E23-T05/P1 | P1 | 强化 callback 签名、重试和 dead-letter | `internal/task`, `internal/worker/jobs`, `docs/runbook` | callback 带 HMAC 签名，支持 allowlist、超时、retry 上限和可追踪失败终态 |
| [x] | P13/E23-T06/P1 | P1 | 对齐 classifier 推断顺序 | `internal/dataplane/classifier`, `internal/dataplane/parser` | 按 header、path、model registry、body schema、content-type/accept、ambiguous 顺序执行并有表驱动测试 |
| [x] | P13/E23-T07/P1 | P1 | 补齐 async dispatcher fallback 能力 | `internal/task`, `internal/dataplane/router`, `internal/dataplane/dispatch` | 第一个 candidate submit 失败且未创建外部任务时，可按 retry/fallback 限制尝试后续 provider/channel |
| [x] | P13/E23-T08/P1 | P1 | 增加商业边界 focused tests | focused package tests, contract tests | Async price pin、预算语义、egress、classifier 和 async fallback 均有可重复测试 |

## P14 Review P2 Engineering Readiness

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P14/E24-T01/P1 | P1 | API key hash 迁移到 HMAC-SHA256 | `internal/dataplane/auth`, `internal/controlplane/admin`, migrations/config | 新 key 默认 `hmac-sha256`，旧 SHA-256 在兼容窗口可验证，server secret 缺失时生产 fail-closed |
| [x] | P14/E24-T02/P1 | P1 | 加固 control plane 静态 token 安全基线 | `internal/transport/controlhttp`, `internal/controlplane/admin` | constant-time compare、body size limit、写 API idempotency、operator audit hook 和网络隔离说明可验证 |
| [x] | P14/E24-T03/P1 | P1 | 为 stream close settlement 增加 timeout | `internal/dataplane/stream`, `internal/billing` | 客户端断开不取消账务修复，DB 慢或 settlement 失败不会无限阻塞 HTTP goroutine |
| [x] | P14/E24-T04/P1 | P1 | 实现按 SSE event 边界的 usage parser | `internal/provider/openai`, `internal/dataplane/stream` | usage JSON 跨 read chunk 时仍可解析，`[DONE]` 和多 `data:` 行处理稳定 |
| [x] | P14/E24-T05/P2 | P2 | 修复 nil metrics panic 风险 | `internal/billing`, `internal/worker/jobs`, bootstrap tests | metrics 缺失时使用 no-op 或构造 fail-fast，测试/复用路径不 panic |
| [x] | P14/E24-T06/P2 | P2 | 引入 trusted proxy client IP 解析 | `internal/transport/httpserver`, `internal/dataplane/plugin/builtin` | 只有可信代理 CIDR 下才信任 `X-Forwarded-For`/`X-Real-IP`，默认不信任外部 header |
| [x] | P14/E24-T07/P2 | P2 | 补齐 README 交付入口 | `README.md`, `docs/runbook` | README 覆盖产品定位、本地启动、curl、支持 API、配置、账务、安全、worker、已知限制和生产 checklist |

## P15 Review Follow-up P0 Production Blockers

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P15/E25-T01/P0 | P0 | 分离 internal request ID 与 client request ID | `internal/dataplane/engine`, `internal/transport/httpserver`, `internal/dataplane/observe` | `state.RequestID` 永远服务端生成，客户 `X-Request-ID` 只进入 `ClientRequestID`、日志/trace client 字段和 `X-Client-Request-ID` |
| [x] | P15/E25-T02/P0 | P0 | 将账务、limit 和 repair key 切到 internal request ID | `internal/billing`, `internal/dataplane/limit`, `internal/dataplane/dispatch`, `internal/worker/jobs` | hold、usage record、ledger、failed settlement、Redis lease member 和 provider attempt lease 不受客户 request id 影响 |
| [x] | P15/E25-T03/P0 | P0 | 增加重复 `X-Request-ID` 回归测试 | `internal/dataplane/engine`, `internal/billing`, Redis/MySQL integration tests | 两个不同请求携带同一客户 request id 时产生两个 internal id、两条 usage record 和独立 settlement/limit lease |
| [x] | P15/E25-T04/P0 | P0 | 为 Redis concurrency lease 增加 renewal 能力 | `internal/dataplane/limit`, `internal/infra/redis` | lease 可原子更新 zset score 和 key TTL，renew/release 幂等，renew 失败可观测 |
| [x] | P15/E25-T05/P0 | P0 | 在 stream close 生命周期中驱动 lease renewal | `internal/dataplane/stream`, `internal/dataplane/engine` | 长 stream 超过 `lease_ttl` 仍占用并发，close/read error/client disconnect 后停止 renewal 并释放 |
| [x] | P15/E25-T06/P0 | P0 | 让 hold reaper 跳过 queued/running task hold | `internal/billing`, `internal/task`, `internal/worker/jobs` | expired active hold 若被未终态 task 引用则 protected，不被普通 reaper 释放，task 完成后可结算 |
| [x] | P15/E25-T07/P0 | P0 | 禁止 provider 直接调用 customer callback URL | `internal/provider/replicate`, `internal/task`, `internal/worker/jobs` | provider submit 不包含客户 callback URL，客户 callback 只由 gateway outbox 签名、重试、审计并投递一次 |
| [x] | P15/E25-T08/P0 | P0 | 生产环境 egress guard fail-closed | `internal/bootstrap`, `pkg/egressguard`, `configs`, runbook | 非 local/test 环境启用 file URL/callback/provider custom URL 时，关闭 egress guard 会启动失败或配置发布失败 |
| [x] | P15/E25-T09/P0 | P0 | 补齐 P15 真实依赖和 failure tests | focused tests, `tests/rc/clean_env_smoke.sh` if needed | request identity、stream renewal、task hold lifecycle、callback contract 和 egress fail-closed 均有可重复验证 |

## P16 Review Follow-up Accounting Audit

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P16/E26-T01/P0 | P0 | 修正 async idempotency 并发 duplicate race | `internal/task`, `internal/dataplane/engine`, repositories | 同 key 同 body 并发创建只产生一个 task/provider submit，第二个请求稳定 replay 或返回明确 pending/conflict |
| [x] | P16/E26-T02/P0 | P0 | 校验 idempotency request hash 并稳定返回 conflict | `internal/task/idempotency.go`, repositories | 同 key 不同 body 不 replay 已有 task，第二个 admission hold 被释放或不创建 |
| [x] | P16/E26-T03/P0 | P0 | 持久化 async submit attempts | `internal/task`, `internal/billing`, migrations if needed | async provider submit 的 provider/channel、status、error、retryable、fallback、task_id 和 final 均进入 durable audit |
| [x] | P16/E26-T04/P0 | P0 | 修正 sync dispatcher final failed attempt durable 标记 | `internal/dataplane/dispatch`, `internal/billing` | 最后一个 candidate 失败时，DB 中 usage_attempts 的 final 标记正确，不只修改内存 |
| [x] | P16/E26-T05/P0 | P0 | 为 failed settlement replay 增加 row claim | `internal/billing`, `internal/worker/jobs`, migrations if needed | 多 worker 下同一 failed settlement row 只被一个 owner processing，owner 超时后可重新 claim |
| [x] | P16/E26-T06/P1 | P1 | 收敛 daily budget 产品语义 | `internal/dataplane/limit`, `internal/billing/reporting`, docs/runbook | 明确 admission estimate guard 与 ledger-based actual spend 的差异，配置命名、报表和 runbook 一致 |
| [x] | P16/E26-T07/P1 | P1 | 增加账务审计不变量测试 | focused tests, integration tests | successful provider result 最终有 usage/ledger 或 failed settlement；hold 最终 settled/released/protected/repairable |
| [x] | P16/E26-T08/P1 | P1 | 补齐 reporting 和观测字段 | `internal/billing/reporting`, metrics, logs, runbook | async attempt 失败率、final failed attempt、failed settlement backlog、budget estimate vs actual 可查询 |

## P17 Review Follow-up Worker Callback Stability

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P17/E27-T01/P1 | P1 | 校验 worker job lease TTL 与 timeout 配置 | `internal/bootstrap`, `internal/worker`, configs | 生产环境 job lease TTL 小于 timeout 安全倍数时启动失败或 normalize 并输出 warning |
| [x] | P17/E27-T02/P1 | P1 | 实现 worker lease heartbeat | `internal/worker`, `internal/infra/redis` | 长 job 运行期间定期续约 lease，owner mismatch 或连续续约失败有明确处理和 metrics |
| [x] | P17/E27-T03/P1 | P1 | 落地或移除 `MaxConcurrency()` 接口 | `internal/worker`, `internal/worker/jobs` | CallbackDispatcher 可按配置并发处理，ProviderTaskPoller 保持单并发或明确上限，接口不再空转 |
| [x] | P17/E27-T04/P1 | P1 | ProviderTaskPoller 单任务错误隔离 | `internal/worker/jobs/provider_task_poller.go`, `internal/task` | 一个 task poll 失败不阻断 batch 后续 task，task-level error 可观测 |
| [x] | P17/E27-T05/P1 | P1 | CallbackDispatcher durable claim | `internal/worker/jobs/callback_dispatcher.go`, migrations if needed | 多 worker 下同一 callback row 只被一个 owner 投递，owner 超时后可重试或 dead-letter |
| [x] | P17/E27-T06/P2 | P2 | Callback HTTP delivery 细节补齐 | `internal/worker/jobs/callback_dispatcher.go`, metrics/logs | callback 响应 body 被 drain/close，delivery id、signature version、status、latency 和 retryable 结果可观测 |
| [x] | P17/E27-T07/P2 | P2 | 增加 worker/callback failure drills | `tests/failure`, focused tests | lease 过期、heartbeat 中断、poll 单任务失败、callback 5xx/timeout 和 dispatcher crash recovery 可复现 |

## P18 Review Follow-up File Asset Boundary

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P18/E28-T01/P1 | P1 | 统一文件能力命名为 transient metadata registry | README, OpenAPI, `docs/runbook`, `docs/plan/29-p18-review-followup-file-asset-boundary.md` | 客户文档不再把当前能力描述为对象存储、download proxy 或完整文件上传存储 |
| [x] | P18/E28-T02/P1 | P1 | 修正 FileQuota 排除 expired assets | `internal/task`, repositories | quota 统计只包含未过期 asset，测试覆盖 expired、active、no-expiry 和跨 tenant/project |
| [x] | P18/E28-T03/P1 | P1 | 增加 expired transient file cleanup job | `internal/worker/jobs`, `internal/task` | cleanup 可删除或归档过期 metadata rows，输出 deleted/error/max age/next run metrics |
| [x] | P18/E28-T04/P1 | P1 | 收敛 URL asset registration 行为 | `internal/task/file_service.go`, `pkg/egressguard` | URL flow 只保存 source URL 与必要 metadata，不生成长期 gateway file URL，生产 egress 关闭时 fail closed |
| [x] | P18/E28-T05/P1 | P1 | 收敛 base64 metadata 行为和输入上限 | `internal/task/file_service.go`, parser/config | base64 只保存 hash/size/media type/metadata 或按上限拒绝，不承诺后续下载 |
| [x] | P18/E28-T06/P2 | P2 | 固化 stream upload disabled contract | `internal/transport/httpserver`, OpenAPI, README | 未实现真实 spool/reference 时稳定返回 feature disabled，文档与运行行为一致 |
| [x] | P18/E28-T07/P2 | P2 | 补齐文件资产 contract tests 和 runbook | `tests/contract`, focused tests, `docs/runbook` | quota、expires_at、cleanup、URL/base64 metadata、stream disabled 和 egress fail-closed 均可验证 |

## P19 Console Monorepo Foundation

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P19/E29-T01/P0 | P0 | 固化 console API surface 和进程边界 | `docs/design`, `docs/plan/30-p19-console-monorepo-foundation.md`, `CLAUDE.md` | `/api/portal/v1/*`、`/api/admin/v1/*`、`/portal/*`、`/admin-ui/*` 归属 `cmd/console`，`/v1/*`、`/v1/portal/*`、`/admin/*` 既有职责不变 |
| [x] | P19/E29-T02/P0 | P0 | 新增 `cmd/console` 与 bootstrap skeleton | `cmd/console`, `internal/bootstrap/console.go`, configs | console 可独立启动 health/ready，不承载 gateway 数据面和 machine control API |
| [x] | P19/E29-T03/P0 | P0 | 建立 console transport skeleton | `internal/transport/consolehttp`, `internal/transport/portalwebhttp`, `internal/transport/adminhttp` | session、CSRF、static、BFF sub-router 边界清晰，暂未实现的 API 返回稳定错误 |
| [x] | P19/E29-T04/P0 | P0 | 建立 OpenAPI split 基线 | `api/openapi`, `docs/design/ai_gateway_openapi.yaml`, contract tests | gateway-public、portal-public、portal-bff、admin-bff、control 拆分策略可校验，兼容旧 OpenAPI 入口 |
| [x] | P19/E29-T05/P0 | P0 | 建立 frontend workspace | `package.json`, `pnpm-workspace.yaml`, `tsconfig.base.json`, `web/apps`, `web/packages` | Portal/Admin app 和 shared packages 可 lint、typecheck、test、build |
| [x] | P19/E29-T06/P1 | P1 | 建立 generated API client 流程 | `web/packages/api-client`, `api/openapi`, Makefile/CI | `pnpm generate:api` 生成 Portal/Admin BFF 类型，CI 可检查 generated diff |
| [x] | P19/E29-T07/P1 | P1 | 建立 console 本地开发与 CI 命令 | Makefile, `.github/workflows`, docs/runbook | `run-console`、`web-*`、`api-check` 或等价命令可重复执行 |

## P20 Portal Web BFF

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P20/E30-T01/P0 | P0 | 迁移 Portal use case 到 `internal/app/portal` | `internal/app/portal`, `internal/portal` | 保留 `internal/portal` 兼容 shim，`/v1/portal/*` 行为和 contract tests 不变 |
| [x] | P20/E30-T02/P0 | P0 | 实现 Portal session login/logout/me | `internal/app/portal/service`, `internal/transport/portalwebhttp`, migrations/Redis | API key 只换取 HttpOnly session，不返回、不落日志、不存 browser storage |
| [x] | P20/E30-T03/P0 | P0 | 实现 Portal CSRF 和 session middleware | `internal/transport/consolehttp`, `internal/transport/portalwebhttp` | 无 session、过期/revoked session、mutation 缺 CSRF 均返回稳定错误 |
| [x] | P20/E30-T04/P0 | P0 | 实现 Portal dashboard/onboarding BFF | `internal/app/portal/service`, `internal/app/portal/repository`, `portalwebhttp` | credits、usage、API keys、tasks 聚合按 tenant/project scoped，不暴露 provider cost 或敏感 metadata |
| [x] | P20/E30-T05/P0 | P0 | 实现 Portal API key 和 task BFF | `internal/app/portal`, `internal/transport/portalwebhttp` | 派生 key 不能扩大 allowed_models，task 查询跨 tenant/project 被拒绝 |
| [x] | P20/E30-T06/P1 | P1 | 实现 Portal frontend | `web/apps/portal`, `web/packages/api-client`, `web/packages/ui` | login、dashboard、models、credits、usage、api-keys、tasks、logout 基础流程可用 |
| [x] | P20/E30-T07/P1 | P1 | 补齐 Portal OpenAPI、tests 和 smoke | `api/openapi/portal-bff.yaml`, tests, `tools` or E2E | generated client、contract tests、focused Go tests、Portal smoke 通过 |

## P21 Admin Web BFF

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P21/E31-T01/P0 | P0 | 建立 Admin app bounded context | `internal/app/admin/service`, `internal/app/admin/repository`, `internal/app/admin/types.go` | `internal/app/admin` 与 `internal/controlplane/admin` 语义清晰，service/repository 按 use case/read model 拆分 |
| [x] | P21/E31-T02/P0 | P0 | 实现 operator/session/RBAC/audit 数据模型 | migrations, `internal/app/admin/repository` | operator、session、audit tables 可迁移，bootstrap operator 策略不依赖 browser admin token |
| [x] | P21/E31-T03/P0 | P0 | 实现 Admin auth/session/CSRF | `internal/transport/adminhttp`, `internal/app/admin/service` | login/logout/me、session expiry/revoke、mutation CSRF deny 有 focused tests |
| [x] | P21/E31-T04/P0 | P0 | 实现 RBAC 和 permission guard | `internal/app/admin/service`, `internal/transport/adminhttp` | super_admin/config_admin/finance_admin/support/ops/read_only 权限边界可测试 |
| [x] | P21/E31-T05/P0 | P0 | 实现 Admin read models 和 operations views | `internal/app/admin/repository`, `internal/transport/adminhttp` | dashboard、config lists、settlements、callbacks、workers、holds 支持 filter/pagination 且响应脱敏 |
| [x] | P21/E31-T06/P0 | P0 | 实现 Admin mutation owner-service workflow | `internal/app/admin/service`, `internal/controlplane/admin`, `internal/billing`, `internal/task` | config write、snapshot publish/rollback、settlement replay、callback retry 均调用 owner service 并写 audit |
| [x] | P21/E31-T07/P1 | P1 | 补齐 Admin BFF OpenAPI 和回归测试 | `api/openapi/admin-bff.yaml`, tests/contract, focused tests | `/admin/*` machine API 行为不变，Admin BFF generated client 和 contract tests 通过 |

## P22 Console Frontend Production

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [ ] | P22/E32-T01/P0 | P0 | 实现 Admin frontend app shell 和核心页面 | `web/apps/admin`, `web/packages/ui`, `web/packages/auth` | dashboard、tenants、projects、api-keys、models、channels、routes、pricing、limits、snapshots、operations、audit、operators 可用 |
| [ ] | P22/E32-T02/P0 | P0 | 实现危险操作 UX 和权限体验 | `web/apps/admin`, `web/packages/auth` | confirm、reason、diff/dry-run、idempotency、permission denied、session expired 行为稳定 |
| [ ] | P22/E32-T03/P0 | P0 | 收口 Portal/Admin shared frontend packages | `web/packages/ui`, `web/packages/api-client`, `web/packages/format` | shared packages 只放纯 UI/auth/format/client 能力，不包含业务 feature 组件 |
| [ ] | P22/E32-T04/P0 | P0 | 建立 Portal/Admin E2E smoke | Playwright or equivalent, tests/e2e, CI | Portal login/key/usage/tasks/logout 和 Admin login/config/snapshot/audit/logout 可重复执行 |
| [ ] | P22/E32-T05/P1 | P1 | 固化 static asset 和 security headers | `cmd/console`, deploy configs, docs/runbook | `/portal/*`、`/admin-ui/*` cache policy、CSP、HSTS、cookie Secure/SameSite、CORS/CSRF 可验证 |
| [ ] | P22/E32-T06/P1 | P1 | 更新 Docker/deploy/release gate | Dockerfile, deployments, Makefile, docs/runbook, release handoff | Go checks、OpenAPI checks、web checks、E2E smoke、rollback/cache purge 进入发布证据 |
| [ ] | P22/E32-T07/P1 | P1 | 补齐 console production runbook | `docs/runbook`, `docs/tasks.md`, `docs/plan/33-p22-console-frontend-production.md` | 本地、staging、production、operator bootstrap、session secret rotation、rollback 流程清晰 |

## P23 Console Directory Structure Alignment

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P23/E33-T01/P0 | P0 | 锁定 Admin/Portal owner 决策和结构差异清单 | `docs/plan/34-p23-console-directory-structure-alignment.md`, docs/runbook or ARCHITECTURE | 明确 `internal/controlplane/admin -> internal/controlplane/configadmin` rename-only、`internal/portal` 全量迁移并删除运行包、必须拆、可延后和不应该拆的文件目录，避免空包式重构 |
| [x] | P23/E33-T02/P0 | P0 | 拆分 Admin/Portal transport handlers | `internal/transport/adminhttp`, `internal/transport/portalhttp`, `internal/transport/portalwebhttp` | routes、auth、dashboard、resource handlers、response/session/csrf helpers 分文件，路径和响应行为不变 |
| [x] | P23/E33-T03/P0 | P0 | 全量迁移 Portal public use case 并拆分 service/repository | `internal/app/portal/service`, `internal/app/portal/repository`, `internal/transport/portalhttp` | `/v1/portal/*` 直接由 `internal/app/portal` public API service 支撑，删除 `internal/portal` runtime package，auth、dashboard、models、credits、usage、api_keys、tasks、onboarding、sessions 分文件，P8/P20 tests 继续通过 |
| [x] | P23/E33-T04/P0 | P0 | Rename control config owner 并拆分 Admin app service | `internal/controlplane/configadmin`, `internal/app/admin/service` | `internal/controlplane/admin` rename-only 到 `configadmin`，auth、config、pricing、operations、audit、sessions 分文件，RBAC/audit/owner service flow 不变 |
| [x] | P23/E33-T05/P0 | P0 | 拆分 Portal/Admin frontend apps | `web/apps/portal/src`, `web/apps/admin/src` | `App.tsx` 拆为 app/routes/providers/features/shared，现有 UI 行为和 generated client 调用不变 |
| [x] | P23/E33-T06/P1 | P1 | 拆分 frontend shared packages | `web/packages/api-client`, `web/packages/ui`, `web/packages/auth`, `web/packages/format`, optional `config/test-utils` | api-client、auth、format、ui 按职责分文件，保留 public exports，不放业务 feature 组件 |
| [x] | P23/E33-T07/P1 | P1 | 补齐 API examples/scripts 和 OpenAPI bundle 入口 | `api/examples`, `api/scripts`, `api/openapi` | lint、generate TS client、api diff check 脚本可被 Makefile/CI 调用，examples 不含 secret |
| [x] | P23/E33-T08/P1 | P1 | 增加 import 边界和结构回归检查 | scripts, Makefile/CI, docs/runbook | 防止 app 互相依赖、packages 反向依赖 apps、Admin browser 调 `/admin/*`、`controlplane/configadmin -> app/admin`、新增全局 store/repository |
| [x] | P23/E33-T09/P1 | P1 | 完成结构重构验证和文档同步 | docs/tasks.md, docs/runbook, ARCHITECTURE or README | `go test ./internal/controlplane/configadmin ./internal/app/admin/... ./internal/app/portal/... ./internal/transport/adminhttp ./internal/transport/portalhttp ./internal/transport/portalwebhttp`、运行代码无 `internal/portal` import、`go test ./...`、`go vet ./...`、`go build ./cmd/...`、`pnpm generate:api`、`pnpm lint/typecheck/test/build`、`make api-check`、`git diff --check` 通过 |
| [x] | P23/E33-T10/P1 | P1 | 拆分 Admin app 和 configadmin repository | `internal/app/admin/repository`, `internal/controlplane/configadmin` | 按业务聚合拆为 operators、sessions、audit、tenants、projects、api_keys、models、channels、routes、pricing、limits、plugins、marketplace、snapshots 等文件，文件名不使用 `mysql_` 前缀，repository contract 不变 |

## P24 NewAPI Lean Console Parity

| 状态 | ID | 优先级 | 任务 | 目标位置 | 验收标准 |
|---|---|---|---|---|---|
| [x] | P24/E34-T01/P0 | P0 | 固化 NewAPI 精简对标设计和裁剪边界 | `docs/design/ai_gateway_newapi_lean_console_design.md`, `docs/plan/35-p24-newapi-lean-console-parity.md`, `docs/tasks.md`, `CLAUDE.md` | 明确渠道、模型、用户/客户账户、令牌、日志、任务、操练场和最小额度运营进入范围；用户/模型/渠道分组、倍率、订阅、兑换码、支付、邀请返利、模型部署和复杂系统设置不进入 P24 |
| [x] | P24/E34-T02/P0 | P0 | 建立 Admin/Portal P24 信息架构和 route plan | `web/apps/admin`, `web/apps/portal`, `docs/runbook` | Admin 使用 Workbench/Catalog/Routing/Accounts/Activity/Tools/Settings 信息架构，Portal 使用 Dashboard/Models/Playground/API Keys/Usage/Tasks/Credits/Settings，不复制 NewAPI 菜单结构，前端静态展示文案使用中文并预留 `zh-CN` i18n 入口 |
| [x] | P24/E34-T03/P0 | P0 | 实现 Channel Management lean workflows | `internal/app/admin/service`, `internal/controlplane/configadmin`, `internal/transport/adminhttp`, `web/apps/admin` | channel safe CRUD、credential rotation、test、sync preview/apply、health read model、model coverage、route policy hint 和 audit 可用，credential 不返回明文 |
| [ ] | P24/E34-T04/P0 | P0 | 实现 Model Management 和 Portal 模型预览 | `internal/app/admin/service`, `internal/app/portal/service`, `web/apps/admin`, `web/apps/portal` | model catalog metadata、category、modalities、capabilities、schema preview、pricing summary、channel coverage 和 Portal preview 可用，UI/API 不出现倍率 |
| [ ] | P24/E34-T05/P0 | P0 | 实现 Customer Account Management | `internal/app/admin/service`, `internal/app/admin/repository`, `internal/billing`, `web/apps/admin` | tenant/project scoped customer account、status、role、credits、API key、session reset、manual adjustment 和 audit 可用，UI/API 不出现用户分组 |
| [ ] | P24/E34-T06/P0 | P0 | 收口 Admin/Portal API Key 管理 | `internal/app/admin/service`, `internal/app/portal/service`, `api/openapi`, `web/apps/admin`, `web/apps/portal` | create/disable/rotate、allowed models、IP allowlist、expires_at、usage summary 可用，plaintext key 只显示一次且不能扩大权限 |
| [ ] | P24/E34-T07/P1 | P1 | 实现 Usage Logs 和 Task Logs 运营视图 | `internal/app/admin/repository`, `internal/app/portal/repository`, `web/apps/admin`, `web/apps/portal` | usage/task 支持 time range、request_id/task_id、tenant/project、API key、model、channel、provider、status 等筛选，详情只返回 safe metadata |
| [ ] | P24/E34-T08/P1 | P1 | 实现 schema-driven Playground 和 Channel Test 复用 | `internal/app/admin/service`, `internal/app/portal/service`, `web/apps/admin`, `web/apps/portal` | Admin/Portal playground scope 隔离，支持 schema-driven payload、stream/debug safe result、导入导出不含 secret，并复用 channel test executor |
| [ ] | P24/E34-T09/P1 | P1 | 实现最小额度运营和 ledger/export 视图 | `internal/billing`, `internal/app/admin/service`, `internal/app/portal/service`, `web/apps/admin`, `web/apps/portal` | balance、ledger、active holds、failed settlement links、manual adjustment 和 usage/ledger export 可用，额度调整必须 reason + idempotency + audit |
| [ ] | P24/E34-T10/P1 | P1 | 补齐 P24 OpenAPI、generated client 和 contract tests | `api/openapi/admin-bff.yaml`, `api/openapi/portal-bff.yaml`, `web/packages/api-client`, tests | OpenAPI schema、generated TS client、BFF contract tests、RBAC/CSRF/audit/redaction tests 一致，denylist 字段不出现在 safe DTO |
| [ ] | P24/E34-T11/P1 | P1 | 增加 P24 cut-scope regression | tests, `web/apps/admin`, `web/apps/portal`, docs/runbook | UI/API 不暴露 group/ratio/payment/subscription/redemption/deployment/broad settings route 或字段，任务板裁剪边界可被测试或静态检查覆盖 |
| [ ] | P24/E34-T12/P1 | P1 | 完成 P24 E2E smoke、runbook 和 release evidence | tests/e2e or equivalent, docs/runbook, release handoff | Admin channel/model/customer/API key/log/playground smoke 和 Portal models/playground/API key/usage/tasks/credits smoke 可重复执行，release handoff 记录 known limits 和验证命令 |

## 阶段验收总览

| 阶段 | 验收标准 |
|---|---|
| M0 | `go test ./...`、`make lint`、healthz、readyz、metrics、OpenAPI、ADR 和文档入口通过 |
| M1 | `/v1/chat/completions` non-stream 可认证、路由、调用 provider、返回标准响应并记录基础观测 |
| M2 | provider 成功后的本地结算失败可修复，ledger 与 balance 可对账 |
| M3 | OpenAI stream、Claude、Gemini 和 stream close-time accounting 可用 |
| M4 | 统一媒体任务可创建、幂等查询、轮询、回调和最终结算 |
| M5 | 新增模型、渠道、价格、路由和限流无需重启 gateway |
| M6 | 插件可按 scope 绑定并执行 deny、redact、audit 和 metrics 行为 |
| M7 | dashboard、alert、压测和 failure drills 可支撑灰度商用 |
| M8 | Realtime session API 和 WebSocket stub 可编译，未启用时明确返回 501 |
| M9 | 客户、运营和财务能围绕余额、用量、成本、利润、对账和灾备开展运营 |
| P0 | worker、异步任务、公开 API、snapshot stale policy 和 emergency disable 形成生产闭环 |
| P1 | 多维限流、策略路由、显式 policy stage 和 model catalog 达到设计能力要求 |
| P2 | 独立 configd、剩余插件、分类器增强和 Realtime disabled contract 边界与完整架构蓝图一致 |
| P3 | 限流算法语义、账务计费策略、Native media、RouteSignals、configd 分发和生产集成验收达到商用硬化要求 |
| P4 | 干净依赖环境、真实 provider、worker 运营 job、OpenAPI 管理面、观测安全和灰度回滚验收达到 release candidate 要求 |
| P5 | OpenAI、Claude、Gemini 的 SDK 兼容、stream、tool calling、multimodal、usage/error 映射和合同测试达到可客户接入要求 |
| P6 | Provider/channel 健康信号、熔断、retry budget、fallback 限制和 failure drills 能隔离上游故障 |
| P7 | 媒体能力以非存储输入资产转发为边界，真实 provider task 生命周期、result URL、callback 和 settlement 可验证 |
| P8 | Portal API 支持模型、schema、credits、usage、API key 自助管理和 task 查询，权限边界清晰 |
| P9 | 客户接入验收可重复执行，Portal smoke、OpenAPI import preflight 和 RC smoke 集成可形成发布证据 |
| P10 | 发布交接可重复生成，PR 模板、release handoff、验证命令、客户验收和 rollback 字段可形成交付证据 |
| P11 | 模型分类、复杂价格、渠道成本、模型目录展示和渠道测试/同步 preview 可验证，客户售价与 provider 成本边界清晰 |
| P12 | Review P0 正确性问题收敛，stream lease、async idempotency、terminal task settlement、zero-price/no-hold settlement 和真实依赖账务不变量可验证 |
| P13 | Review P1 商业账务与安全边界收敛，async price pin、预算语义、transient input asset、egressguard、classifier 和 async fallback 可验证 |
| P14 | Review P2 工程交付与安全基线收敛，API key HMAC、管理面安全基线、stream timeout、SSE parser、trusted proxy 和 README 可验证 |
| P15 | Follow-up review 生产阻塞收敛，internal request id、stream lease renewal、task hold lifecycle、callback contract 和 egress fail-closed 可验证 |
| P16 | Follow-up review 账务审计一致性收敛，async idempotency race、async attempt audit、final failed attempt、failed settlement claim 和 budget 语义可验证 |
| P17 | Follow-up review worker/callback 稳定性收敛，worker heartbeat、job concurrency、poller 错误隔离、callback durable claim 和 failure drills 可验证 |
| P18 | Follow-up review 文件资产边界收口，transient metadata registry、expired quota、cleanup job、stream disabled contract 和客户文档边界可验证 |
| P19 | Console monorepo foundation 可验证，`cmd/console`、OpenAPI split、frontend workspace、generated client、CI 和本地开发入口可用 |
| P20 | Portal Web BFF 和 Portal UI 可验证，session login、dashboard、API key、usage、tasks、smoke 和 `/v1/portal/*` 兼容性稳定 |
| P21 | Admin Web BFF 可验证，operator session、RBAC、audit、safe read/write workflow、operations views 和 `/admin/*` 兼容性稳定 |
| P22 | Console frontend production 可验证，Admin UI、Portal/Admin 前端收口、static asset、安全头、E2E smoke、deployment/rollback runbook 完成 |
| P23 | Console 目录结构对齐可验证，`internal/controlplane/admin` 已 rename-only 到 `internal/controlplane/configadmin`，`internal/portal` 已全量迁移到 `internal/app/portal` 并退出运行代码树，handler/service/repository/frontend packages 按目标树拆分且 API 行为、session、RBAC、audit 和 frontend public exports 保持兼容 |
| P24 | NewAPI 精简对标可验证，渠道、模型、用户/客户账户、令牌、使用日志、任务日志、操练场和最小额度运营完成，且用户/模型/渠道分组、倍率、订阅、兑换码、支付、邀请返利、模型部署和复杂系统设置未进入 UI/API |
