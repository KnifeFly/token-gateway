# P18 Review Follow-up 文件资产边界收口

## 阶段目标

P18 的目标是把文件/媒体输入资产能力从容易被误解的“文件上传存储”收敛为明确的 transient input asset metadata registry，除非后续产品明确投入对象存储能力。

当前路线选择保守策略：继续 metadata registry，不承诺对象存储、下载代理、生命周期 SLA 或媒体资产托管。P18 要把代码、OpenAPI、README、runbook、quota 和 cleanup job 都对齐到这个边界，避免客户把 `/v1/files/*` 当作长期文件系统使用。

## 交付物

- 产品命名和文档统一为 transient input asset metadata registration：URL asset registration、base64 hash/metadata registration、limited inline input，不叫完整文件上传存储。
- OpenAPI、README、Portal/客户 runbook 明确声明：gateway 不保存真实对象内容，不提供 download proxy，不承诺 media lifecycle，不提供存储 SLA。
- File quota 查询排除 expired assets，避免过期 transient rows 继续占用客户 quota。
- Expired transient file cleanup job：定期删除或归档过期 metadata rows，并记录 cleanup metrics。
- URL/base64/stream upload 行为收敛：URL 只保存 source URL 与必要 metadata；base64 只保存 hash/size/metadata 或明确拒绝大输入；stream upload 若未实现真实 spool，保持 feature disabled。
- 如果后续选择真正文件资产服务，单独开启新阶段，覆盖 object storage/local spool、streaming multipart、content-addressed storage、signed download URL、scan hook 和 lifecycle worker。
- 文件资产 contract tests 覆盖 quota、expires_at、cleanup、URL metadata、base64 metadata、stream disabled 和 egress guard 生产要求。

## 核心实现顺序

1. 审计 `/v1/files/*`、FileService、portal task/file 查询、OpenAPI schema、README 和 runbook 中所有“upload、file storage、download、lifecycle”表述，标记会夸大能力的位置。
2. 将文档术语改为 transient input asset metadata registry，并明确 `file_id` 只代表可被任务引用的输入 metadata，不代表可下载对象。
3. 修改 MySQL `FileQuota()` 查询：按 tenant/project 统计时增加 `(expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)` 条件，过期 rows 不再占用有效 quota。
4. 增加 cleanup job 或扩展现有 worker job：按 `expires_at < now` 删除/归档 transient file rows，记录删除数量、失败数量、最大 age 和 next run。
5. 校准 URL flow：保留 source URL、hash/size 可选 metadata 和 egress guard 校验结果；禁止把 source URL 包装成 gateway 承诺的长期 file URL。
6. 校准 base64 flow：对 inline size 设置明确上限，保存 hash、size、media type、metadata 和 expires_at；不保存内容时禁止后续 download 语义。
7. 保持 stream upload endpoint feature disabled，除非同阶段实现真实 spool/reference；错误响应、OpenAPI 和 README 必须一致。
8. 补充 contract tests 和 focused tests，覆盖 expired quota、cleanup、URL/base64 metadata、stream disabled、生产 egress guard 关闭失败和客户可见响应字段。
9. 在 docs/plan、docs/tasks.md 和 runbook 中记录未来路线 B 的进入条件：对象存储、signed URL、scan hook、cleanup lifecycle、quota 和成本模型都必须一次性设计。

## 关键设计约束

- P18 默认不做对象存储；文件能力只是媒体任务输入归一化和审计 metadata。
- `file_id` 不应被客户理解为长期可下载对象 ID；任何下载、代理、存储 SLA 都必须明确不支持。
- 过期 transient rows 不应继续占用客户当前 quota，但可以按审计需要保留最小 metadata 或归档记录。
- URL asset 必须经过 egress guard；生产环境不得因配置关闭而绕过 SSRF 防线。
- Base64 输入不能变成无上限内存读取；必须有 body size、decoded size 和 metadata 上限。
- 如果未来要做真实文件资产服务，必须另立阶段，不在 P18 中偷偷实现半套存储能力。

## 验收标准

- README、OpenAPI、runbook 和 customer-facing docs 中不再把当前能力描述为完整文件上传存储；能力边界统一为 transient metadata registry。
- `FileQuota()` 不统计已过期 asset；新增测试覆盖 expired、active、no-expiry 和跨 tenant/project。
- Cleanup job 能删除或归档过期 transient file rows，并输出 metrics/log；失败可重试，不影响热路径。
- URL asset registration 在 egress guard 关闭的生产配置下 fail closed；在 allowlist/安全 URL 下保存正确 metadata。
- Base64 input 只保存 metadata/hash/size 或按上限拒绝，不承诺后续下载。
- Stream upload 未实现时稳定返回 feature disabled，OpenAPI/README 与运行行为一致。
- Contract tests、focused tests、`go test ./internal/task ./internal/worker/jobs ./internal/transport/httpserver` 或等价命令和 `git diff --check` 通过。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 客户已依赖 file URL 作为下载地址 | 文档和响应字段区分 source URL 与 gateway file id，避免新增 gateway download 承诺 |
| 过期 row cleanup 影响审计 | 只删除非审计必要 metadata，或先归档后删除；usage/ledger/task 记录仍保留任务审计链 |
| URL passthrough 暴露内网 SSRF | P15/P18 双重要求生产 egress fail-closed，并提供 allowlist 明确放行 |
| Base64 大输入导致内存压力 | 严格 body/decoded size limit，必要时直接拒绝而不是半支持 |
| 路线 B 被零散实现 | 在 plan 中明确对象存储路线需要独立阶段和完整交付物，不在 P18 范围内 |

## 设计来源

- [路线图](./00-roadmap.md)
- [M4 Unified Media Async Task](./05-m4-media-tasks.md)
- [P7 Media Forwarding Providers](./18-p7-media-forwarding-providers.md)
- [P13 Review P1 商业账务与安全边界](./24-p13-review-p1-commercial-hardening.md)
- [P15 Review Follow-up P0 生产阻塞收敛](./26-p15-review-followup-p0-production-blockers.md)
- [任务清单](../tasks.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
