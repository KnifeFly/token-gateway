import { StatusBadge } from "@token-gateway/ui";

import { adminCopy } from "../../shared/i18n";
import type { AdminModelView } from "./modelApi";

const sampleModels: AdminModelView[] = [
  {
    public_model: "gpt-4o-mini",
    aliases: ["chat-default"],
    display_name: "GPT-4o Mini",
    description: "面向日常对话、结构化输出和轻量视觉输入的默认模型。",
    protocol: "native_openai",
    capability: "chat",
    type: "multimodal",
    category: "chat",
    tags: ["默认", "视觉", "工具调用"],
    provider_family: "openai",
    modalities: ["text", "image"],
    capabilities: ["chat", "tool", "vision"],
    input_modalities: ["text", "image"],
    output_modalities: ["text"],
    context_window: 128000,
    max_output_tokens: 4096,
    status: "stable",
    deprecated: false,
    schema_available: true,
    enabled: true,
    async: false,
    pricing_summary: {
      configured: true,
      currency: "CNY",
      category: "chat",
      components: [
        { unit: "input_token", micros_per_unit: 2 },
        { unit: "output_token", micros_per_unit: 8 }
      ],
      component_price_count: 2,
      legacy_token_price_active: false
    },
    channel_coverage: [
      {
        channel_id: "channel_openai_primary",
        provider_type: "openai",
        enabled: true,
        upstream_model: "gpt-4o-mini",
        capabilities: ["chat", "vision"],
        health_status: "healthy",
        test_status: "passed",
        cost_config_status: "configured",
        credential_configured: true
      }
    ]
  },
  {
    public_model: "qwen-plus",
    display_name: "Qwen Plus",
    description: "中文任务、知识问答和长文本处理的备用模型。",
    protocol: "native_openai",
    capability: "chat",
    type: "text",
    category: "chat",
    tags: ["中文", "备用"],
    provider_family: "dashscope",
    modalities: ["text"],
    capabilities: ["chat"],
    input_modalities: ["text"],
    output_modalities: ["text"],
    status: "candidate",
    deprecated: false,
    schema_available: true,
    enabled: true,
    async: false,
    pricing_summary: {
      configured: true,
      currency: "CNY",
      category: "chat",
      components: [{ unit: "input_token", micros_per_unit: 1 }],
      component_price_count: 1,
      legacy_token_price_active: false
    },
    channel_coverage: [
      {
        channel_id: "channel_qwen_backup",
        provider_type: "dashscope",
        enabled: false,
        upstream_model: "qwen-plus",
        capabilities: ["chat"],
        health_status: "unknown",
        test_status: "untested",
        cost_config_status: "incomplete",
        credential_configured: false
      }
    ]
  }
];

const workflowRows = [
  { label: adminCopy.models.actions.create, endpoint: "POST /api/admin/v1/models" },
  { label: adminCopy.models.actions.patch, endpoint: "PATCH /api/admin/v1/models/{id}" },
  { label: adminCopy.models.actions.disable, endpoint: "POST /api/admin/v1/models/{id}/disable" },
  {
    label: adminCopy.models.actions.deprecate,
    endpoint: "POST /api/admin/v1/models/{id}/deprecate"
  },
  {
    label: adminCopy.models.actions.channels,
    endpoint: "GET /api/admin/v1/models/{id}/channels"
  },
  {
    label: adminCopy.models.actions.schema,
    endpoint: "GET /api/admin/v1/models/{id}/schema-preview"
  },
  {
    label: adminCopy.models.actions.syncPreview,
    endpoint: "POST /api/admin/v1/models/sync-preview"
  }
];

function statusLabel(value?: string): string {
  const labels: Record<string, string> = {
    stable: adminCopy.models.state.stable,
    candidate: adminCopy.models.state.candidate,
    deprecated: adminCopy.models.state.deprecated,
    healthy: adminCopy.models.state.healthy,
    unknown: adminCopy.models.state.unknown,
    passed: adminCopy.models.state.passed,
    untested: adminCopy.models.state.untested,
    configured: adminCopy.models.state.configured,
    incomplete: adminCopy.models.state.incomplete
  };
  return labels[(value ?? "").toLowerCase()] ?? value ?? adminCopy.models.state.unknown;
}

function priceSummary(model: AdminModelView): string {
  if (!model.pricing_summary.configured) {
    return adminCopy.models.state.unpriced;
  }
  const units = model.pricing_summary.components?.map((component) => component.unit).join(" / ");
  return `${model.pricing_summary.currency ?? "-"} · ${units || adminCopy.models.state.componentPrice}`;
}

export function ModelManagementPanel() {
  return (
    <section className="panel model-panel" id="catalog">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.models.title}</h2>
          <p>{adminCopy.models.subtitle}</p>
        </div>
        <StatusBadge tone="warning">{adminCopy.models.status}</StatusBadge>
      </div>

      <div className="model-layout">
        <article className="model-section">
          <h3>{adminCopy.models.sections.catalog}</h3>
          <div className="table model-table" role="table">
            <div className="table-row table-head model-row" role="row">
              <span role="columnheader">{adminCopy.models.columns.model}</span>
              <span role="columnheader">{adminCopy.models.columns.category}</span>
              <span role="columnheader">{adminCopy.models.columns.capability}</span>
              <span role="columnheader">{adminCopy.models.columns.price}</span>
              <span role="columnheader">{adminCopy.models.columns.coverage}</span>
              <span role="columnheader">{adminCopy.models.columns.state}</span>
            </div>
            {sampleModels.map((model) => (
              <div className="table-row model-row" key={model.public_model} role="row">
                <span role="cell">
                  <strong>{model.display_name ?? model.public_model}</strong>
                  <small>{model.public_model}</small>
                </span>
                <span role="cell">{model.category ?? "-"}</span>
                <span role="cell">{model.capabilities?.join(" / ")}</span>
                <span role="cell">{priceSummary(model)}</span>
                <span role="cell">{model.channel_coverage?.length ?? 0}</span>
                <span role="cell">{model.enabled ? adminCopy.models.state.enabled : adminCopy.models.state.disabled}</span>
              </div>
            ))}
          </div>
        </article>

        <article className="model-section">
          <h3>{adminCopy.models.sections.coverage}</h3>
          <div className="coverage-grid">
            {sampleModels.flatMap((model) =>
              (model.channel_coverage ?? []).map((coverage) => (
                <div className="coverage-item" key={`${model.public_model}-${coverage.channel_id}`}>
                  <strong>{model.public_model}</strong>
                  <span>{coverage.channel_id}</span>
                  <small>
                    {coverage.provider_type} · {coverage.upstream_model}
                  </small>
                  <small>
                    {adminCopy.models.columns.health} {statusLabel(coverage.health_status)} ·{" "}
                    {adminCopy.models.columns.cost} {statusLabel(coverage.cost_config_status)}
                  </small>
                </div>
              ))
            )}
          </div>
        </article>

        <article className="model-section">
          <h3>{adminCopy.models.sections.portalPreview}</h3>
          <div className="portal-preview-grid">
            {sampleModels.map((model) => (
              <div className="portal-preview-item" key={model.public_model}>
                <strong>{model.display_name ?? model.public_model}</strong>
                <span>{model.description}</span>
                <div className="tag-list">
                  {(model.input_modalities ?? []).map((item) => (
                    <span key={`${model.public_model}-${item}`}>{item}</span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </article>

        <article className="model-section">
          <h3>{adminCopy.models.sections.workflow}</h3>
          <div className="endpoint-list">
            {workflowRows.map((action) => (
              <code key={action.endpoint}>
                {action.label} · {action.endpoint}
              </code>
            ))}
          </div>
        </article>
      </div>
    </section>
  );
}
