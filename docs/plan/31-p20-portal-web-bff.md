# P20 Portal Web BFF

## 阶段目标

P20 的目标是在 P19 console foundation 上实现客户浏览器 Portal：新增 Portal Web BFF、Portal session、dashboard/onboarding/read model、Portal UI，并保持既有 `/v1/portal/*` programmatic API 完全兼容。

Portal Web 不是 Admin，也不应该拥有 provider/channel/route/price/limit/snapshot 配置能力。它只服务当前 tenant/project 范围内的 customer self-service。

## 交付物

- 将现有 `internal/portal` 能力迁移或包裹到 `internal/app/portal`，保留 `internal/portal` 兼容 shim，降低一次性 import churn。
- `internal/app/portal/service` 拆分为 auth、dashboard、models、credits、usage、api_keys、tasks、onboarding、sessions 等 use case 文件。
- `internal/app/portal/repository` 拆分为 dashboard、usage、api_keys、tasks、sessions 等 read model/SQL 文件。
- `internal/transport/portalwebhttp` 实现 `/api/portal/v1/*` browser BFF。
- Portal session store：MVP 可 Redis 或 MySQL，必须支持 create/get/delete、expires_at、revoked_at、last_seen_at 和 CSRF secret/hash。
- API key login：customer API key 只用于交换服务端 session，不返回、不落日志、不存浏览器 storage。
- Portal dashboard/onboarding BFF：聚合 credits、usage、active API keys、task 状态、recent usage 和 recent tasks。
- Portal UI：登录、dashboard、models/schema、credits、usage、API keys、tasks、onboarding、settings project。
- Portal OpenAPI：`api/openapi/portal-bff.yaml` 与 generated TS client 同步。
- Portal smoke：浏览器或 CLI 跑通 login、dashboard、create derived key、list keys、usage、tasks、logout。

## 核心实现顺序

1. 审计现有 `internal/portal` 和 `internal/transport/portalhttp`，明确哪些方法属于 public programmatic API，哪些可以复用到 web BFF。
2. 新增 `internal/app/portal` 类型：`Principal`、`Session`、`Dashboard`、`UsageFilter`、`TaskFilter`、`APIKey`、`OnboardingState`。
3. 把现有 Portal use case 迁移到 `internal/app/portal/service`，`internal/portal` 只做兼容转发或在后续阶段删除。
4. 拆分 Portal repository ports：`ModelReader`、`CreditsReader`、`UsageReader`、`APIKeyStore`、`TaskReader`、`DashboardReader`、`SessionStore`。
5. 实现 Portal session：API key login 调用现有 API key authenticator，成功后创建 HttpOnly Secure SameSite cookie 和 CSRF token。
6. 实现 `portalwebhttp` auth middleware：读取 session、校验 expires/revoked、注入 tenant/project/api_key/allowed_models principal。
7. 实现 Portal dashboard 和 onboarding BFF，所有查询都必须 tenant/project scoped，跨 tenant/project 返回 not found 或 forbidden。
8. 实现 Portal API key self-service BFF：派生 key 只能继承当前 tenant/project 和 allowed_models 子集，历史 plaintext key 不可查询。
9. 实现 Portal tasks BFF：只返回当前 tenant/project 下 task，隐藏 provider secret、raw provider error、raw prompt/raw response 和敏感 metadata。
10. 补齐 Portal frontend routes、layout、auth guard、query hooks、empty/error/loading states 和基础表单校验。
11. 更新 OpenAPI、generated client、contract tests、focused tests 和 smoke。

## API Scope

```text
POST   /api/portal/v1/auth/api-key-login
POST   /api/portal/v1/auth/logout
GET    /api/portal/v1/auth/me

GET    /api/portal/v1/dashboard
GET    /api/portal/v1/onboarding

GET    /api/portal/v1/models
GET    /api/portal/v1/models/{model}/schema

GET    /api/portal/v1/credits
GET    /api/portal/v1/usage
GET    /api/portal/v1/usage/export

GET    /api/portal/v1/api-keys
POST   /api/portal/v1/api-keys
POST   /api/portal/v1/api-keys/{key_id}/disable

GET    /api/portal/v1/tasks
GET    /api/portal/v1/tasks/{task_id}
POST   /api/portal/v1/tasks/{task_id}/cancel

GET    /api/portal/v1/settings/project
```

`tasks/{task_id}/cancel` 只有在 existing task domain 明确支持 safe cancel 时才启用；否则 OpenAPI 标记为 not implemented 或不进入本阶段。

## 关键设计约束

- Portal Web BFF 不取代 `/v1/portal/*`，不破坏 P8 programmatic contract。
- API key login 后不得在 browser localStorage/sessionStorage 保存 API key。
- 所有 mutation 都需要 CSRF；Bearer API key 的 `/v1/*` 和 `/v1/portal/*` 不使用 CSRF。
- Portal 不提供 provider channel、route、price、limit、plugin、snapshot、emergency action。
- Portal repository 不直接写 ledger、settlement、task 状态机或 snapshot。
- Portal response 不返回 provider cost、provider credential、raw prompt、raw response、raw repair payload、API key hash。
- Portal UI 只通过 generated client 或 typed API wrapper 调用 BFF。

## 验收标准

- `/v1/portal/*` existing contract tests 继续通过。
- `POST /api/portal/v1/auth/api-key-login` 可用有效 customer API key 创建 session，并且响应和 browser storage 不含 API key。
- 无 session、过期 session、revoked session、CSRF 缺失 mutation 都返回稳定错误。
- Dashboard、usage、API key、task 查询按 tenant/project/api_key scope 过滤，跨 scope 有回归测试。
- 派生 API key 不能扩大 allowed_models，不能查看历史 plaintext key。
- Portal UI 能完成 login、dashboard、models、credits、usage、API keys、tasks、logout 基础流程。
- `pnpm --filter @tg/portal test`、`pnpm --filter @tg/portal build` 或等价命令通过。
- `go test ./internal/app/portal/... ./internal/transport/portalwebhttp ./internal/transport/portalhttp` 或等价 focused tests 通过。
- Portal smoke 能形成可保存的验收证据。

## 风险与处理

| 风险 | 处理 |
|---|---|
| API key 暴露在浏览器 | 只允许 API key exchange session；禁止 localStorage/sessionStorage；日志和错误统一 redaction |
| Portal Web 变成 Admin 子集 | 明确禁止 config/snapshot/provider/price/limit/emergency 能力 |
| session store 与 API key revoke 不一致 | session principal 绑定 api_key_id，me/dashboard 查询时检查 key revoke 或缩短 session TTL |
| dashboard 聚合拖慢请求 | read model SQL 分文件，必要时缓存短 TTL；不进入 gateway hot path |
| 迁移 `internal/portal` 影响 P8 | 保留 shim 和 contract tests，先迁移内部实现再调整 imports |
| task cancel 语义不稳定 | 只有 owner task service 支持 safe cancel 时才开放，否则不在 OpenAPI 声明 |

## 依赖关系

- 依赖 P19 的 `cmd/console`、frontend workspace、OpenAPI generation 和 CI baseline。
- 依赖 P8 的 Portal public API 权限边界和 API key self-service 规则。
- P20 完成后，P21 可以复用 console session/CSRF/static/error writer 基础能力实现 Admin BFF。

## 设计来源

- [P19 Console Monorepo Foundation](./30-p19-console-monorepo-foundation.md)
- [Portal/Admin Console Monorepo 设计](../design/ai_gateway_console_monorepo_design.md)
- [P8 Portal API](./19-p8-portal-api.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [任务清单](../tasks.md)
