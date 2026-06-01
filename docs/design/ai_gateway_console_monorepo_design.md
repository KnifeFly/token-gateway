# Portal/Admin Console Monorepo 设计

本文档定义 token-gateway 后续完整 Portal/Admin 前后端的目标架构。它不改变已完成的 Data Plane、Control Plane、Config Plane、Worker Plane 职责，而是在其上新增 Human Console Plane，用于承载浏览器 Portal/Admin、BFF API、session、RBAC、审计和前端 monorepo。

## 1. 目标

后续阶段需要从“Portal 第一版只做后端 API”扩展到完整的人机界面：

- Portal Web：客户自助查看模型、余额、用量、任务、API key、onboarding 和 project settings。
- Admin Web：运营人员管理 tenant/project/API key、模型、渠道、路由、价格、限额、snapshot、worker/callback/settlement 操作和 audit。
- Portal/Admin BFF：为浏览器提供 session、CSRF、页面聚合 read model、RBAC、redaction 和 UI-friendly error shape。
- Frontend monorepo：Portal/Admin 两个 SPA 与共享 UI、auth、format、generated API client package。
- API contract first：OpenAPI 是后端 handler、前端类型和 contract test 的协作边界。

目标不是把所有进程合并成单体，也不是让 Admin 前端直接调用机器控制 API。新增 console 只负责 human workflow，不接管 gateway 热路径、账务状态机、task 状态机或 snapshot 发布核心逻辑。

## 2. 平面与进程边界

保留现有进程：

```text
Data Plane       cmd/gateway      /v1/*, /v1/portal/*
Control Plane    cmd/control-api  /admin/*
Config Plane     cmd/configd      snapshot distribution
Worker Plane     cmd/worker       async task, settlement, callback, jobs
Human Console    cmd/console      /api/portal/v1/*, /api/admin/v1/*, optional static assets
```

生产推荐域名：

```text
api.example.com                 -> cmd/gateway
control.internal.example.com    -> cmd/control-api
console.example.com/portal      -> Portal SPA
console.example.com/admin-ui    -> Admin SPA
console.example.com/api/*       -> cmd/console BFF
```

本地默认端口建议：

```text
gateway      9501
control-api  9502
worker       9503
configd      9504
console      9505
```

## 3. API Surface

| Surface | Path | Process | Auth | Consumer | Stability |
|---|---|---|---|---|---|
| Data Plane Public | `/v1/*` | `cmd/gateway` | Bearer customer API key | SDK / customers | High |
| Portal Public API | `/v1/portal/*` | `cmd/gateway` | Bearer customer API key | scripts / customer automation | High |
| Portal Web BFF | `/api/portal/v1/*` | `cmd/console` | HttpOnly session + CSRF | Portal SPA | Medium |
| Admin Web BFF | `/api/admin/v1/*` | `cmd/console` | operator session + RBAC + CSRF | Admin SPA | Medium |
| Machine Control API | `/admin/*` | `cmd/control-api` | admin token / future mTLS | ops automation | High |
| Static Web | `/portal/*`, `/admin-ui/*` | CDN or `cmd/console` | browser route guard | browser | Low |

`/admin/*` 不能成为 Admin SPA 的浏览器 API。它是机器控制 API，适合 curl、CI 和内部 automation。Admin SPA 需要 operator session、RBAC、CSRF、audit、pagination、table filters、diff preview、confirm workflow 和 redaction，这些能力属于 `/api/admin/v1/*`。

`/v1/portal/*` 继续作为稳定 programmatic Portal API。Portal SPA 使用 `/api/portal/v1/*`，因为浏览器场景需要 session cookie、CSRF、dashboard 聚合和更灵活的页面响应。

## 4. 目标目录结构

```text
api/
  openapi/
    gateway-public.yaml
    portal-public.yaml
    portal-bff.yaml
    admin-bff.yaml
    control.yaml
    components/
      common.yaml
      errors.yaml
      pagination.yaml
      money.yaml
      usage.yaml

cmd/
  gateway/
  control-api/
  configd/
  worker/
  migrate/
  console/

internal/
  app/
    portal/
      types.go
      service/
        service.go
        ports.go
        auth.go
        dashboard.go
        models.go
        credits.go
        usage.go
        api_keys.go
        tasks.go
        onboarding.go
        sessions.go
      repository/
        repository.go
        memory.go
        mysql.go
        mysql_dashboard.go
        mysql_usage.go
        mysql_api_keys.go
        mysql_tasks.go
        mysql_sessions.go
    admin/
      types.go
      service/
        service.go
        ports.go
        auth.go
        dashboard.go
        tenants.go
        projects.go
        api_keys.go
        models.go
        channels.go
        routes.go
        pricing.go
        limits.go
        snapshots.go
        operations.go
        audit.go
        operators.go
        sessions.go
      repository/
        repository.go
        memory.go
        mysql.go
        mysql_sessions.go
        mysql_operators.go
        mysql_dashboard.go
        mysql_config_views.go
        mysql_operations.go
        mysql_audit.go
  portal/
    service.go
    types.go
  controlplane/
    admin/
  transport/
    consolehttp/
    portalhttp/
    portalwebhttp/
    adminhttp/
    controlhttp/

web/
  apps/
    portal/
    admin/
  packages/
    api-client/
    ui/
    auth/
    format/
    config/
    test-utils/
```

`internal/portal` 可以作为兼容 shim 过渡到 `internal/app/portal`，但新增 Portal Web 能力应落在 `internal/app/portal`。新增 Admin 应使用 `internal/app/admin`，避免与已有 `internal/controlplane/admin` 混淆。

## 5. Repository 与 Service 拆分规则

不要做全局 `repository` 或全局 `service`。Portal/Admin 可以各自有 `service/` 和 `repository/` 子目录，但必须挂在清晰 bounded context 下：

```text
internal/app/portal/service
internal/app/portal/repository
internal/app/admin/service
internal/app/admin/repository
```

拆分原则：

- service port 按 use case 拆，不按数据库表机械拆。
- repository SQL 文件按页面/read model/操作族拆，避免巨大 `mysql_repository.go`。
- HTTP handler 按 resource 拆，transport 只做 decode、auth/session、route/query param、service call、response write 和 error mapping。
- owner service 不变：billing owns settlement/ledger/holds；task owns task lifecycle/callback；controlplane admin owns config writes；snapshot owns build/validate/publish/rollback。

Portal repository 可以做 dashboard、usage、credits、task list/detail、safe API key metadata、session store 和 onboarding read model。不得写 ledger、推进 task 状态机、发布 snapshot、修改 provider/channel/route/price/limit，或返回 raw prompt/raw response/provider secret。

Admin repository 可以做 dashboard、config safe read model、operations views、operator/session/RBAC 和 audit store。不得绕过 `controlplane/admin.Service` 写配置，不得绕过 billing/task owner service 做 repair/retry，不得返回 plaintext credential、ciphertext、API key hash、raw prompt、raw response、raw repair payload 或 DB/Redis internal error。

## 6. Portal Web BFF

MVP 可以使用 API key login，但 API key 只用于交换服务端 session：

```text
POST /api/portal/v1/auth/api-key-login
POST /api/portal/v1/auth/logout
GET  /api/portal/v1/auth/me

GET  /api/portal/v1/dashboard
GET  /api/portal/v1/onboarding
GET  /api/portal/v1/models
GET  /api/portal/v1/models/{model}/schema
GET  /api/portal/v1/credits
GET  /api/portal/v1/usage
GET  /api/portal/v1/api-keys
POST /api/portal/v1/api-keys
POST /api/portal/v1/api-keys/{key_id}/disable
GET  /api/portal/v1/tasks
GET  /api/portal/v1/tasks/{task_id}
GET  /api/portal/v1/settings/project
```

登录后服务端创建 Portal session，包含 tenant_id、project_id、api_key_id、allowed_models、expires_at、csrf secret 等。浏览器只保存 HttpOnly Secure SameSite cookie，不在 localStorage/sessionStorage 保存 API key。

所有 Portal BFF 方法必须 tenant/project scoped。跨 tenant/project 查询返回 not found 或 forbidden，不暴露资源存在性。

## 7. Admin Web BFF

Admin BFF 负责 operator UI workflow：

```text
POST /api/admin/v1/auth/login
POST /api/admin/v1/auth/logout
GET  /api/admin/v1/auth/me

GET  /api/admin/v1/dashboard
GET  /api/admin/v1/tenants
POST /api/admin/v1/tenants
GET  /api/admin/v1/projects
POST /api/admin/v1/projects
GET  /api/admin/v1/api-keys
POST /api/admin/v1/api-keys
GET  /api/admin/v1/models
POST /api/admin/v1/models
GET  /api/admin/v1/channels
POST /api/admin/v1/channels
GET  /api/admin/v1/routes
POST /api/admin/v1/routes
GET  /api/admin/v1/pricing
POST /api/admin/v1/pricing
GET  /api/admin/v1/limits
POST /api/admin/v1/limits
GET  /api/admin/v1/snapshots
POST /api/admin/v1/snapshots/validate
POST /api/admin/v1/snapshots/publish
POST /api/admin/v1/snapshots/rollback
GET  /api/admin/v1/operations/settlements
POST /api/admin/v1/operations/settlements/{id}/replay
GET  /api/admin/v1/operations/callbacks
POST /api/admin/v1/operations/callbacks/{id}/retry
GET  /api/admin/v1/operations/workers
GET  /api/admin/v1/operations/holds
GET  /api/admin/v1/audit
GET  /api/admin/v1/operators
POST /api/admin/v1/operators
```

角色建议：

```text
super_admin
config_admin
finance_admin
support
ops
read_only
```

权限使用 action/resource，例如 `tenant:read`、`channel:write`、`snapshot:publish`、`settlement:replay`、`audit:read`、`operator:write`。

所有 Admin mutation 必须包含 actor、permission、request_id、Idempotency-Key、reason、diff 或 dry-run 信息，并写 durable audit event。写操作必须调用 owner service：

```text
adminhttp
  -> adminservice authorize/validate/audit intent
  -> owner domain service
       config write       -> controlplane/admin.Service
       snapshot publish   -> controlplane/snapshot.Publisher
       settlement replay  -> billing repair service or worker trigger
       callback retry     -> task/callback service
  -> audit store
  -> safe response
```

## 8. Session、CSRF、CORS

Cookie-session mutation 必须检查 CSRF：

```text
Cookie: tg_console_session=...
X-CSRF-Token: ...
```

GET 不改变状态。POST、PUT、PATCH、DELETE 必须检查 CSRF。Bearer API key 的 `/v1/*` 和 `/v1/portal/*` 不使用 CSRF。

生产推荐同域部署 console。如果跨域，必须 origin allowlist、credentials true，禁止 `*`，并禁止浏览器访问 `/admin/*` machine control API。

## 9. OpenAPI 与 TypeScript Client

推荐把合同逐步迁移到 `api/openapi/`：

```text
api/openapi/gateway-public.yaml
api/openapi/portal-public.yaml
api/openapi/portal-bff.yaml
api/openapi/admin-bff.yaml
api/openapi/control.yaml
```

短期保留 `docs/design/ai_gateway_openapi.yaml` 作为兼容入口，可以在后续变成 aggregate 或镜像。

规则：

1. 新增或修改 API 先改 OpenAPI。
2. Handler contract test 校验 path、method、schema、security。
3. Frontend 只使用生成类型或薄 fetch wrapper。
4. Handler 改了而 OpenAPI 没改，CI 失败。
5. OpenAPI 改了而 generated client 没更新，CI 失败。

前端建议使用 `openapi-typescript` 生成 types，再通过 `web/packages/api-client` 提供 fetch wrapper，统一 session、CSRF、错误归一化和 request_id 展示。

## 10. Frontend Monorepo

推荐技术栈：

```text
TypeScript
React
Vite
pnpm workspace
openapi-typescript
TanStack Query or equivalent server-state query layer
```

Portal/Admin 是登录后台型 SPA，不需要一开始引入 SSR。除非后续有 SEO、多租户主题服务端渲染、边缘缓存或复杂首屏要求，否则 Vite 静态构建更适合当前边界。

依赖规则：

```text
web/apps/portal -> web/packages/api-client
web/apps/portal -> web/packages/ui
web/apps/portal -> web/packages/auth
web/apps/portal -> web/packages/format

web/apps/admin -> web/packages/api-client
web/apps/admin -> web/packages/ui
web/apps/admin -> web/packages/auth
web/apps/admin -> web/packages/format
```

禁止：

```text
web/apps/portal -> web/apps/admin
web/apps/admin -> web/apps/portal
web/packages/ui -> web/apps/*
web/packages/api-client -> web/apps/*
```

`web/packages/ui` 只放纯 UI：Button、Input、Select、Dialog、ConfirmDialog、DataTable、StatusBadge、SecretInput、DateRangePicker、EmptyState、ErrorState、LoadingState、Toast。不要放 `TenantTable`、`ChannelEditor`、`UsageChart` 这类业务组件。

## 11. 静态资源与部署

生产优先选择 CDN/Nginx 托管静态资源，`cmd/console` 承载 BFF：

```text
/portal/*       -> Portal dist
/admin-ui/*     -> Admin dist
/api/*          -> cmd/console
```

简化部署可以让 `cmd/console` serve static dist，但必须设置缓存：

- HTML: `Cache-Control: no-cache`
- hashed JS/CSS: `Cache-Control: public, max-age=31536000, immutable`
- Portal Vite base path: `/portal/`
- Admin Vite base path: `/admin-ui/`

不要让 Admin SPA 占用 `/admin/*`，避免与 machine control API 冲突。

## 12. 测试策略

后端必须覆盖：

- Portal API key login/logout/me。
- Portal session 不泄漏 API key。
- Portal derived key 不能扩大 allowed models。
- Portal dashboard、usage、task tenant/project scoped。
- Admin login/session/logout。
- Admin RBAC deny。
- Admin mutation audit event。
- Admin credential response redaction。
- Admin snapshot publish 调用既有 publisher。
- `/admin/*` machine API 行为不变。
- `/v1/portal/*` public API 行为不变。

前端至少覆盖：

- routing smoke。
- auth guard。
- dashboard empty state。
- table pagination。
- form validation。
- API error state。
- permission denied state。

E2E smoke：

```text
Portal: login -> dashboard -> create derived key -> usage -> tasks -> logout
Admin: login -> dashboard -> list config -> validate snapshot -> publish snapshot -> audit -> logout
```

## 13. 文件大小与拆分阈值

| 文件类型 | 阈值 | 动作 |
|---|---:|---|
| HTTP handler | 250 行 | 按 resource 拆 |
| Service use case | 300 行 | 按 use case 拆 |
| Repository SQL file | 400 行 | 按 page/read model 拆 |
| Interface | 8-10 methods | 拆成小 port |
| React component | 200 行 | 拆 components/hooks |
| Feature page | 300 行 | 拆 sections |
| Frontend API module | 150 行 | 按 resource 拆 |

## 14. 后续阶段映射

```text
P19 Console Monorepo Foundation
  cmd/console, OpenAPI split, frontend workspace, generated clients, CI baseline

P20 Portal Web BFF
  internal/app/portal migration, portalwebhttp, Portal session, Portal UI, portal smoke

P21 Admin Web BFF
  internal/app/admin, operator/session/RBAC/audit, adminhttp, safe read/write workflows

P22 Console Frontend Production
  Admin UI, static asset strategy, E2E, CSP/security headers, deployment and rollback runbook
```

P19-P22 是后续新增产品面，不改变 P8 作为“Portal Public API 第一版”的历史完成范围。
