# M9 商业化运营能力

## 阶段目标

支持正式商业运营所需的余额、用量、成本、利润、账单、对账、租户报表、模型市场、Agent workflow 维度分析和灾备流程。

## 交付物

- usage report、cost report、profit report 和 tenant dashboard。
- 充值流水、扣费流水、账单导出和发票数据接口。
- 渠道成本配置、模型市场配置和模型维度利润分析。
- Agent workflow、scene、shot metadata report。
- reconciliation service 和异常账务追踪。
- backup/restore runbook 和 disaster recovery drill。

## 核心实现顺序

1. 以 ledger 和 usage_record 为报表事实源。
2. 建立客户余额、用量和扣费流水查询。
3. 建立运营侧渠道成本、模型利润和 provider 成本分析。
4. 建立财务对账、异常账务追踪和账单导出。
5. 建立模型市场配置和租户可见模型能力。
6. 支持 Agent metadata 的 workflow、scene 和 shot 维度报表。
7. 建立备份恢复 runbook，并完成灾备演练。

## 关键设计约束

- 财务报表以 ledger 为准，不直接从 provider attempt 推导扣费。
- 成本、售价和利润要区分 provider cost、customer price 和 settlement amount。
- 对账必须能定位 failed settlement、重复扣费、漏扣费和 callback 不一致。
- 运营报表不得泄露原始 prompt、response 或 provider credential。
- 灾备流程必须说明账务、snapshot、任务和 failed settlement 的恢复顺序。

## 验收标准

- 客户可查余额和用量。
- 运营可查渠道利润。
- 财务可对账。
- 失败账务可追踪。
- 模型维度成本可分析。
- Agent 场景和镜头维度可分析。
- 备份恢复演练通过。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 报表事实源混乱 | ledger、usage_record、cost profile 分工固定 |
| 商业报表影响热路径 | 报表读模型走异步投影或分析库 |
| 对账无法解释差异 | 保存 request_id、attempt_id、ledger_id 和 settlement state 关联 |
| 灾备恢复导致账务二次扣费 | replay 和 manual adjustment 必须幂等并带审计 |

## 设计来源

- [实施计划 M9](../design/ai_gateway_implementation_plan.md)
- [系统设计核心验收标准](../design/ai_gateway_system_design.md)
- [架构设计账务架构](../design/ai_gateway_architecture_design.md)
- [任务清单 M9](../tasks.md)
