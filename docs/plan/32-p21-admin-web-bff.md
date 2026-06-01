# P21 Admin Web BFF

## 阶段目标

P21 的目标是新增 Admin Web BFF 后端能力：operator session、RBAC、CSRF、durable audit、safe read model、operations views，以及受控写 workflow。Admin Web BFF 服务浏览器，不复用 `/admin/*` machine Control API，也不把 control admin token 暴露给前端。

P21 先把后端安全边界立稳，再让 P22 承接完整 Admin UI 和生产化收敛。

## 交付物

- `internal/app/admin` bounded context：`types.go`、`service/`、`repository/`。
- Operator/session/RBAC/audit 数据模型和 migration：`admin_operators`、`admin_sessions`、`admin_audit_events`，权限可先 roles JSON + action/resource。
- `internal/transport/adminhttp`，承载 `/api/admin/v1/*` browser BFF。
- Admin auth：login/logout/me、session cookie、CSRF、session revoke、last_seen_at。
- Admin RBAC middleware 和 service-level authorization。
- Admin dashboard read model：tenant/project/API key/model/channel/task/settlement/callback/worker/hold 汇总。
- Admin config safe read models：tenants、projects、API keys、models、channels、routes、pricing、limits、snapshots。
- Admin operations views：failed settlements、callback deliveries、worker jobs、hold aging。
- Admin write workflows：tenant/project/API key/model/channel/route/pricing/limit/snapshot/operation retry 的最小受控写路径。
- Durable audit：所有 Admin mutation 记录 actor、action、resource、request_id、idempotency_key、reason、status、redacted before/after、IP、UA hash。
- Redaction policy：credential、API key hash、raw prompt/response、raw provider error、repair payload、DB/Redis internal error 不返回前端。
- Admin OpenAPI：`api/openapi/admin-bff.yaml` 与 generated TS client 同步。

## 核心实现顺序

1. 定义 `adminapp.Actor`、`Operator`、`Session`、`Role`、`Permission`、`AuditEvent`、`MutationOptions`。
2. 新增 operator/session/audit migration，选择初始 bootstrap operator 策略：seed、CLI 或 migration fixture，但不得依赖 browser 输入 control admin token 作为长期认证。
3. 实现 `OperatorStore`、`SessionStore`、`AuditStore` repository ports 和 memory/mysql 实现。
4. 实现 Admin login/logout/me 和 session middleware；所有 mutation 必须校验 CSRF。
5. 实现 RBAC：先支持固定 roles + permissions map，保留后续更细粒度 policy 扩展点。
6. 实现 Admin dashboard 和只读列表 read model，先按 UI 查询设计 pagination/filter/sort，避免 handler 手写 SQL。
7. 实现 config write workflow：Admin service 校验权限、构造 audit intent 和 diff，再调用 `controlplane/admin.Service` 或既有 owner service。
8. 实现 snapshot validate/publish/rollback：必须调用 snapshot owner，不直接改 active snapshot 表或 Redis key。
9. 实现 operations workflow：settlement replay 调 billing repair/worker trigger，callback retry 调 task/callback owner，不直接 patch 状态表。
10. 所有 mutation 写 durable audit event，失败也记录 status 和 safe error code。
11. 补齐 OpenAPI、generated client、contract tests、RBAC tests、audit tests 和 control API 回归，确保 `/admin/*` 行为不变。

## API Scope

```text
POST   /api/admin/v1/auth/login
POST   /api/admin/v1/auth/logout
GET    /api/admin/v1/auth/me

GET    /api/admin/v1/dashboard

GET    /api/admin/v1/tenants
POST   /api/admin/v1/tenants
GET    /api/admin/v1/tenants/{tenant_id}

GET    /api/admin/v1/projects
POST   /api/admin/v1/projects

GET    /api/admin/v1/api-keys
POST   /api/admin/v1/api-keys
POST   /api/admin/v1/api-keys/{key_id}/disable

GET    /api/admin/v1/models
POST   /api/admin/v1/models
GET    /api/admin/v1/channels
POST   /api/admin/v1/channels
POST   /api/admin/v1/channels/{channel_id}/test
POST   /api/admin/v1/channels/{channel_id}/disable
POST   /api/admin/v1/channels/{channel_id}/enable

GET    /api/admin/v1/routes
POST   /api/admin/v1/routes

GET    /api/admin/v1/pricing
POST   /api/admin/v1/pricing

GET    /api/admin/v1/limits
POST   /api/admin/v1/limits

GET    /api/admin/v1/snapshots
POST   /api/admin/v1/snapshots/validate
POST   /api/admin/v1/snapshots/publish
POST   /api/admin/v1/snapshots/rollback

GET    /api/admin/v1/operations/settlements
POST   /api/admin/v1/operations/settlements/{id}/replay
GET    /api/admin/v1/operations/callbacks
POST   /api/admin/v1/operations/callbacks/{id}/retry
GET    /api/admin/v1/operations/workers
GET    /api/admin/v1/operations/holds

GET    /api/admin/v1/audit
GET    /api/admin/v1/operators
POST   /api/admin/v1/operators
POST   /api/admin/v1/operators/{operator_id}/disable
```

可以按风险分批开放：read-only dashboard/operations 先行，config write 和 operator management 后行。

## RBAC 建议

| Role | 能力 |
|---|---|
| `super_admin` | 全权限，能管理 operator |
| `config_admin` | 管理 model/channel/route/price/limit/snapshot |
| `finance_admin` | 查看 billing/reporting，执行授权 manual adjustment 或 settlement repair |
| `support` | 只读 tenant/project/task/usage，不能看 secret |
| `ops` | worker/callback/settlement repair |
| `read_only` | 只读 dashboard 和配置 |

权限使用 action/resource，例如 `tenant:read`、`channel:write`、`snapshot:publish`、`settlement:replay`、`callback:retry`、`audit:read`、`operator:write`。

## 关键设计约束

- Admin browser 不调用 `/admin/*`，不持有 control admin token。
- Admin BFF write path 必须调用 owner service，不直接写 control/config、billing、task、worker 状态表。
- 所有 mutation 必须有 actor、permission、request_id、Idempotency-Key、reason、audit。
- Audit event 的 before/after 必须先 redaction，不能存 plaintext credential 或 raw sensitive payload。
- Admin read model 可以为 UI 优化 pagination/filter/count，但不能暴露 secret、hash、ciphertext 或 raw internal error。
- `/admin/*` machine Control API 行为必须保持兼容；P21 不能把它重写成 browser API。

## 验收标准

- Admin login/logout/me、session expiry/revoke、CSRF mutation deny 有 focused tests。
- RBAC deny 覆盖每类高风险 action，403 响应不泄漏资源敏感信息。
- Admin dashboard、config lists、operations lists 可按 filter/pagination 查询，响应字段已 redacted。
- Create/update/disable/publish/replay/retry 等 mutation 调用 owner service，并写 durable audit event。
- Audit 查询能按 operator、action、resource、time range 查询，敏感 diff 已 redacted。
- `/admin/*` controlhttp focused tests 继续通过。
- `go test ./internal/app/admin/... ./internal/transport/adminhttp ./internal/controlplane/admin ./internal/transport/controlhttp` 或等价 focused tests 通过。
- `api/openapi/admin-bff.yaml`、generated client 和 contract tests 一致。

## 风险与处理

| 风险 | 处理 |
|---|---|
| Admin BFF 绕过 owner service | service ports 明确 owner dependency，repository 只做 read model/session/audit，不提供裸写配置方法 |
| RBAC 过度复杂拖慢上线 | 首版固定 roles + action/resource map，后续再接 OIDC/group mapping |
| Audit 写失败导致真实 mutation 已发生 | 高风险 mutation 使用事务或 outbox；无法同事务时至少记录 failed audit repair backlog |
| Redaction 漏字段 | 建立 denylist + type-level safe DTO，contract tests 检查 credential/hash/raw payload 不出现 |
| `/admin/*` 行为被改坏 | P21 所有 Admin Web 新能力走 `adminhttp`，controlhttp 回归测试必跑 |
| Operator bootstrap 不安全 | 采用 seed/CLI/one-time setup token，生产默认 fail closed |

## 依赖关系

- 依赖 P19 的 console BFF skeleton、OpenAPI generation 和 CI。
- 可复用 P20 的 session/CSRF/error writer 基础，但 Admin 必须独立 operator principal。
- P21 完成后解锁 P22 Admin UI、E2E smoke、deployment hardening 和 release handoff。

## 实施记录

- 已新增 `internal/app/admin` bounded context，Admin operator/session/audit 与 `internal/controlplane/admin` machine control/config domain 分离。
- 已实现 `/api/admin/v1/*` browser BFF：login/logout/me、dashboard、config read/write、snapshot validate/publish/rollback、operations read models、audit 和 operator management。
- Admin mutation 统一校验 session、CSRF、RBAC、`Idempotency-Key` 和 `X-Reason`，写入 redacted durable audit；配置写入委托 `controlplane/admin.Service`，snapshot 委托 snapshot publisher，不让 browser 直接调用 `/admin/*`。
- 已新增 MySQL migration `000017_p21_admin_web_bff`，覆盖 `admin_operators`、`admin_sessions`、`admin_audit_events`。
- 已同步 `api/openapi/admin-bff.yaml` 和 generated TypeScript client，并补 focused tests 覆盖 login/session/CSRF、RBAC deny、audit redaction 和 `/admin/*` control API 回归。

## 设计来源

- [P19 Console Monorepo Foundation](./30-p19-console-monorepo-foundation.md)
- [Portal/Admin Console Monorepo 设计](../design/ai_gateway_console_monorepo_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [ADR](../design/ai_gateway_ADR.md)
- [任务清单](../tasks.md)
