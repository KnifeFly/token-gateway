export const adminLocale = "zh-CN" as const;

export const adminCopy = {
  brand: "管理端",
  navigationLabel: "管理端导航",
  workspaceLabel: "管理端工作区",
  topbar: {
    title: "P24 运营管理台",
    subtitle: "浏览器端只访问管理端中转接口，后续页面按这套路由骨架补齐。",
    signInAction: "操作员登录待接入"
  },
  routePlan: {
    title: "信息架构与路由计划",
    generatedAtLabel: "生成时间",
    status: "P24-T02",
    sectionColumn: "区域",
    routeColumn: "路由前缀",
    ownerColumn: "归属",
    nextTaskColumn: "下一任务",
    pagesLabel: "页面",
    guardrailTitle: "裁剪边界",
    guardrailDescription: "以下能力不进入 P24 页面、路由或字段设计。"
  },
  session: {
    signedIn: "已登录",
    signedOut: "未登录"
  }
};

export function adminSessionLabel(authenticated: boolean): string {
  return authenticated ? adminCopy.session.signedIn : adminCopy.session.signedOut;
}
