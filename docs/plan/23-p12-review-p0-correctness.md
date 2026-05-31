# P12 Review P0 正确性收敛

## 阶段目标

P12 的目标是按 2026-05-31 静态 review 结果，优先修复会直接影响商用收费、余额 hold、并发限流和异步任务结算的 P0 级正确性问题。

本阶段不新增产品 endpoint，不扩展 provider 能力，也不继续堆功能。所有工作围绕一个目标：任何可收费请求都必须满足 hold、usage、ledger、settlement、replay、stream close 和 idempotency 的一致性不变量。

## 交付物

- Streaming 请求的 request/provider concurrency lease 改为在 stream close 后释放。
- Async idempotency replay 命中已有任务时，不新增 provider request，不遗留新的 balance hold。
- Async provider submit 直接返回 terminal 状态时，必须先结算或记录失败修复，再完成任务状态推进。
- Zero-price、non-billable 和 no-hold settlement 路径语义明确，并能写入 usage audit 或 0 金额 ledger。
- Hold 生命周期回归测试覆盖 settled、released、failed settlement、idempotency replay 和 reaper 前状态。
- Stream concurrency、async task、zero-price settlement 的单元测试和 MySQL/Redis 集成测试可重复执行。

## 核心实现顺序

1. 梳理 `GatewayEngine.Handle`、limit acquire/release、dispatcher attempt release 和 `AccountingStream.Close()` 的 ownership，确认 stream 返回后仍需保留的 release handle。
2. 将 streaming 的 request-level 和 attempt-level concurrency release 交给 `AccountingStream` 或等价 close finalizer，确保 close 后统一释放。
3. 调整 async task idempotency path，让 `TaskBridge.CreateAndDispatch` 或等价 service 明确返回 idempotency hit，并在 engine/admission 层释放本次新建 hold。
4. 修正 async provider submit terminal 分支，复用 poller 的 `SettleTerminalTask` -> `CompleteTask` 顺序；结算失败时进入 failed settlement 或任务失败修复状态。
5. 重构 settlement plan 的 zero-price/no-hold 语义，明确 `RequiresHold` 或 zero-amount hold 策略。
6. 为 stream lease、async replay hold、terminal submit settlement、zero-price/no-hold settlement 增加 focused tests。
7. 增加 MySQL/Redis 集成测试或现有 compose smoke 扩展，验证商业账务不变量在真实依赖下成立。

## 关键设计约束

- Stream accounting 的最终入口仍是 stream close；provider 已输出 token 后不得透明 fallback。
- Concurrency lease 的释放必须与真实请求占用时长一致，不能在 engine 返回 stream response 时提前释放。
- Admission 创建的任何 hold 最终只能进入 settled、released 或 failed settlement repair，不允许长期无主 active hold。
- Idempotency replay 不应新扣费、新 hold、新发 provider request，也不应改变已有 task 的计费锚点。
- Provider submit terminal result 与 poll terminal result 必须共享同一套结算语义。
- Zero-price 或 non-billable 不代表没有审计记录；usage audit、billable reason 和 ledger 语义必须可解释。
- P12 只修正确性和测试，不引入新的价格体系、对象存储、RBAC、Realtime 或动态插件。

## 验收标准

- 慢速或未结束 stream 返回给 handler 后，Redis concurrency lease 仍存在；stream close 后 lease 消失。
- Async idempotency 并发 replay 只保留一个有效 hold，不产生第二次 provider submit。
- Provider submit 直接返回 succeeded/failed/canceled 时，任务终态、usage record、ledger entry、hold 状态和 failed settlement 行为与 poller 路径一致。
- Free model、0 价格规则和 non-billable policy 不会因为空 hold 导致 settlement 失败。
- 所有 billable request 的 hold 最终可通过测试证明 settled、released 或进入 repair。
- `go test` focused packages、MySQL/Redis 相关集成测试和 `git diff --check` 通过。

## 风险与处理

| 风险 | 处理 |
|---|---|
| stream close 路径释放失败导致 lease 泄漏 | release handle 必须幂等，并在 close、read error、client disconnect 和 settlement failure 路径都执行 |
| idempotency replay 与 admission ownership 不清 | 让 replay hit 成为显式返回值，engine 统一处理本次 hold release |
| terminal submit 与 poller 结算逻辑分叉 | 抽出共享 settlement helper 或在 bridge 中严格复用 poller 顺序 |
| zero-price 语义被误用成免费绕账 | 明确 non-billable reason，仍写 usage audit 或 0 金额 ledger |
| 集成测试依赖不稳定 | 先写 focused 单测，再把真实依赖验证接入现有 compose/RC smoke |

## 设计来源

- [路线图](./00-roadmap.md)
- [M2 账务闭环](./03-m2-billing-loop.md)
- [M3 Streaming + Native Compatible](./04-m3-protocols-streaming.md)
- [M4 Unified Media Async Task](./05-m4-media-tasks.md)
- [P3 Production Hardening](./14-p3-production-hardening.md)
- [P6 Provider Reliability](./17-p6-provider-reliability.md)
- [任务清单](../tasks.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [代码蓝图](../design/ai_gateway_code_blueprint.md)
