# P24 Console Route Plan

本文记录 P24 `P24/E34-T02` 的 Admin/Portal 信息架构、前端路由计划和中文/i18n 约束。后续 P24 业务任务应在本路线图下补页面和 BFF 能力，不回退到 NewAPI 菜单结构。

## Route Principles

- 前端所有面向用户的静态文案默认使用中文。
- 中文文案集中在各 app 的 `shared/i18n/zh-CN.ts`，组件只消费 copy 和展示映射，后续新增 locale 时替换 i18n 入口，不改组件结构。
- Browser Admin 只访问 `/api/admin/v1/*`，Portal 只访问 `/api/portal/v1/*`。
- 前端路由使用 token-gateway 自己的信息架构，不复制 NewAPI 菜单。
- 路由、页面和 DTO 不增加用户分组、模型分组、渠道分组、倍率、订阅、兑换码、支付、邀请返利、模型部署和大而全系统设置。

## Admin IA

| 区域 | SPA 路由前缀 | 页面 | 后续任务 |
|---|---|---|---|
| 工作台 | `/admin-ui/workbench` | 总览、异常提醒、发布证据 | P24/E34-T07 |
| 模型目录 | `/admin-ui/catalog` | 模型列表、模型详情、门户预览 | P24/E34-T04 |
| 渠道路由 | `/admin-ui/routing` | 渠道列表、渠道详情、测试与同步、健康事件 | P24/E34-T03 |
| 客户账户 | `/admin-ui/accounts` | 客户列表、账户详情、额度调整、密钥 | P24/E34-T05 |
| 活动日志 | `/admin-ui/activity` | 用量日志、任务日志、审计日志、导出 | P24/E34-T07 |
| 调试工具 | `/admin-ui/tools` | 操练场、渠道测试、请求模板 | P24/E34-T08 |
| 基础设置 | `/admin-ui/settings` | 安全边界、发布限制、本地化 | P24/E34-T11 |

## Portal IA

| 区域 | SPA 路由 | 用途 | 后续任务 |
|---|---|---|---|
| 仪表盘 | `/portal/dashboard` | 查看额度、请求量、密钥和任务概况 | P24/E34-T07 |
| 模型 | `/portal/models` | 浏览可用模型、schema 和价格摘要 | P24/E34-T04 |
| 操练场 | `/portal/playground` | 按模型 schema 生成测试请求，结果不暴露内部密钥 | P24/E34-T08 |
| API 密钥 | `/portal/api-keys` | 创建、禁用和查看项目范围内的派生密钥 | P24/E34-T06 |
| 用量 | `/portal/usage` | 查看请求、模型、状态和额度消耗 | P24/E34-T07 |
| 任务 | `/portal/tasks` | 追踪异步任务和媒体任务状态 | P24/E34-T07 |
| 额度 | `/portal/credits` | 查看余额、已用额度和冻结额度 | P24/E34-T09 |
| 设置 | `/portal/settings` | 查看当前项目、租户和可用模型范围 | P24/E34-T11 |

## I18n Reservation

当前实现先固定 `zh-CN`：

```text
web/apps/admin/src/shared/i18n/zh-CN.ts
web/apps/portal/src/shared/i18n/zh-CN.ts
```

后续扩展语言时，按以下顺序处理：

1. 新增同结构 locale 文件，例如 `en-US.ts`。
2. 在 `shared/i18n/index.ts` 增加 locale 选择器或 provider。
3. 组件继续消费 copy 对象、状态映射函数和路由配置，不直接内联新的语言字符串。
4. API 字段名、JSON schema、模型 ID、任务 ID、request ID 不做翻译，只翻译标签、按钮、说明和枚举展示值。

## Cut Scope Check

后续 P24 任务提交前运行：

```bash
make p24-cut-scope-check
```

该检查覆盖：

- BFF OpenAPI path 不出现 group、ratio、payment、subscription、redemption、invite、deployment 或 broad settings route。
- BFF OpenAPI schema 和 generated client 不出现 NewAPI 分组、倍率、支付、订阅、兑换、邀请返利或模型部署字段。
- Admin/Portal 前端 route config 和 API call 不指向裁剪范围外 route。
- Admin/Portal 前端不新增裁剪范围外 feature 文件。

允许保留“裁剪边界”说明文案，用于提醒后续开发不要把范围外能力加回页面、路由、字段或 DTO。
