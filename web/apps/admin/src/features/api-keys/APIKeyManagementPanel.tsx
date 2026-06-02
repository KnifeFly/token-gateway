import { formatISODateTime, formatInteger, formatMoney } from "@token-gateway/format";
import { StatusBadge } from "@token-gateway/ui";

import { adminCopy } from "../../shared/i18n";
import type { AdminAPIKeyView } from "./apiKeyApi";

const sampleAPIKeys: AdminAPIKeyView[] = [
  {
    id: "key_acme_live",
    tenant_id: "tenant_acme",
    project_id: "project_platform",
    name: "生产环境",
    fingerprint: "f2a9c7d81e34",
    enabled: true,
    allowed_models: ["gpt-4o-mini", "qwen-plus"],
    ip_allowlist: ["203.0.113.10", "2001:db8::/32"],
    expires_at: "2026-06-30T00:00:00Z",
    last_used_at: "2026-06-02T08:20:00Z",
    usage_summary: {
      requests: 1842,
      input_tokens: 2205000,
      output_tokens: 642000,
      revenue_micros: 85000000,
      currency: "CNY"
    },
    created_at: "2026-05-20T03:13:00Z",
    updated_at: "2026-06-01T10:42:00Z"
  },
  {
    id: "key_nova_sandbox",
    tenant_id: "tenant_nova",
    project_id: "project_sandbox",
    name: "验证密钥",
    fingerprint: "0ac921fd7821",
    enabled: false,
    allowed_models: ["gpt-4o-mini"],
    ip_allowlist: [],
    expires_at: "2026-06-15T00:00:00Z",
    last_used_at: "2026-05-30T11:04:00Z",
    usage_summary: {
      requests: 96,
      input_tokens: 82000,
      output_tokens: 19000,
      revenue_micros: 60000000,
      currency: "CNY"
    },
    revoked_at: "2026-06-01T14:18:00Z",
    created_at: "2026-05-18T04:49:00Z",
    updated_at: "2026-06-01T14:18:00Z"
  }
];

const workflowRows = [
  { label: adminCopy.apiKeys.actions.create, endpoint: "POST /api/admin/v1/api-keys" },
  {
    label: adminCopy.apiKeys.actions.update,
    endpoint: "POST /api/admin/v1/api-keys/{id}/update"
  },
  {
    label: adminCopy.apiKeys.actions.enable,
    endpoint: "POST /api/admin/v1/api-keys/{id}/enable"
  },
  {
    label: adminCopy.apiKeys.actions.disable,
    endpoint: "POST /api/admin/v1/api-keys/{id}/disable"
  },
  {
    label: adminCopy.apiKeys.actions.rotate,
    endpoint: "POST /api/admin/v1/api-keys/{id}/rotate"
  }
];

function dateLabel(value?: string | null): string {
  return value ? formatISODateTime(value) : adminCopy.apiKeys.state.none;
}

function modelLabel(models?: string[]): string {
  if (!models || models.length === 0) {
    return adminCopy.apiKeys.state.inherit;
  }
  if (models.includes("*")) {
    return adminCopy.apiKeys.state.allModels;
  }
  return models.join(" / ");
}

function ipLabel(values?: string[]): string {
  return values && values.length > 0 ? values.join(" / ") : adminCopy.apiKeys.state.anyIP;
}

function usageLabel(key: AdminAPIKeyView): string {
  const usage = key.usage_summary;
  const amount = formatMoney((usage?.revenue_micros ?? 0) / 1_000_000, 4);
  return `${formatInteger(usage?.requests ?? 0)} ${adminCopy.apiKeys.columns.requests} · ${
    usage?.currency ?? "CNY"
  } ${amount}`;
}

export function APIKeyManagementPanel() {
  return (
    <section className="panel api-key-panel" id="api-keys">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.apiKeys.title}</h2>
          <p>{adminCopy.apiKeys.subtitle}</p>
        </div>
        <StatusBadge tone="warning">{adminCopy.apiKeys.status}</StatusBadge>
      </div>

      <div className="api-key-layout">
        <article className="api-key-section">
          <h3>{adminCopy.apiKeys.sections.list}</h3>
          <div className="table api-key-table" role="table">
            <div className="table-row table-head api-key-row" role="row">
              <span role="columnheader">{adminCopy.apiKeys.columns.key}</span>
              <span role="columnheader">{adminCopy.apiKeys.columns.scope}</span>
              <span role="columnheader">{adminCopy.apiKeys.columns.models}</span>
              <span role="columnheader">{adminCopy.apiKeys.columns.ip}</span>
              <span role="columnheader">{adminCopy.apiKeys.columns.expires}</span>
              <span role="columnheader">{adminCopy.apiKeys.columns.usage}</span>
              <span role="columnheader">{adminCopy.apiKeys.columns.state}</span>
            </div>
            {sampleAPIKeys.map((key) => (
              <div className="table-row api-key-row" key={key.id} role="row">
                <span role="cell">
                  <strong>{key.name}</strong>
                  <small>{key.fingerprint}</small>
                </span>
                <span role="cell">
                  {key.tenant_id} / {key.project_id}
                </span>
                <span role="cell">{modelLabel(key.allowed_models)}</span>
                <span role="cell">{ipLabel(key.ip_allowlist)}</span>
                <span role="cell">{dateLabel(key.expires_at)}</span>
                <span role="cell">{usageLabel(key)}</span>
                <span role="cell">
                  {key.enabled ? adminCopy.apiKeys.state.enabled : adminCopy.apiKeys.state.disabled}
                </span>
              </div>
            ))}
          </div>
        </article>

        <div className="api-key-detail-grid">
          <article className="api-key-section">
            <h3>{adminCopy.apiKeys.sections.rotation}</h3>
            <div className="api-key-secret-preview">
              <strong>{adminCopy.apiKeys.hints.oneTimeTitle}</strong>
              <span>{adminCopy.apiKeys.hints.oneTimeBody}</span>
              <code>{adminCopy.apiKeys.hints.oneTimePlaceholder}</code>
            </div>
          </article>

          <article className="api-key-section">
            <h3>{adminCopy.apiKeys.sections.guardrails}</h3>
            <div className="tag-list">
              <span>{adminCopy.apiKeys.hints.noHash}</span>
              <span>{adminCopy.apiKeys.hints.noExpansion}</span>
              <span>{adminCopy.apiKeys.hints.audit}</span>
            </div>
          </article>
        </div>

        <article className="guardrail-band api-key-workflow">
          <h3>{adminCopy.apiKeys.sections.workflow}</h3>
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
