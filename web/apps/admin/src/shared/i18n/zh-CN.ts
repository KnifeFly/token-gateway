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
  creditOps: {
    title: "额度运营",
    subtitle: "T09 聚合余额、账本、冻结、修复队列、人工调整约束和安全导出。",
    status: "P24-T09",
    sections: {
      balance: "余额摘要",
      holds: "冻结状态",
      repairs: "结算修复",
      ledger: "账本记录",
      exports: "安全导出"
    },
    columns: {
      available: "可用额度",
      held: "冻结额度",
      used: "已用额度",
      retry: "重试"
    },
    state: {
      active: "活跃冻结",
      protected_task_holds: "任务保护冻结",
      manual_adjustment: "人工调整",
      usage_debit: "用量扣减",
      usage_settlement: "用量结算"
    },
    exportKind: {
      usage: "用量导出",
      ledger: "账本导出"
    },
    hints: {
      reason: "额度调整必须带理由和 Idempotency-Key。",
      audit: "调整、修复和导出入口必须保留操作员审计证据。",
      safeExport: "导出只包含 safe DTO，不包含密钥、原始提示词、响应或回调地址。",
      noPayment: "不接入支付、订阅、兑换码或充值配置。",
      exportFile: "示例导出文件"
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
  activity: {
    title: "活动日志",
    subtitle: "T07 补齐请求用量和异步任务运营视图，支持关键维度筛选和安全详情。",
    status: "P24-T07",
    sections: {
      usage: "用量日志",
      tasks: "任务日志",
      filters: "筛选维度",
      safeDetail: "安全详情"
    },
    columns: {
      request: "请求",
      task: "任务",
      scope: "租户/项目",
      model: "模型",
      route: "供应商/渠道",
      tokens: "Token",
      cost: "金额",
      progress: "进度",
      callback: "回调",
      status: "状态"
    },
    state: {
      configured: "已配置",
      notConfigured: "未配置",
      unknown: "未知"
    },
    hints: {
      noRawPayload: "详情不返回原始请求、响应或回调地址。",
      safeMetadata: "metadata 按敏感键过滤后再展示。",
      exportReady: "导出只使用 safe DTO，后续接入审计留痕。"
    }
  },
  tools: {
    title: "调试工具",
    subtitle: "T08 接入结构定义驱动操练场，并让渠道测试复用同一套安全执行器。",
    status: "P24-T08",
    sections: {
      playground: "操练场",
      channelTest: "渠道测试复用",
      importExport: "导入导出",
      safeDebug: "安全调试"
    },
    columns: {
      item: "项目",
      endpoint: "接口",
      scope: "范围",
      status: "状态"
    },
    actions: {
      run: "运行校验",
      importPreview: "导入预览",
      export: "安全导出",
      channelTest: "渠道测试"
    },
    state: {
      ready: "就绪",
      warning: "需处理",
      dryRun: "安全校验",
      shared: "复用执行器"
    },
    hints: {
      schema: "请求字段来自模型结构定义，不提供任意原始请求代理。",
      scope: "Admin 可指定渠道做受控测试；Portal 只能使用客户项目范围。",
      debug: "调试只展示请求 ID、路由、渠道、耗时、用量和安全错误。",
      export: "导入导出会移除密钥、凭证、提示词、响应和媒体原文。"
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
