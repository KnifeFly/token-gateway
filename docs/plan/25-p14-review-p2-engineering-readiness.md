# P14 Review P2 工程交付与安全基线

## 阶段目标

P14 的目标是在 P12/P13 的正确性和商业边界收敛后，处理 review 中 P2 级工程规范和交付可信度问题。重点是降低未来复用踩坑、部署误用和交接成本。

本阶段不启动完整 RBAC/OIDC/mTLS 平台建设，不引入对象存储、完整 Realtime、动态插件或多地域架构。P14 只补足当前 ToB 网关交付前需要的安全基线、stream 稳定性、metrics 防御性和 README/运行文档。

## 交付物

- Customer API key hash 从纯 SHA-256 迁移到带 server secret 的 HMAC-SHA256，并保留兼容迁移窗口。
- Control plane 静态 token 认证补齐 constant-time compare、body size limit、写 API idempotency、operator audit hook 和网络隔离说明。
- Stream close settlement 使用 bounded background context，避免 `context.Background()` 无限阻塞 HTTP goroutine。
- Stream usage 解析改为按 SSE event 边界聚合，不依赖 TCP read chunk 分割。
- Billing、worker、callback 等 metrics 依赖允许 nil-safe 或强制构造校验，避免测试/复用路径 panic。
- Trusted proxy client IP 模型明确，只在可信代理 CIDR 下读取 `X-Forwarded-For`/`X-Real-IP`。
- README 补齐产品定位、本地启动、curl 示例、支持 API、配置、账务、安全、worker、已知限制和生产 checklist。

## 核心实现顺序

1. 定义 API key hash version，新增 `hmac-sha256` 格式和 server secret 配置校验。
2. 在 API key 校验路径支持旧 SHA-256 与新 HMAC 并存，新增 key 默认写 HMAC；设计旧 key 迁移和禁用窗口。
3. 加固 control plane token 校验和请求保护，补齐 constant-time compare、body size limit、写 API idempotency 和 operator audit 事件。
4. 将 stream settlement、RecordFailed 和 failed settlement 写入改为带 timeout 的 background context，并保证失败进入 repair/outbox。
5. 实现或复用 SSE parser，按 event/data 边界解析 stream usage，并处理跨 chunk JSON。
6. 审计 metrics nil panic 路径，统一选择 no-op metrics 或构造函数 fail-fast，并补回归测试。
7. 实现 trusted proxy CIDR 配置和 client IP 解析规则，默认不信任外部 header。
8. 重写 README，使新工程师、客户 POC 和部署人员能从仓库根目录完成理解、启动和安全边界确认。

## 关键设计约束

- API key 明文仍不得落库、日志、metrics、trace 或错误响应。
- HMAC server secret 必须来自安全配置；缺失时生产模式 fail-closed。
- P14 不建设完整 control plane RBAC/用户体系；如要上 OIDC/mTLS/RBAC，需要另立产品和架构阶段。
- Client disconnect 不能取消账务修复，但 stream close 也不能无限期阻塞。
- SSE parser 必须处理 event 跨 read chunk、空行结束、多 `data:` 行和 `[DONE]`。
- Trusted proxy 默认关闭；只有请求来源在可信代理 CIDR 中，才接受 forwarded headers。
- README 不能夸大能力，已知限制要与当前路线边界一致。

## 验收标准

- 新建 customer API key 使用 HMAC hash，旧 SHA-256 key 在兼容窗口内仍可验证，迁移路径有测试。
- Control plane token 比较为 constant-time，超大 body 被拒绝，写 API idempotency 和 audit hook 有覆盖。
- Stream close 在 DB 慢或 settlement 失败时不会无限阻塞，并能进入 failed settlement repair。
- Stream usage parser 能正确解析跨 chunk SSE usage，不漏记 final usage。
- Metrics 为 nil 或缺失时不会 panic，或构造函数明确返回错误并被 bootstrap 捕获。
- Trusted proxy 测试覆盖直接连接、可信代理、不可信代理伪造 header 和 IPv4/IPv6。
- README 能让读者完成本地启动、健康检查、最小 chat curl、worker 说明和生产安全 checklist。

## 风险与处理

| 风险 | 处理 |
|---|---|
| API key hash 迁移影响线上 key | 使用 hash version 前缀和双读策略，新 key 先切 HMAC，旧 key 有明确迁移窗口 |
| Control plane hardening 被误解为完整 RBAC | 文档明确 P14 只做静态 token 安全基线，完整 RBAC/OIDC/mTLS 另立路线 |
| Stream settlement timeout 后丢账 | timeout 只限制 close 阻塞时间，失败必须写 failed settlement 或 repair outbox |
| SSE parser 过度复杂 | 只实现协议必需事件边界和 usage 提取，provider 特化仍留在 adapter |
| README 与实现漂移 | README 只链接权威 plan/design/runbook，并用可执行命令做最小入口 |

## 设计来源

- [路线图](./00-roadmap.md)
- [M1 最小数据面](./02-m1-minimal-dataplane.md)
- [M3 Streaming + Native Compatible](./04-m3-protocols-streaming.md)
- [M5 控制面与 Runtime Snapshot](./06-m5-control-plane-snapshot.md)
- [M7 生产观测与稳定性](./08-m7-observability-stability.md)
- [P8 Portal API](./19-p8-portal-api.md)
- [任务清单](../tasks.md)
- [系统设计](../design/ai_gateway_system_design.md)
- [架构设计](../design/ai_gateway_architecture_design.md)
- [OpenAPI 合同](../design/ai_gateway_openapi.yaml)
