import { StatusBadge } from "@token-gateway/ui";

import { adminCopy } from "../../shared/i18n";
import type { AdminChannelView } from "./channelApi";

const sampleChannels: AdminChannelView[] = [
  {
    id: "channel_openai_primary",
    provider_type: "openai",
    base_url: "https://api.openai.com/v1",
    credential_configured: true,
    enabled: true,
    model_count: 2,
    health_status: "healthy",
    test_status: "passed",
    cost_config_status: "configured",
    route_policy_hints: [
      {
        route_id: "route_gpt_4o_mini",
        public_model: "gpt-4o-mini",
        strategy: "weighted",
        enabled: true,
        priority: 10,
        weight: 80
      }
    ],
    models: [
      {
        public_model: "gpt-4o-mini",
        upstream_model: "gpt-4o-mini",
        capabilities: ["chat", "vision"],
        health_status: "healthy",
        test_status: "passed",
        cost_config_status: "configured"
      },
      {
        public_model: "text-embedding-3-small",
        upstream_model: "text-embedding-3-small",
        capabilities: ["embedding"],
        health_status: "healthy",
        test_status: "passed",
        cost_config_status: "configured"
      }
    ]
  },
  {
    id: "channel_qwen_backup",
    provider_type: "dashscope",
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1",
    credential_configured: false,
    enabled: false,
    model_count: 1,
    health_status: "disabled",
    test_status: "untested",
    cost_config_status: "incomplete",
    route_policy_hints: [],
    models: [
      {
        public_model: "qwen-plus",
        upstream_model: "qwen-plus",
        capabilities: ["chat"],
        health_status: "unknown",
        test_status: "untested",
        cost_config_status: "unknown"
      }
    ]
  }
];

const workflowRows = [
  { label: adminCopy.channels.actions.create, endpoint: "POST /api/admin/v1/channels" },
  {
    label: adminCopy.channels.actions.rotate,
    endpoint: "POST /api/admin/v1/channels/{id}/rotate-credential"
  },
  { label: adminCopy.channels.actions.test, endpoint: "POST /api/admin/v1/channels/{id}/test" },
  {
    label: adminCopy.channels.actions.syncPreview,
    endpoint: "POST /api/admin/v1/channels/{id}/sync-preview"
  },
  {
    label: adminCopy.channels.actions.syncApply,
    endpoint: "POST /api/admin/v1/channels/{id}/sync-apply"
  },
  {
    label: adminCopy.channels.actions.healthEvents,
    endpoint: "GET /api/admin/v1/channels/{id}/health-events"
  }
];

function channelStatusLabel(value?: string): string {
  const labels: Record<string, string> = {
    healthy: adminCopy.channels.state.healthy,
    unknown: adminCopy.channels.state.unknown,
    configured: adminCopy.channels.state.configured,
    passed: adminCopy.channels.state.passed,
    untested: adminCopy.channels.state.untested,
    incomplete: adminCopy.channels.state.incomplete,
    disabled: adminCopy.channels.state.disabled
  };
  return labels[(value ?? "").toLowerCase()] ?? value ?? adminCopy.channels.state.unknown;
}

export function ChannelManagementPanel() {
  return (
    <section className="panel channel-panel" id="routing">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.channels.title}</h2>
          <p>{adminCopy.channels.subtitle}</p>
        </div>
        <StatusBadge tone="warning">{adminCopy.channels.status}</StatusBadge>
      </div>

      <div className="channel-layout">
        <article className="channel-section">
          <h3>{adminCopy.channels.sections.workflow}</h3>
          <div className="table channel-table" role="table">
            <div className="table-row table-head channel-row" role="row">
              <span role="columnheader">{adminCopy.channels.columns.channel}</span>
              <span role="columnheader">{adminCopy.channels.columns.provider}</span>
              <span role="columnheader">{adminCopy.channels.columns.state}</span>
              <span role="columnheader">{adminCopy.channels.columns.credential}</span>
              <span role="columnheader">{adminCopy.channels.columns.models}</span>
              <span role="columnheader">{adminCopy.channels.columns.action}</span>
            </div>
            {sampleChannels.map((channel) => (
              <div className="table-row channel-row" key={channel.id} role="row">
                <span role="cell">{channel.id}</span>
                <span role="cell">{channel.provider_type}</span>
                <span role="cell">
                  {channel.enabled ? adminCopy.channels.state.enabled : adminCopy.channels.state.disabled}
                </span>
                <span role="cell">
                  {channel.credential_configured
                    ? adminCopy.channels.state.configured
                    : adminCopy.channels.state.missing}
                </span>
                <span role="cell">{channel.model_count}</span>
                <span role="cell">
                  <span className="inline-actions">
                    {workflowRows.slice(0, 4).map((action) => (
                      <span title={action.endpoint} key={action.label}>
                        {action.label}
                      </span>
                    ))}
                  </span>
                </span>
              </div>
            ))}
          </div>
        </article>

        <article className="channel-section">
          <h3>{adminCopy.channels.sections.coverage}</h3>
          <div className="coverage-grid">
            {sampleChannels.flatMap((channel) =>
              (channel.models ?? []).map((model) => (
                <div className="coverage-item" key={`${channel.id}-${model.public_model}`}>
                  <strong>{model.public_model}</strong>
                  <span>{model.upstream_model}</span>
                  <small>
                    {adminCopy.channels.columns.health} {channelStatusLabel(model.health_status)} ·{" "}
                    {adminCopy.channels.columns.cost} {channelStatusLabel(model.cost_config_status)}
                  </small>
                </div>
              ))
            )}
          </div>
        </article>

        <article className="channel-section">
          <h3>{adminCopy.channels.sections.routeHints}</h3>
          <div className="coverage-grid compact">
            {sampleChannels.flatMap((channel) =>
              (channel.route_policy_hints ?? []).map((hint) => (
                <div className="coverage-item" key={`${channel.id}-${hint.route_id}`}>
                  <strong>{hint.public_model}</strong>
                  <span>{hint.route_id}</span>
                  <small>
                    优先级 {hint.priority} · 权重 {hint.weight}
                  </small>
                </div>
              ))
            )}
          </div>
        </article>

        <article className="guardrail-band channel-guardrail">
          <h3>{adminCopy.channels.sections.safeFields}</h3>
          <div className="tag-list">
            <span>{adminCopy.channels.hints.noSecret}</span>
            <span>{adminCopy.channels.hints.noGroup}</span>
            <span>{adminCopy.channels.hints.audit}</span>
          </div>
          <div className="endpoint-list">
            {workflowRows.map((action) => (
              <code key={action.endpoint}>{action.endpoint}</code>
            ))}
          </div>
        </article>
      </div>
    </section>
  );
}
