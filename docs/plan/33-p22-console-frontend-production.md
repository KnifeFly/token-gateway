# P22 Console Frontend Production

## 阶段目标

P22 的目标是把 P20/P21 的 Portal/Admin BFF 能力交付成可用的完整 Console：Admin UI 全量页面、Portal/Admin 前端体验收口、静态资源部署、安全头、E2E smoke、观测、发布/回滚 runbook 和 release gate。

P22 不新增对象存储、支付/发票、完整 Realtime、WASM/动态插件或多地域能力。该阶段专注 human console 的生产可交付性。

## 交付物

- Admin UI：dashboard、tenants、projects、API keys、models、channels、routes、pricing、limits、snapshots、operations、audit、operators。
- Portal UI 收口：导航、空态、错误态、loading、pagination、filter、export、settings、session expiry 体验。
- Shared UI package 收口：Button、Input、Select、Dialog、ConfirmDialog、DataTable、StatusBadge、SecretInput、DateRangePicker、EmptyState、ErrorState、LoadingState、Toast。
- Shared auth package：session fetch、CSRF token、permission guard、401/403 handling。
- Shared api-client：generated types、fetch wrapper、error normalization、request_id display。
- Static asset strategy：CDN/Nginx 或 `cmd/console` embedded/static dist，明确 `/portal/*` 和 `/admin-ui/*` cache policy。
- Security headers：CSP、HSTS、X-Content-Type-Options、Referrer-Policy、Frame-Options 或 frame-ancestors、cookie Secure/SameSite。
- E2E smoke：Portal login/dashboard/key/usage/tasks/logout；Admin login/dashboard/config/snapshot/audit/logout。
- Release gate：Go tests、OpenAPI checks、generated client diff、frontend lint/typecheck/test/build、E2E smoke。
- Deployment docs：本地、staging、production、rollback、static asset cache purge、operator bootstrap、session secret rotation。

## 核心实现顺序

1. 完成 Admin app shell、routing、navigation、auth guard、permission guard 和 layout，确保无权限页面不加载敏感数据。
2. 按业务域实现 Admin pages：dashboard -> read-only config lists -> operations -> snapshots -> config write forms -> operator management。
3. 为每个危险操作增加 confirm dialog、reason input、dry-run/diff preview 和 idempotency key 生成策略。
4. 完成 Portal frontend polish：usage filter/export、task detail、API key create/disable flow、session expiry、empty/error/loading states。
5. 收敛 shared UI package，只保留纯 UI；业务 table/form 留在各自 app feature。
6. 完成 api-client fetch wrapper：credentials include、CSRF、401 redirect、403 permission denied、409 conflict、422 field errors、429 rate limited、5xx request_id。
7. 建立 frontend tests：routing smoke、auth guard、dashboard empty state、table pagination、form validation、permission denied、dangerous action confirmation。
8. 建立 Playwright 或等价 E2E smoke，并接入 local/staging console。
9. 决定静态资源部署模式：CDN/Nginx 优先，`cmd/console` static 作为简化部署；配置 HTML 与 hashed assets cache policy。
10. 加强 security headers、cookie、CORS、CSRF、CSP，并编写安全回归检查。
11. 更新 Dockerfile/deployment/runbook，支持前端 build stage 或独立 static publish。
12. 将 release gate 接入 CI 和 release handoff，形成可重复证据。

## Admin UI 页面范围

```text
Dashboard
Tenants
Projects
API Keys
Models
Model Marketplace / Catalog
Channels
Routes
Pricing
Limits
Snapshots
Operations
  Settlements
  Callbacks
  Workers
  Holds
Audit
Operators
```

首版页面可以按风险降级：只读页面必须完整可查；高风险写操作可以先提供 validate/dry-run，再逐步开放 publish/disable/replay。

## 前端错误处理

统一展示：

```text
message
code
request_id
retryable
field_errors
```

错误行为：

- 401：跳转登录或弹出 session expired。
- 403：展示权限不足，不重试。
- 404：显示 not found，不暴露敏感资源存在性。
- 409：展示 idempotency/conflict。
- 400/422：展示字段错误。
- 429：展示限流和可重试提示。
- 5xx：展示 request_id，隐藏 raw backend error。

## 静态资源与部署

推荐生产部署：

```text
CDN/Nginx:
  /portal/*
  /admin-ui/*

cmd/console:
  /api/portal/v1/*
  /api/admin/v1/*
```

简化部署可由 `cmd/console` serve dist：

```text
/portal/*      -> web/apps/portal/dist
/admin-ui/*    -> web/apps/admin/dist
/api/*         -> BFF
```

缓存要求：

- HTML: `Cache-Control: no-cache`
- hashed JS/CSS: `Cache-Control: public, max-age=31536000, immutable`
- Admin SPA 不占用 `/admin/*`

## 关键设计约束

- UI 不绕过 generated client 直接拼裸 fetch。
- Admin UI 不显示 plaintext credential、ciphertext、API key hash、raw prompt、raw response、raw repair payload。
- SecretInput 只能输入新 secret 或展示 fingerprint/last_rotated_at，不能展示存量 secret。
- Dangerous action 必须 reason + confirm + idempotency + audit 可追踪。
- Console 静态部署不能影响 `cmd/gateway` 数据面性能。
- CSP 初版可以从严格同源开始，第三方脚本/字体/图表库必须显式 allowlist。
- P22 不把 console 变成 observability 平台；只展示本产品必要运营视图。

## 验收标准

- Portal/Admin frontend `pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build` 通过。
- Generated API client 与 OpenAPI 无 diff；OpenAPI contract tests 通过。
- Portal E2E smoke 覆盖 login、dashboard、API key create/disable、usage、tasks、logout。
- Admin E2E smoke 覆盖 login、dashboard、list tenants/projects/models/channels、validate snapshot、publish/rollback 或 dry-run、audit、logout。
- Security header check 通过，cookie 使用 HttpOnly/Secure/SameSite，CSRF mutation deny 可复现。
- Static asset cache policy 可验证，rollback 或 cache purge runbook 可执行。
- Release handoff 能包含 Go checks、web checks、OpenAPI checks、E2E smoke、known limits 和 rollback 证据。
- `go test ./...`、`go vet ./...`、`git diff --check` 或 release gate 约定的等价命令通过。

## 风险与处理

| 风险 | 处理 |
|---|---|
| Admin UI 一次性范围过大 | 按 read-only -> operations -> config write -> operator management 顺序交付 |
| UI 泄漏敏感字段 | Safe DTO + generated client + contract tests + frontend snapshot denylist |
| Dangerous action 误操作 | confirm dialog、reason、diff/dry-run、idempotency、audit 和 permission guard |
| Static cache 发布后无法回滚 | HTML no-cache、hashed assets immutable、runbook 包含 cache purge 和 previous dist rollback |
| E2E 不稳定拖慢交付 | smoke 只覆盖核心路径，复杂场景留给 focused tests |
| CSP 过松失去价值 | 默认 self，新增外部资源必须在 PR 中说明和测试 |

## 依赖关系

- 依赖 P20 Portal Web BFF 和 P21 Admin Web BFF。
- 依赖 P19 的 frontend workspace、OpenAPI generation 和 CI baseline。
- 完成 P22 后，Portal/Admin 前后端可作为正式 console product 进入后续运营增强阶段。

## 设计来源

- [P20 Portal Web BFF](./31-p20-portal-web-bff.md)
- [P21 Admin Web BFF](./32-p21-admin-web-bff.md)
- [Portal/Admin Console Monorepo 设计](../design/ai_gateway_console_monorepo_design.md)
- [实施计划](../design/ai_gateway_implementation_plan.md)
- [任务清单](../tasks.md)
