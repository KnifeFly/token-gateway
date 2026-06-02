import { StatusBadge } from "@token-gateway/ui";

import { adminCopy } from "../../shared/i18n";
import type { AdminPlaygroundRunResult } from "./playgroundApi";

const sampleResult: AdminPlaygroundRunResult = {
  request_id: "pg_admin_preview",
  scope: "admin",
  status: "ready",
  message: "结构校验通过，已完成安全校验，未调用上游。",
  model: "gpt-public",
  mode: "chat",
  stream: false,
  payload_fields: ["model"],
  schema: {
    required: ["input", "model"],
    accepted_fields: ["model"],
    missing_required: []
  },
  debug: {
    route_id: "route_gpt_public",
    channel_id: "channel_primary",
    provider_type: "openai",
    latency_ms: 1,
    usage: {
      input_tokens: 16,
      output_tokens: 0,
      total_tokens: 16
    },
    channel_test: {
      channel_id: "channel_primary",
      status: "ready",
      message: "渠道配置已通过供应商测试前置检查。",
      credential_configured: true,
      model_count: 1,
      tested_at: new Date().toISOString()
    }
  },
  result: {
    object: "playground.dry_run",
    summary: "安全校验已完成，结果不包含原始提示词、响应或凭证。"
  },
  ran_at: new Date().toISOString()
};

const workflows = [
  {
    action: adminCopy.tools.actions.run,
    endpoint: "POST /api/admin/v1/playground/run",
    scope: adminCopy.tools.hints.scope,
    status: adminCopy.tools.state.dryRun
  },
  {
    action: adminCopy.tools.actions.channelTest,
    endpoint: "POST /api/admin/v1/channels/{id}/test",
    scope: adminCopy.tools.sections.channelTest,
    status: adminCopy.tools.state.shared
  },
  {
    action: adminCopy.tools.actions.importPreview,
    endpoint: "POST /api/admin/v1/playground/import-preview",
    scope: adminCopy.tools.sections.importExport,
    status: adminCopy.tools.state.ready
  },
  {
    action: adminCopy.tools.actions.export,
    endpoint: "POST /api/admin/v1/playground/export",
    scope: adminCopy.tools.sections.importExport,
    status: adminCopy.tools.state.ready
  }
];

export function ToolsPlaygroundPanel() {
  return (
    <section className="tools-panel panel" id="tools">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.tools.title}</h2>
          <p>{adminCopy.tools.subtitle}</p>
        </div>
        <StatusBadge tone="neutral">{adminCopy.tools.status}</StatusBadge>
      </div>

      <div className="tools-layout">
        <article className="tools-section">
          <h3>{adminCopy.tools.sections.playground}</h3>
          <div className="tools-table">
            <div className="tools-row head">
              <span>{adminCopy.tools.columns.item}</span>
              <span>{adminCopy.tools.columns.endpoint}</span>
              <span>{adminCopy.tools.columns.status}</span>
            </div>
            {workflows.map((workflow) => (
              <div className="tools-row" key={workflow.endpoint}>
                <span>{workflow.action}</span>
                <span>{workflow.endpoint}</span>
                <span>{workflow.status}</span>
              </div>
            ))}
          </div>
        </article>

        <article className="tools-section">
          <h3>{adminCopy.tools.sections.safeDebug}</h3>
          <div className="debug-summary">
            <StatusBadge tone="success">{adminCopy.tools.state.ready}</StatusBadge>
            <strong>{sampleResult.result.summary}</strong>
            <dl>
              <div>
                <dt>请求 ID</dt>
                <dd>{sampleResult.request_id}</dd>
              </div>
              <div>
                <dt>路由/渠道</dt>
                <dd>
                  {sampleResult.debug.route_id} / {sampleResult.debug.channel_id}
                </dd>
              </div>
              <div>
                <dt>供应商</dt>
                <dd>{sampleResult.debug.provider_type}</dd>
              </div>
              <div>
                <dt>用量</dt>
                <dd>{sampleResult.debug.usage.total_tokens} Token</dd>
              </div>
            </dl>
          </div>
        </article>
      </div>

      <div className="tools-hint-grid">
        <span>{adminCopy.tools.hints.schema}</span>
        <span>{adminCopy.tools.hints.scope}</span>
        <span>{adminCopy.tools.hints.debug}</span>
        <span>{adminCopy.tools.hints.export}</span>
      </div>
    </section>
  );
}
