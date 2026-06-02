export const portalLocale = "zh-CN" as const;

export const portalCopy = {
  brand: "客户门户",
  navigationLabel: "客户门户导航",
  workspaceLabel: "客户门户工作区",
  summaryLabel: "客户门户摘要",
  topbar: {
    title: "客户门户",
    syncing: "同步中",
    signedIn: "已登录",
    logout: "退出"
  },
  login: {
    apiKeyLabel: "客户 API 密钥",
    signingIn: "正在登录",
    signIn: "登录"
  },
  summary: {
    credits: "可用额度",
    creditsFallback: "默认币种",
    requests: "请求数",
    requestsDetail: "已结算用量",
    activeKeys: "有效密钥",
    totalKeys: (count: string) => `共 ${count} 个密钥`,
    tasks: "任务数",
    processingTasks: (count: string) => `${count} 个处理中`
  },
  dashboard: {
    usageTitle: "用量概览",
    requests: "请求数",
    inputTokens: "输入 Token",
    outputTokens: "输出 Token",
    tasksTitle: "任务概览",
    queued: (count: string) => `排队 ${count}`,
    processing: (count: string) => `处理中 ${count}`,
    completed: (count: string) => `已完成 ${count}`
  },
  models: {
    title: "模型",
    previewTitle: "模型预览",
    schemaTitle: "结构定义",
    modelColumn: "模型",
    typeColumn: "类型",
    categoryColumn: "分类",
    modeColumn: "模式",
    priceColumn: "价格摘要",
    contextColumn: "上下文",
    unpriced: "未配置价格",
    componentPrice: "组件价格",
    descriptionFallback: "暂无模型说明"
  },
  playground: {
    title: "操练场",
    description: "P24 先固定入口和文案边界，后续按模型结构定义生成请求表单。",
    routeColumn: "路由",
    purposeColumn: "用途",
    nextTaskColumn: "下一任务",
    guardrailTitle: "安全边界",
    guardrailItems: ["不保存客户密钥", "不展示上游凭证", "不导出原始敏感响应"]
  },
  apiKeys: {
    title: "API 密钥",
    createTitle: "创建密钥",
    nameColumn: "名称",
    modelsColumn: "模型范围",
    ipColumn: "IP 白名单",
    expiresColumn: "过期时间",
    usageColumn: "用量",
    stateColumn: "状态",
    actionColumn: "操作",
    nameLabel: "名称",
    allowedModelsLabel: "允许模型",
    ipAllowlistLabel: "IP 白名单",
    expiresAtLabel: "过期时间",
    createAction: "创建",
    disableAction: "停用",
    rotateAction: "轮换",
    plaintextHint: "明文密钥仅显示一次",
    noLimit: "不限",
    neverExpires: "永不过期",
    usageRequests: (count: string) => `${count} 次`
  },
  credits: {
    title: "额度",
    bucketColumn: "账户",
    remainingColumn: "剩余额度",
    usedColumn: "已用额度",
    heldColumn: "冻结额度"
  },
  tasks: {
    title: "任务",
    taskColumn: "任务",
    requestColumn: "请求",
    statusColumn: "状态",
    modelColumn: "模型",
    channelColumn: "供应商/渠道",
    createdColumn: "创建时间",
    requestIDLabel: "请求 ID",
    apiKeyLabel: "API 密钥",
    filterAction: "筛选",
    empty: "暂无任务记录",
    fallbackTask: "任务",
    fallbackStatus: "未知"
  },
  usage: {
    title: "用量",
    requestColumn: "请求",
    statusColumn: "状态",
    modelColumn: "模型",
    channelColumn: "供应商/渠道",
    inputColumn: "输入",
    outputColumn: "输出",
    creditsColumn: "额度",
    requestIDLabel: "请求 ID",
    apiKeyLabel: "API 密钥",
    filterAction: "筛选",
    empty: "暂无用量记录",
    aggregateRow: "汇总"
  },
  settings: {
    title: "项目设置",
    tenant: "租户",
    project: "项目",
    apiKey: "API 密钥",
    allowedModels: "允许模型"
  },
  defaults: {
    derivedKeyName: "派生密钥",
    ledger: "账务记录"
  },
  state: {
    enabled: "已启用",
    disabled: "已停用",
    sync: "同步",
    async: "异步"
  }
};

const statusLabels: Record<string, string> = {
  queued: "排队中",
  processing: "处理中",
  running: "运行中",
  succeeded: "成功",
  success: "成功",
  completed: "已完成",
  failed: "失败",
  canceled: "已取消",
  cancelled: "已取消",
  settled: "已结算",
  pending: "待处理",
  unknown: "未知"
};

export function enabledLabel(enabled: boolean): string {
  return enabled ? portalCopy.state.enabled : portalCopy.state.disabled;
}

export function modelModeLabel(asyncModel: boolean): string {
  return asyncModel ? portalCopy.state.async : portalCopy.state.sync;
}

export function displayStatusLabel(value?: string): string {
  if (!value) {
    return portalCopy.tasks.fallbackStatus;
  }

  return statusLabels[value.toLowerCase()] ?? value;
}
