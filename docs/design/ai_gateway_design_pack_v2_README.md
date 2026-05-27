# 商用 AI Gateway 设计包 v0.2 索引

建议阅读顺序：

1. `ai_gateway_system_design_v2.md`：先理解产品定位、协议风格、核心能力和商用边界。
2. `ai_gateway_architecture_design_v2.md`：看整体架构、分层、目录、核心模块和运行机制。
3. `ai_gateway_code_blueprint_v2.md`：直接按 package、文件、接口、结构体开始编码。
4. `ai_gateway_implementation_plan_v2.md`：按 M0-M8 阶段推进。
5. `ai_gateway_task_list_v2.md`：把 Epic/Task 导入项目管理工具。
6. `ai_gateway_openapi_v2.yaml`：导入 Apifox，作为对外 API 草案。

关键设计原则：

- 对外 URI 按能力命名，不按供应商命名。
- `model` 是路由入口，不只是字符串。
- 数据面只读 runtime snapshot，不实时查管理表。
- Provider adapter 只做协议翻译和调用，不做计费和路由。
- 插件链从第一版就设计，但先做内置插件，不急着做动态插件市场。
- 计费必须有预占、结算、ledger、失败修复和对账。
- 流式响应必须在 stream close 时做最终 accounting。
- 所有敏感信息默认不进日志、metrics label、trace attribute。
