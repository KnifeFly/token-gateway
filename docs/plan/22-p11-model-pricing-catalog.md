# P11 模型价格目录增强

## 阶段目标

P11 的目标是在 P10 发布交接收口之后，补齐模型分类、复杂价格体系、渠道成本和模型目录运营能力。先做底层能力，不新增 Web 控制台、用户账号、支付、New API 分组或新的模型广场公开接口。

本阶段把当前以 input/output token 为主的价格能力升级为 category 驱动的多币种组件化价格簿，并保持客户售价、provider 成本、账务结算和运营展示之间的边界清晰。

## 交付物

- 模型 category 体系：固定 chat、embedding、rerank、image、video、audio_speech、audio_transcription、music、moderation 和 realtime_reserved 等产品分类。
- 分类价格模板：每个 category 定义允许的价格单位、展示单位和默认估算维度。
- 客户售价组件化：支持 token、cache、reasoning、audio/image/video token、request、image、audio_second、video_second 和 task 等组件。
- Provider 成本组件化：与客户售价同构，但独立存储、独立权限、独立用途。
- 统一报价器：balance hold、settlement、failed replay 和 async task settlement 使用同一套客户售价报价逻辑。
- 模型目录展示字段：补齐 tags、category、provider family、modalities、capabilities、context window、max output、status、deprecated、sort order 和 metadata。
- 渠道模型 metadata：记录渠道模型能力覆盖、支持参数、健康/测试状态和成本配置状态。
- 渠道测试和上游模型同步 preview：支持后台测试与模型差异预览，暂不要求控制台。

## 核心实现顺序

1. 定义模型 category 枚举、默认 category 元数据和 category 到 price template 的映射。
2. 定义组件化价格单位、展示 scale、币种和校验规则，确保模型价格只能使用所属 category 允许的单位。
3. 扩展客户售价配置，保留旧 input/output token 字段兼容，并把旧字段规范化为 price components。
4. 实现统一报价器，按 usage/meter 与 price components 计算 hold 和最终 settlement 金额。
5. 将 balance hold、SettlementPlanner、failed settlement replay 和 async task settlement 切到统一报价器。
6. 扩展 provider cost profile 为组件化成本，保持 provider cost 不参与客户扣费。
7. 增强 model catalog 字段和 runtime snapshot，让模型展示、筛选、schema 和后续模型广场读取同一事实源。
8. 增强渠道模型 metadata，记录 public model 到 upstream model 的能力、测试和成本配置状态。
9. 增加渠道测试和上游模型同步 preview 的后台 service/CLI，并在 snapshot 发布前输出稳定 warning/error。
10. 同步 OpenAPI、runbook、focused tests 和兼容测试。

## 关键设计约束

- 金额存储必须使用 currency + micros 整数，不允许使用 float 表达金额。
- 前端展示使用 USD、CNY 等真实币种和人可读单位，例如 `$0.15 / 1M input tokens` 或 `¥0.8 / image`。
- Category 是产品分类，不等于 provider type、route strategy 或 New API group。
- 客户售价和 provider 成本可以共用组件 schema，但必须分表、分用途、分权限。
- 客户扣费只能读取客户售价；provider 成本只能用于利润报表、least-cost 信号和运营分析。
- 数据面热路径仍只读 runtime snapshot 和索引，不实时查控制面管理表。
- `/v1/models` 暂不强制展示价格，避免破坏兼容；模型广场/定价只读接口后续再补。
- P11 不引入 Web 控制台、用户账号体系、支付、订阅套餐、复杂发票、对象存储、完整 Realtime 或 New API 分组。

## 验收标准

- 模型 category、分类价格模板和价格单位校验有确定性单元测试。
- 旧 input/output token 价格配置能兼容读写，并规范化为组件化价格。
- Chat、image、video、audio task 的 hold、settlement、ledger、failed replay 金额一致。
- Provider cost 与 customer price 分离，利润报表可按 provider/channel/model 聚合成本、收入和利润。
- 缺失 provider cost 时只标记运营 warning，不影响客户扣费。
- Model catalog 可发布 category、tags、modalities、capabilities、status 和 metadata 到 runtime snapshot。
- 渠道模型映射仍支持 public model 到 upstream model 名称不一致，并可记录测试状态和成本配置状态。
- 渠道测试和上游模型同步 preview 能输出新增、删除、变更、无法识别 category 和缺价格/成本配置的结果。
- `go test ./internal/domain/pricing ./internal/billing ./internal/controlplane ./internal/dataplane/snapshot ./internal/portal` 或等价 focused tests 通过。
- `git diff --check` 通过，OpenAPI、计划和任务看板保持一致。

## 实现记录

- 2026-06-01：已落地 category/template/component quoter、旧 token 字段兼容、客户售价到 hold/settlement/async task settlement 的统一报价路径、provider cost component profile、模型目录字段、渠道模型 metadata 和 `/admin/channels/model-sync-preview` 非持久化预览。
- 2026-06-01：已同步 MySQL migration `000016_p11_model_pricing_catalog`、runtime snapshot/index、Portal 模型摘要、OpenAPI 管理合同和 focused regression tests。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 价格单位无限扩张导致后台难以维护 | P11 固定首批 category 和允许单位，新增单位必须先扩展模板 |
| 客户售价与 provider 成本混用 | 类型、存储和权限分离，settlement 只接受客户售价 quote |
| 展示币种和存储精度混淆 | 存储统一 micros，展示层按 currency 和 display scale 格式化 |
| 媒体模型无法用 token 价格表达 | 增加 request、image、audio_second、video_second 和 task 等非 token meter |
| 旧价格配置迁移破坏线上请求 | 保留旧字段兼容，snapshot 构建时规范化为组件价格 |
| 模型目录字段膨胀影响路由 | 展示 metadata 不参与路由，路由仍只使用模型、渠道、route policy 和 snapshot 索引 |
| 上游模型同步误改生产配置 | 同步先只生成 preview，实际 upsert 需要显式后台操作 |

## 设计来源

- [路线图](./00-roadmap.md)
- [P1 设计能力补齐](./12-p1-design-capabilities.md)
- [M9 商业化运营能力](./10-m9-commercial-ops.md)
- [P8 Portal API](./19-p8-portal-api.md)
- [任务清单 P11](../tasks.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
