export type ViewID =
  | "dashboard"
  | "models"
  | "playground"
  | "api-keys"
  | "usage"
  | "tasks"
  | "credits"
  | "settings";

export interface PortalRouteItem {
  id: ViewID;
  label: string;
  route: string;
  purpose: string;
  nextTask: string;
}

export const navItems: PortalRouteItem[] = [
  {
    id: "dashboard",
    label: "仪表盘",
    route: "/portal/dashboard",
    purpose: "查看额度、请求量、密钥和任务概况。",
    nextTask: "P24/E34-T07"
  },
  {
    id: "models",
    label: "模型",
    route: "/portal/models",
    purpose: "浏览可用模型、结构定义和价格摘要。",
    nextTask: "P24/E34-T04"
  },
  {
    id: "playground",
    label: "操练场",
    route: "/portal/playground",
    purpose: "按模型结构定义生成测试请求，结果不暴露内部密钥。",
    nextTask: "P24/E34-T08"
  },
  {
    id: "api-keys",
    label: "API 密钥",
    route: "/portal/api-keys",
    purpose: "创建、禁用和查看项目范围内的派生密钥。",
    nextTask: "P24/E34-T06"
  },
  {
    id: "usage",
    label: "用量",
    route: "/portal/usage",
    purpose: "查看请求、模型、状态和额度消耗。",
    nextTask: "P24/E34-T07"
  },
  {
    id: "tasks",
    label: "任务",
    route: "/portal/tasks",
    purpose: "追踪异步任务和媒体任务状态。",
    nextTask: "P24/E34-T07"
  },
  {
    id: "credits",
    label: "额度",
    route: "/portal/credits",
    purpose: "查看余额、已用额度和冻结额度。",
    nextTask: "P24/E34-T09"
  },
  {
    id: "settings",
    label: "设置",
    route: "/portal/settings",
    purpose: "查看当前项目、租户和可用模型范围。",
    nextTask: "P24/E34-T11"
  }
];
