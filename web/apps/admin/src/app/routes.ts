export type AdminRouteID =
  | "workbench"
  | "catalog"
  | "routing"
  | "accounts"
  | "activity"
  | "tools"
  | "settings";

export interface AdminRouteSection {
  id: AdminRouteID;
  label: string;
  href: string;
  routePrefix: string;
  owner: string;
  purpose: string;
  pages: string[];
  nextTask: string;
}

export const adminRouteSections: AdminRouteSection[] = [
  {
    id: "workbench",
    label: "工作台",
    href: "#workbench",
    routePrefix: "/admin-ui/workbench",
    owner: "运营视图",
    purpose: "汇总渠道健康、请求量、额度风险和待处理事项，不承载复杂配置。",
    pages: ["总览", "异常提醒", "发布证据"],
    nextTask: "P24/E34-T07"
  },
  {
    id: "catalog",
    label: "模型目录",
    href: "#catalog",
    routePrefix: "/admin-ui/catalog",
    owner: "模型视图",
    purpose: "展示模型元数据、能力、结构定义、价格摘要和渠道覆盖。",
    pages: ["模型列表", "模型详情", "门户预览"],
    nextTask: "P24/E34-T04"
  },
  {
    id: "routing",
    label: "渠道路由",
    href: "#routing",
    routePrefix: "/admin-ui/routing",
    owner: "渠道视图",
    purpose: "管理渠道、凭证轮换、测试、同步预览和路由策略提示。",
    pages: ["渠道列表", "渠道详情", "测试与同步", "健康事件"],
    nextTask: "P24/E34-T03"
  },
  {
    id: "accounts",
    label: "客户账户",
    href: "#accounts",
    routePrefix: "/admin-ui/accounts",
    owner: "客户视图",
    purpose: "按租户和项目管理客户状态、角色、额度、密钥和审计。",
    pages: ["客户列表", "账户详情", "额度调整", "密钥"],
    nextTask: "P24/E34-T05"
  },
  {
    id: "activity",
    label: "活动日志",
    href: "#activity",
    routePrefix: "/admin-ui/activity",
    owner: "运营只读",
    purpose: "查看用量、任务和审计详情，只返回脱敏后的安全元数据。",
    pages: ["用量日志", "任务日志", "审计日志", "导出"],
    nextTask: "P24/E34-T07"
  },
  {
    id: "tools",
    label: "调试工具",
    href: "#tools",
    routePrefix: "/admin-ui/tools",
    owner: "安全测试",
    purpose: "提供结构定义驱动的操练场和渠道测试复用入口。",
    pages: ["操练场", "渠道测试", "请求模板"],
    nextTask: "P24/E34-T08"
  },
  {
    id: "settings",
    label: "基础设置",
    href: "#settings",
    routePrefix: "/admin-ui/settings",
    owner: "最小设置",
    purpose: "仅保留会话、安全头、发布限制和已启用能力说明。",
    pages: ["安全边界", "发布限制", "本地化"],
    nextTask: "P24/E34-T11"
  }
];

export const adminCutScopeItems = [
  "用户、模型、渠道分组",
  "非组件化计费配置",
  "订阅套餐",
  "兑换码",
  "第三方支付配置",
  "邀请返利",
  "模型部署服务",
  "大而全系统设置"
];
