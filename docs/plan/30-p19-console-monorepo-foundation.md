# P19 Console Monorepo Foundation

## 阶段目标

P19 的目标是为完整 Portal/Admin 前后端建立基础结构，但不急于实现完整业务页面。该阶段要把 console 作为新的 Human Console Plane 纳入项目：新增 `cmd/console` 的进程边界、OpenAPI contract 拆分规则、前端 workspace、generated API client、CI/Makefile 验证入口和本地开发约定。

P19 解除此前“Portal 只做后端 API、不做 UI”的阶段性限制，但只做 foundation，不在本阶段扩张 Admin 写操作、RBAC 复杂策略或对象存储/支付/发票等新产品面。

## 当前状态

- `/v1/*` 数据面由 `cmd/gateway` 承载。
- `/v1/portal/*` programmatic Portal API 已在 P8 落地，仍使用 customer Bearer API key。
- `/admin/*` machine Control API 已由 `cmd/control-api` 承载。
- P19 已建立 browser-facing Portal/Admin BFF skeleton、`cmd/console`、frontend monorepo、OpenAPI 多文件拆分和 TS client generation 基线；P20/P21 继续补真实 session 与业务 handler。

## 交付物

- `cmd/console` 进程骨架和 `internal/bootstrap/console.go` wiring，占位但不接管现有 gateway/control/worker/configd。
- `internal/transport/consolehttp` 基础 mux/session/static 边界，明确 `/api/portal/v1/*`、`/api/admin/v1/*`、`/portal/*`、`/admin-ui/*` route ownership。
- `api/openapi/README.md` 或等价设计入口，说明 `gateway-public`、`portal-public`、`portal-bff`、`admin-bff`、`control` 的拆分策略。
- 保留 `docs/design/ai_gateway_openapi.yaml` 作为短期兼容入口，明确后续可以生成 aggregate 或镜像。
- `web/` monorepo scaffold：`web/apps/portal`、`web/apps/admin`、`web/packages/api-client`、`web/packages/ui`、`web/packages/auth`、`web/packages/format`。
- 根部 `package.json`、`pnpm-workspace.yaml`、`tsconfig.base.json`、ESLint/Prettier 配置和基础 build/typecheck/test scripts。
- API generation 入口：从 `api/openapi/portal-bff.yaml` 和 `api/openapi/admin-bff.yaml` 生成前端类型。
- Makefile/CI 入口：`run-console`、`console`、`web-lint`、`web-typecheck`、`web-test`、`web-build`、`api-generate`、`api-check` 或等价命令。
- 本地开发端口和 dev proxy 约定：console 9505，Portal/Admin Vite dev server 代理到 `http://localhost:9505/api/*`。

## 核心实现顺序

1. 固化 route boundary：`/v1/*` 和 `/v1/portal/*` 仍在 `cmd/gateway`，`/admin/*` 仍在 `cmd/control-api`，新增 `/api/portal/v1/*` 和 `/api/admin/v1/*` 只属于 `cmd/console`。
2. 新增 `cmd/console` 最小启动链路，复用现有配置加载、logger、telemetry、DB/Redis client 和 graceful shutdown 约定。
3. 新增 `internal/bootstrap/console.go`，只做依赖组装，不写业务逻辑、RBAC 判断、SQL 聚合或审计 diff。
4. 新增 `internal/transport/consolehttp`，先提供 health/ready、session/static route skeleton 和 BFF sub-router 挂载点。
5. 建立 OpenAPI 拆分目录和 README，先放 skeleton contract，确保 path/security/tag 命名不会与现有 `docs/design/ai_gateway_openapi.yaml` 冲突。
6. 建立 frontend workspace，Portal/Admin app 先能启动和构建，页面可以是空 shell 或 mock state。
7. 建立 `web/packages/api-client`，实现 generated types 放置路径和统一 fetch wrapper skeleton，fetch wrapper 预留 `credentials: "include"`、CSRF header 和错误归一化。
8. 建立 `web/packages/ui`、`auth`、`format` 的最小可编译导出，不放业务组件。
9. 更新 Makefile/CI，让 Go、OpenAPI、frontend 三条验证链路可单独运行，也能进入后续 `full-check`。
10. 更新 README/runbook 或开发文档，记录本地五进程 + 前端 dev server 启动顺序。

## 关键设计约束

- P19 不改变现有 `/v1/portal/*` 行为，不破坏 P8/P9/P10 客户验收资产。
- P19 不让 browser 调 `/admin/*`，也不把 admin token 注入前端配置。
- P19 不引入 Next.js/SSR，默认采用 Vite SPA；除非后续有明确 SEO/SSR 需求。
- Frontend package 依赖方向必须单向：apps 依赖 packages，packages 不依赖 apps，Portal/Admin app 互不依赖。
- `web/packages/ui` 只放纯 UI 组件，不放 tenant/channel/usage 等业务组件。
- API contract 是前后端协作边界，BFF handler 改动必须同步 OpenAPI，OpenAPI 改动必须同步 generated client。
- 新目录可以先 skeleton，但不能堆无 owner 的空包；每个包必须有明确后续阶段用途。

## 验收标准

- `go run ./cmd/console -config configs/local.yaml` 或等价命令能启动 console 进程，并暴露 health/ready。
- `cmd/gateway`、`cmd/control-api`、`cmd/configd`、`cmd/worker` 启动入口不受影响。
- `/api/portal/v1/*`、`/api/admin/v1/*`、`/portal/*`、`/admin-ui/*` 的 route ownership 在 docs 和 code skeleton 中一致。
- `pnpm install --frozen-lockfile`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build` 或等价命令通过。
- `pnpm generate:api` 或等价命令可从 BFF OpenAPI 生成 TS 类型。
- `git diff --check` 通过，新增 OpenAPI/plan/tasks/CLAUDE/design 文档互相指向一致。
- P19 完成后，后续 P20 可以在不重排目录的情况下实现 Portal Web BFF 和 Portal UI。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 一次性生成大量空包 | 只创建 P19/P20/P21 明确需要的 skeleton；无 owner 的包不落地 |
| OpenAPI 双入口漂移 | 明确 `api/openapi/*` 为后续 source，`docs/design/ai_gateway_openapi.yaml` 短期作为兼容 aggregate 或镜像 |
| 前端构建拖慢 Go CI | CI 拆分 go、api、web job，允许并行运行 |
| Admin token 被误放进前端 | P19 配置规则禁止 browser 使用 `control.admin_token`，后续 Admin 只走 operator session |
| `/admin-ui/*` 与 `/admin/*` 冲突 | Admin SPA 固定使用 `/admin-ui/*` 或独立 host，不占用 `/admin/*` |
| frontend monorepo 过早复杂化 | 先不用 turborepo，只有 build graph 复杂到需要时再引入 |

## 依赖关系

- 依赖 P8 的 `/v1/portal/*` programmatic API 作为 Portal Web 登录和自助能力的后端事实源之一。
- 依赖 P12-P18 的账务、worker、callback、文件边界和 outbound 安全收敛，避免 console 建在不稳的核心链路上。
- P19 完成后解锁 P20 Portal Web BFF、P21 Admin Web BFF 和 P22 Console frontend production。

## 设计来源

- [路线图](./00-roadmap.md)
- [Portal/Admin Console Monorepo 设计](../design/ai_gateway_console_monorepo_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
- [ADR](../design/ai_gateway_ADR.md)
- [任务清单](../tasks.md)
