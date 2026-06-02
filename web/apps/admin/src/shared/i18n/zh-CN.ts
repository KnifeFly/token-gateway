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
  accounts: {
    title: "客户账户管理",
    subtitle: "T05 把 NewAPI 用户管理转译为租户/项目账户、状态、角色、额度、密钥、会话重置和审计。",
    status: "P24-T05",
    sections: {
      list: "客户列表",
      overview: "账户概览",
      credits: "额度",
      keys: "密钥元数据",
      usage: "近期用量",
      ledger: "账本记录",
      workflow: "工作流"
    },
    columns: {
      account: "账户",
      tenantProject: "租户/项目",
      status: "状态",
      role: "角色",
      credits: "可用额度",
      keys: "有效密钥",
      models: "模型范围",
      usage: "请求数",
      lastSeen: "最近访问",
      email: "联系人",
      notes: "备注",
      available: "可用",
      held: "冻结",
      granted: "已授予",
      used: "已使用",
      requests: "次请求"
    },
    actions: {
      create: "创建账户",
      enable: "启用账户",
      disable: "停用账户",
      adjustCredit: "手动额度调整",
      resetSession: "重置会话"
    },
    state: {
      active: "启用",
      disabled: "停用",
      closed: "关闭",
      enabled: "启用",
      allModels: "全部模型",
      noModels: "未配置模型",
      modelCount: "个模型",
      never: "无记录"
    },
    role: {
      owner: "负责人",
      developer: "开发者",
      viewer: "只读"
    },
    hints: {
      safeKey: "详情只展示密钥元数据，不返回明文密钥或密钥哈希。",
      session: "会话重置按租户、项目和可选密钥范围撤销浏览器会话。",
      adjustment: "调整必须带理由和 Idempotency-Key，并写入审计。",
      audit: "创建、停用、额度调整和会话重置都记录操作员审计。"
    }
  },
  apiKeys: {
    title: "密钥管理",
    subtitle: "T06 收口创建、更新范围、启用、停用、轮换、允许模型、IP 白名单、过期时间和用量摘要。",
    status: "P24-T06",
    sections: {
      list: "密钥列表",
      rotation: "创建与轮换",
      guardrails: "安全约束",
      workflow: "工作流"
    },
    columns: {
      key: "密钥",
      scope: "租户/项目",
      models: "允许模型",
      ip: "IP 白名单",
      expires: "过期时间",
      usage: "用量摘要",
      state: "状态",
      requests: "次请求"
    },
    actions: {
      create: "创建密钥",
      update: "更新范围",
      enable: "启用密钥",
      disable: "停用密钥",
      rotate: "轮换密钥"
    },
    state: {
      enabled: "启用",
      disabled: "停用",
      inherit: "保持当前范围",
      allModels: "全部模型",
      anyIP: "不限 IP",
      none: "未设置"
    },
    hints: {
      oneTimeTitle: "明文只显示一次",
      oneTimeBody: "创建或轮换响应返回一次性明文，列表和详情只展示指纹与安全元数据。",
      oneTimePlaceholder: "tgk_live_************",
      noHash: "不返回密钥哈希或原始密钥。",
      noExpansion: "更新 allowed models 时不能扩大客户项目的模型权限。",
      audit: "创建、更新、启停和轮换都需要理由、CSRF、幂等键和审计。"
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
