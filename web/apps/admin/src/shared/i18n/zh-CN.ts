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
  channels: {
    title: "渠道管理",
    subtitle: "T03 先打通安全读写、凭证轮换、测试、同步预览、健康事件、模型覆盖和路由提示。",
    status: "P24-T03",
    sections: {
      workflow: "工作流",
      coverage: "模型覆盖",
      routeHints: "路由提示",
      health: "健康事件",
      safeFields: "安全字段"
    },
    columns: {
      channel: "渠道",
      provider: "供应商",
      state: "状态",
      credential: "凭证",
      models: "模型数",
      health: "健康",
      test: "测试",
      cost: "成本配置",
      action: "动作"
    },
    actions: {
      create: "创建/编辑",
      rotate: "轮换凭证",
      test: "渠道测试",
      syncPreview: "同步预览",
      syncApply: "同步应用",
      healthEvents: "健康事件"
    },
    state: {
      enabled: "启用",
      disabled: "停用",
      configured: "已配置",
      missing: "未配置",
      healthy: "健康",
      unknown: "未知",
      passed: "已通过",
      untested: "未测试",
      incomplete: "未完整"
    },
    hints: {
      noSecret: "浏览器响应不返回明文或密文凭证。",
      noGroup: "路由提示只展示路由策略候选，不引入渠道分组。",
      audit: "写动作必须带 Idempotency-Key 和 X-Reason，并写入审计。"
    }
  },
  models: {
    title: "模型管理",
    subtitle: "T04 补齐模型目录、安全详情、结构定义预览、组件价格摘要、渠道覆盖和客户门户预览。",
    status: "P24-T04",
    sections: {
      catalog: "模型目录",
      coverage: "渠道覆盖",
      portalPreview: "门户预览",
      workflow: "工作流"
    },
    columns: {
      model: "模型",
      category: "分类",
      capability: "能力",
      price: "价格摘要",
      coverage: "覆盖数",
      state: "状态",
      health: "健康",
      cost: "成本配置"
    },
    actions: {
      create: "创建/编辑",
      patch: "局部更新",
      disable: "停用模型",
      deprecate: "标记弃用",
      channels: "渠道覆盖",
      schema: "结构预览",
      syncPreview: "目录同步预览"
    },
    state: {
      enabled: "启用",
      disabled: "停用",
      stable: "稳定",
      candidate: "候选",
      deprecated: "已弃用",
      healthy: "健康",
      unknown: "未知",
      passed: "已通过",
      untested: "未测试",
      configured: "已配置",
      incomplete: "未完整",
      unpriced: "未配置价格",
      componentPrice: "组件价格"
    }
  },
  session: {
    signedIn: "已登录",
    signedOut: "未登录"
  }
};

export function adminSessionLabel(authenticated: boolean): string {
  return authenticated ? adminCopy.session.signedIn : adminCopy.session.signedOut;
}
