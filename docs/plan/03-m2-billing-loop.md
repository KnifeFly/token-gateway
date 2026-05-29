# M2 计费闭环

## 阶段目标

让 API 转售具备最小商业账务闭环。Provider 成功后本地结算失败必须可修复，所有扣费都能追溯到 usage record 和 ledger entry。

## 交付物

- balances、balance_holds、usage_attempts、usage_records、ledger_entries、failed_settlements 表和 repository。
- PriceEstimator、AdmissionController 和 Balance service。
- Settlement planner、Settlement executor、UsageAttempt writer、UsageRecord writer 和 Ledger service。
- FailedSettlement service 和 replay worker。
- settlement 相关 metrics、trace、access log 字段和 audit。

## 核心实现顺序

1. 完成 billing migration 和事务 repository。
2. 请求前按 usage estimate 估价，检查余额并创建 balance hold。
3. 每次 provider attempt 写 usage_attempt。
4. provider 返回后解析真实 usage 并生成 settlement plan。
5. 在事务中写 usage_record、ledger_entry、更新 balance 和 hold。
6. settlement 失败时写 failed_settlement，worker 后台重放。
7. 增加重复 request_id 的幂等保护。

## 关键设计约束

- 金额使用 micros 或明确 money 类型，不能用 float 表示钱。
- settlement 必须幂等、事务化、可重放。
- Provider 未调用前失败释放 hold。
- Provider 调用失败记录 failed attempt 并释放 hold。
- Provider 成功但本地 settlement 失败时保留可修复状态。

## 验收标准

- 重复 request_id 不重复扣费。
- provider 成功但 DB settlement 故障后写 failed_settlement。
- worker 恢复后能 replay 成功。
- ledger sum 与 balance 一致。
- 所有扣费都有 usage_record 和 ledger_entry。
- failed settlement backlog 有 metrics。

## 风险与处理

| 风险 | 处理 |
|---|---|
| usage attempt 和最终扣费混在一起 | attempt 记录 provider 尝试，usage_record 记录最终客户用量 |
| 事务边界不清 | settlement executor 单独负责事务扣费 |
| 失败后无法修复 | 所有失败写 failed_settlement 并保留重放输入 |

## 设计来源

- [实施计划 M2](../design/ai_gateway_implementation_plan.md)
- [系统设计计费能力](../design/ai_gateway_system_design.md)
- [架构设计账务架构](../design/ai_gateway_architecture_design.md)
- [任务清单 M2](../tasks.md)
