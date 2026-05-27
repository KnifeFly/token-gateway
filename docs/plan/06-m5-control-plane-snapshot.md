# M5 控制面与快照

## 阶段目标

实现不发版新增模型、渠道、价格、路由、限流和插件绑定。控制面写配置，数据面只消费经过校验和发布的 runtime snapshot。

## 交付物

- Admin auth、Tenant API、APIKey API、Model API、Provider API、Route API、Price API、Limit API、Snapshot API 和 Audit API。
- Provider credential encryption 和 rotation 基础能力。
- Snapshot builder、validator、publisher、watcher、rollback 和 gateway cache 热加载。
- snapshot version、staleness、publish errors 相关 metrics。

## 核心实现顺序

1. 建立 admin/control API 和 RBAC 或 admin token 中间件。
2. 实现租户、项目、API key、模型、provider channel、route policy、price rule 和 limit rule 配置 API。
3. 配置写入 DB 后触发 snapshot build。
4. Snapshot validator 拒绝坏配置。
5. Snapshot publisher 发布 active version 到 Redis 或 configd。
6. gateway watcher 热加载并原子替换 IndexedSnapshot。
7. 支持 rollback 到 previous active snapshot。

## 关键设计约束

- 数据面请求期间使用 pinned snapshot、price 和 route decision。
- 控制面配置变更不能让数据面实时查管理表。
- provider credential 不以明文进入 snapshot、日志、metrics 或 trace。
- snapshot 发布必须有 audit 记录和可观察 version。

## 验收标准

- 新增模型无需重启 gateway。
- 新增 channel 无需重启 gateway。
- 修改价格后新请求生效。
- 旧请求结算使用请求时 pinned price。
- 发布坏 snapshot 被 validator 拒绝。
- snapshot version 出现在 metrics 和诊断 header。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 控制面绕过 validator | snapshot 发布只允许从 validator 成功结果进入 active |
| 配置更新影响进行中请求 | 请求 state pin 住 snapshot version 和 price |
| credential 泄露 | snapshot 只携带 CredentialRef，解密有审计 |

## 设计来源

- [实施计划 M5](../design/ai_gateway_implementation_plan_v2.md)
- [架构设计 Snapshot 架构](../design/ai_gateway_architecture_design_v2.md)
- [系统设计多租户和安全](../design/ai_gateway_system_design_v2.md)
- [任务清单 Epic 13](../design/ai_gateway_task_list_v2.md)
