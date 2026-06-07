import { formatISODateTime, formatInteger, formatMoney } from "@token-gateway/format";
import { Button, CopyButton, DataTable, EmptyState, LoadingState, StatusBadge } from "@token-gateway/ui";
import { useEffect, useState } from "react";

import { adminCopy } from "../../shared/i18n";
import {
  disableAdminAPIKey,
  enableAdminAPIKey,
  listAdminAPIKeys,
  rotateAdminAPIKey,
  type AdminAPIKeyRotateResponse,
  type AdminAPIKeyView
} from "./apiKeyApi";

interface APIKeyManagementPanelProps {
  csrfToken: string;
}

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

function describeError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function APIKeyManagementPanel({ csrfToken }: APIKeyManagementPanelProps) {
  const [keys, setKeys] = useState<AdminAPIKeyView[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [rotatedKey, setRotatedKey] = useState<AdminAPIKeyRotateResponse>();

  async function loadKeys() {
    setLoading(true);
    setMessage("");
    try {
      const response = await listAdminAPIKeys();
      setKeys(response.data);
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadKeys();
  }, []);

  async function mutateKey(keyID: string, action: "enable" | "disable" | "rotate") {
    setBusy(true);
    setMessage("");
    setRotatedKey(undefined);
    const mutation = {
      csrfToken,
      reason: `P25 Admin API key ${action} ${keyID}`
    };
    try {
      if (action === "enable") {
        await enableAdminAPIKey(keyID, mutation);
      } else if (action === "disable") {
        await disableAdminAPIKey(keyID, mutation);
      } else {
        setRotatedKey(await rotateAdminAPIKey(keyID, mutation));
      }
      await loadKeys();
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel api-key-panel" id="api-keys">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.apiKeys.title}</h2>
          <p>{adminCopy.apiKeys.subtitle}</p>
        </div>
        <StatusBadge tone="success">BFF connected</StatusBadge>
      </div>

      {message ? <div className="inline-alert">{message}</div> : null}
      {loading ? <LoadingState label="正在加载 API keys" /> : null}

      <div className="api-key-layout">
        <article className="api-key-section">
          <h3>{adminCopy.apiKeys.sections.list}</h3>
          <DataTable
            ariaLabel={adminCopy.apiKeys.sections.list}
            className="api-key-table"
            columns={[
              {
                key: "key",
                header: adminCopy.apiKeys.columns.key,
                render: (key) => (
                  <>
                    <strong>{key.name}</strong>
                    <small>{key.fingerprint ?? key.id}</small>
                  </>
                )
              },
              {
                key: "scope",
                header: adminCopy.apiKeys.columns.scope,
                render: (key) => `${key.tenant_id} / ${key.project_id}`
              },
              { key: "models", header: adminCopy.apiKeys.columns.models, render: (key) => modelLabel(key.allowed_models) },
              { key: "ip", header: adminCopy.apiKeys.columns.ip, render: (key) => ipLabel(key.ip_allowlist) },
              { key: "expires", header: adminCopy.apiKeys.columns.expires, render: (key) => dateLabel(key.expires_at) },
              { key: "usage", header: adminCopy.apiKeys.columns.usage, render: usageLabel },
              {
                key: "state",
                header: adminCopy.apiKeys.columns.state,
                render: (key) => (key.enabled ? adminCopy.apiKeys.state.enabled : adminCopy.apiKeys.state.disabled)
              },
              {
                key: "actions",
                header: "操作",
                render: (key) => (
                  <span className="inline-actions">
                    <Button
                      disabled={busy || !csrfToken || key.enabled}
                      onClick={() => mutateKey(key.id, "enable")}
                      variant="ghost"
                    >
                      {adminCopy.apiKeys.actions.enable}
                    </Button>
                    <Button
                      disabled={busy || !csrfToken || !key.enabled}
                      onClick={() => mutateKey(key.id, "disable")}
                      variant="ghost"
                    >
                      {adminCopy.apiKeys.actions.disable}
                    </Button>
                    <Button disabled={busy || !csrfToken} onClick={() => mutateKey(key.id, "rotate")} variant="ghost">
                      {adminCopy.apiKeys.actions.rotate}
                    </Button>
                  </span>
                )
              }
            ]}
            empty={<EmptyState title="暂无 API keys">当前 BFF 没有返回 API key。</EmptyState>}
            getRowKey={(key) => key.id}
            rowClassName="table-row api-key-row"
            rows={keys}
          />
        </article>

        <div className="api-key-detail-grid">
          <article className="api-key-section">
            <h3>{adminCopy.apiKeys.sections.rotation}</h3>
            <div className="api-key-secret-preview">
              <strong>{adminCopy.apiKeys.hints.oneTimeTitle}</strong>
              <span>{adminCopy.apiKeys.hints.oneTimeBody}</span>
              <code>{rotatedKey?.plaintext_key ?? adminCopy.apiKeys.hints.oneTimePlaceholder}</code>
              {rotatedKey?.plaintext_key ? <CopyButton value={rotatedKey.plaintext_key} /> : null}
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
