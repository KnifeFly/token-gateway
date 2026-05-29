# P1 设计能力补齐

## 阶段目标

在 P0 生产闭环稳定后，补齐设计文档中尚未完整落地的核心能力。重点是把 MVP 级实现升级为可配置、可观测、可测试的多维限流、策略路由、显式 policy stage 和统一 model catalog。

## 交付物

- Redis Lua 原子限流，支持 RPM/QPS、TPM、并发、日预算和分钟成本预算。
- 限流维度支持 tenant、project、api key、model、provider 和 channel。
- Local deny cache，避免持续被拒请求反复击穿 Redis。
- Routing strategy registry，保留 priority/weighted，并新增 health weighted、least cost、least latency 和 quota aware。
- 统一 `RouteSignals`，承载健康度、延迟、成本、剩余额度、模型兼容性和 provider/channel 状态。
- Data Plane 主链路显式加入 policy evaluation 阶段。
- Model catalog/schema/alias 作为模型列表、模型 schema、模型权限、provider mapping 和 snapshot 的统一事实源。

## 核心实现顺序

1. 固定 limit rule 的数据结构和运行时索引，明确 tenant、project、api key、model、provider、channel 的优先级和组合方式。
2. 用 Redis Lua 实现 RPM/QPS、TPM、concurrency、daily budget 和 cost-per-minute 的原子判断和计数。
3. 在限流拒绝路径加入 local deny cache，并记录拒绝原因、维度和过期时间。
4. 抽象 routing strategy registry，让现有 priority/weighted 成为默认策略实现。
5. 引入 `RouteSignals`，统一输入健康、延迟、成本、额度、模型兼容性和 emergency disable 状态。
6. 实现 health weighted、least cost、least latency 和 quota aware 策略。
7. 在 GatewayEngine 热路径中增加显式 policy evaluation 阶段，输出 allow、deny、degrade 或 route override decision。
8. 将模型列表、schema、alias、ACL 和 provider mapping 收敛到 model catalog，并通过 runtime snapshot 发布给数据面。
9. 将 `/v1/models` 和 `/v1/models/{model}/schema` 改为读取同一套 catalog 数据。

## 关键设计约束

- 多副本限流不能依赖本地内存作为事实源；Redis Lua 是跨副本原子判断入口。
- Local deny cache 只能缓存拒绝结果，不能作为额度事实源。
- 路由策略不能查询控制面管理表，只能读取 runtime snapshot、热数据和可观测信号。
- Policy stage 必须在主链路中显式可读，保持 auth、classify、policy、route、dispatch、settlement 顺序清晰。
- Model catalog 是模型公开能力、schema、alias、ACL、provider mapping 和 runtime snapshot 的统一事实源。
- Semantic routing 不进入 P1，避免将能力补齐阶段扩大成向量检索或缓存系统建设。

## 验收标准

- Redis 集成测试覆盖 RPM/QPS、TPM、concurrency、daily budget 和 cost-per-minute。
- 限流测试覆盖 tenant、project、api key、model、provider、channel 组合维度。
- Local deny cache 命中时不会访问 Redis，并且过期后恢复 Redis 判断。
- Priority/weighted 旧行为保持兼容。
- Health weighted、least cost、least latency 和 quota aware 策略有确定性单元测试。
- Policy stage 可返回 allow、deny、degrade 和 route override，并有主链路测试覆盖。
- Model alias、schema、ACL 和 provider mapping 通过 snapshot 后在数据面一致读取。
- `/v1/models` 和 `/v1/models/{model}/schema` 的返回来自 model catalog，而不是散落配置。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 限流维度过多导致规则不可解释 | P1 固定六个核心维度，user/ip/custom label 后续再扩展 |
| Redis Lua 规则复杂导致难以排障 | 为每次拒绝返回稳定 reason、scope、limit 和 reset 时间 |
| 路由策略互相覆盖 | 使用 strategy registry 和统一 decision 输出，避免在多个包里分散判断 |
| Policy 与 plugin 职责重叠 | Policy 负责核心决策，plugin 负责配置驱动的扩展检查和副作用 |
| Model catalog 迁移影响已有路由 | 先保持现有模型映射兼容，再逐步收敛读口径 |

## 设计来源

- [路线图](./00-roadmap.md)
- [M1 最小数据面](./02-m1-minimal-dataplane.md)
- [M5 控制面与 Runtime Snapshot](./06-m5-control-plane-snapshot.md)
- [M6 插件与安全治理](./07-m6-plugins-security.md)
- [M9 商业化运营能力](./10-m9-commercial-ops.md)
- [实施计划](../design/ai_gateway_implementation_plan.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
- [任务清单](../tasks.md)
