# M6 插件化与安全治理

## 阶段目标

把请求大小限制、prompt token 限制、PII 脱敏、PromptGuard、ResponseGuard、CostGuard、AuditLog、LLMMetric 和 Callback 变成配置驱动的内置插件。

## 交付物

- Plugin interface、PluginManager、PluginBinding resolver 和 plugin registry。
- RequestSizePlugin、PromptTokenLimitPlugin、PiiRedactionPlugin。
- PromptGuardPlugin、ResponseGuardPlugin、CostGuardPlugin。
- AuditLogPlugin、LLMMetricPlugin 和 CallbackPlugin。
- 插件配置纳入 runtime snapshot。
- 安全事件、审计事件、脱敏测试和 policy denied 错误映射。

## 核心实现顺序

1. 定义 Plugin、Phase、Input、Result 和 Mutation。
2. PluginBinding resolver 按 scope specificity、priority 和 plugin name 排序。
3. 在 GatewayEngine 生命周期中接入必要 phase。
4. 先实现请求大小、prompt token 和 PII 脱敏插件。
5. 再实现 PromptGuard、ResponseGuard、CostGuard、AuditLog、LLMMetric 和 Callback。
6. 为 deny、redact、audit、metrics 和 failure mode 补测试。

## 关键设计约束

- MVP 插件是内置代码加配置绑定，不做动态代码执行。
- 插件错误是否 fail open 必须由配置或阶段语义明确。
- PII 和敏感凭证默认不得进入 access log、audit、metrics label 或 trace attribute。
- 插件可 deny 请求，但必须返回标准错误并写审计线索。

## 验收标准

- 插件可按 tenant、project、model 绑定。
- 插件按 phase 和 priority 执行。
- 插件可 deny 请求。
- 插件可写 audit event。
- PII 不进入 access log。
- PromptGuard 命中后返回 `policy_denied`。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 插件顺序不可解释 | 固定 scope specificity、priority、plugin name 排序 |
| 插件副作用过大 | Result 显式表达 deny、mutation、audit 和 tags |
| 安全日志泄露敏感字段 | 统一 redactor，并对日志和审计做测试 |

## 设计来源

- [实施计划 M6](../design/ai_gateway_implementation_plan_v2.md)
- [系统设计插件体系和安全设计](../design/ai_gateway_system_design_v2.md)
- [架构设计插件架构](../design/ai_gateway_architecture_design_v2.md)
- [代码蓝图插件核心代码骨架](../design/ai_gateway_code_blueprint_v2.md)
- [任务清单 Epic 12 和 Epic 14](../design/ai_gateway_task_list_v2.md)
