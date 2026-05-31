# P13 Review P1 商业账务与安全边界

## 阶段目标

P13 的目标是在 P12 正确性收敛通过后，处理 review 中 P1 级高优先级设计问题：异步任务价格 pin、预算语义、文件/媒体输入资产、SSRF/egress 防护、classifier 顺序和 async fallback。

本阶段强调商业可解释性和安全边界。所有任务都必须让客户账单、provider 成本、任务生命周期、外部 URL 访问和协议分类行为更稳定、更可审计。

## 交付物

- Async task 创建时持久化 price snapshot、currency、route snapshot version 和结算所需价格组件。
- Rate limit 与 spend budget 语义拆分，Redis 预扣只作为 admission/rate guard，真实预算以 settlement/ledger/reconciliation 为准。
- 文件和媒体输入资产语义收敛：要么实现真正 streaming spool/临时对象引用，要么明确只支持 URL passthrough 和有限 inline 输入。
- 统一 `egressguard`，覆盖 file URL、callback URL 和 provider base URL 的 outbound 安全检查。
- Callback 支持 HMAC 签名、重试上限、dead-letter 或等价失败收口，并支持租户级 allowlist。
- Classifier 按设计顺序执行 header、path、model registry、body schema、content-type/accept hint 和 `ambiguous_protocol`。
- Async task submit/fallback 与 sync dispatcher 的 candidate loop、retry eligibility、attempt record 和最终 channel pinning 对齐。

## 核心实现顺序

1. 扩展 task 持久化结构，保存创建时的 price rule、currency、rate、estimated output、route snapshot version 和可审计 price metadata。
2. 将 async settlement 改为读取 task 上的 pinned price，不读取 worker 当前全局价格或新 snapshot 价格。
3. 梳理 `daily_budget_micros`、`cost_per_minute_micros` 和 ledger/reconciliation 的语义，拆分 rate/admission guard 与真实 spend budget。
4. 决定文件/媒体输入资产策略：如果支持 upload，就实现 streaming reader、hash、spool/reference 和 cleanup；如果不支持，就在 API、OpenAPI 和 runbook 中明确 URL passthrough 边界。
5. 实现 `egressguard`，禁止 private/reserved/link-local/loopback/multicast IP、metadata service 和不受信任 scheme，并处理 DNS rebinding 风险。
6. 为 callback 增加签名、allowlist、超时、retry 上限和失败终态记录。
7. 调整 classifier 推断顺序并补表驱动测试，确保设计文档与运行行为一致。
8. 抽象 async dispatcher 或等价 candidate loop，让媒体任务 submit 支持 route candidates、provider/channel disable、attempt record 和 retry/fallback 限制。
9. 增加账务、egress、classifier 和 async fallback 的 focused tests 与 contract tests。

## 关键设计约束

- 已创建 task 的结算价格不受后续控制面价格变更影响。
- Provider cost 与 customer price 可以同构，但 customer settlement 只能使用客户售价。
- Spend budget 不能只依赖 Redis 预扣；最终解释必须能回到 usage record、ledger 和 reconciliation。
- 文件能力仍按 transient/non-storage 输入资产处理；P13 不承诺对象长期存储、下载、生命周期或存储 SLA。
- Outbound URL 访问默认 fail-closed，除非通过明确 allowlist 或安全解析。
- Callback 与 file URL 拉取应使用不同用途的 outbound client 或明确网络策略。
- Async fallback 必须遵守与 sync 一致的可重放、未输出、错误可重试和预算未耗尽约束。

## 验收标准

- Async task 在价格变更前创建、价格变更后完成时，仍按创建时 pinned price 结算。
- Spend budget 相关报表能解释 estimated admission、actual settlement 和 reconciliation 差异。
- File/media endpoint 对大输入不会走全量内存读取；如果选择 URL-only，API 和文档明确拒绝真实 upload 托管语义。
- File URL、callback URL 和 provider base URL 不能访问内网、本机、metadata service 或 DNS rebinding 后的禁止 IP。
- Callback 请求带可验证签名，失败重试到上限后进入可追踪终态。
- Classifier 顺序测试覆盖 header、path、model registry、body schema、content-type/accept 和 ambiguous protocol。
- Async provider 第一个 candidate submit 失败时，在符合规则的情况下可尝试后续 candidate，并记录 attempt/fallback reason。

## 风险与处理

| 风险 | 处理 |
|---|---|
| Task price snapshot 字段膨胀 | 只保存结算必需字段和审计 metadata，展示字段仍归 model catalog |
| Redis budget 语义调整影响既有限流 | 保留 admission guard，但改名或文档明确其不是最终真实花费 |
| Streaming upload 扩大成对象存储产品 | 如果没有对象存储投入，明确 URL passthrough，不假装支持托管 |
| Egress guard 误伤合法企业内网集成 | 通过租户级 allowlist 和部署级网络策略显式放行 |
| Async fallback 造成重复任务 | 只有 provider submit 未成功建立外部任务时才 fallback；已返回 external task ID 后必须 pin channel |

## 设计来源

- [路线图](./00-roadmap.md)
- [M4 Unified Media Async Task](./05-m4-media-tasks.md)
- [M5 控制面与 Runtime Snapshot](./06-m5-control-plane-snapshot.md)
- [P1 设计能力补齐](./12-p1-design-capabilities.md)
- [P7 Media Forwarding Providers](./18-p7-media-forwarding-providers.md)
- [P11 模型价格目录增强](./22-p11-model-pricing-catalog.md)
- [任务清单](../tasks.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
