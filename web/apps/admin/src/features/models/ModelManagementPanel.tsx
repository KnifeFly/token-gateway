import { Button, DataTable, Drawer, EmptyState, LoadingState, StatusBadge } from "@token-gateway/ui";
import { useEffect, useMemo, useState } from "react";

import { adminCopy } from "../../shared/i18n";
import {
  deprecateAdminModel,
  disableAdminModel,
  getAdminModelSchemaPreview,
  listAdminModels,
  type AdminModelSchemaPreview,
  type AdminModelView
} from "./modelApi";

interface ModelManagementPanelProps {
  csrfToken: string;
}

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

function describeError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function ModelManagementPanel({ csrfToken }: ModelManagementPanelProps) {
  const [models, setModels] = useState<AdminModelView[]>([]);
  const [selectedModel, setSelectedModel] = useState("");
  const [schemaPreview, setSchemaPreview] = useState<AdminModelSchemaPreview>();
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  async function loadModels() {
    setLoading(true);
    setMessage("");
    try {
      const response = await listAdminModels();
      setModels(response.data);
      setSelectedModel((current) => current || response.data[0]?.public_model || "");
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadModels();
  }, []);

  const selected = useMemo(
    () => models.find((model) => model.public_model === selectedModel) ?? models[0],
    [models, selectedModel]
  );

  async function loadSchema(modelID: string) {
    setBusy(true);
    setMessage("");
    try {
      setSchemaPreview(await getAdminModelSchemaPreview(modelID));
      setSelectedModel(modelID);
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setBusy(false);
    }
  }

  async function mutateModel(modelID: string, action: "disable" | "deprecate") {
    setBusy(true);
    setMessage("");
    try {
      const mutation = {
        csrfToken,
        reason: `P25 Admin model ${action} ${modelID}`
      };
      if (action === "disable") {
        await disableAdminModel(modelID, mutation);
      } else {
        await deprecateAdminModel(modelID, mutation);
      }
      await loadModels();
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel model-panel" id="catalog">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.models.title}</h2>
          <p>{adminCopy.models.subtitle}</p>
        </div>
        <StatusBadge tone="success">BFF connected</StatusBadge>
      </div>

      {message ? <div className="inline-alert">{message}</div> : null}
      {loading ? <LoadingState label="正在加载模型目录" /> : null}

      <div className="model-layout">
        <article className="model-section">
          <h3>{adminCopy.models.sections.catalog}</h3>
          <DataTable
            ariaLabel={adminCopy.models.sections.catalog}
            className="model-table"
            columns={[
              {
                key: "model",
                header: adminCopy.models.columns.model,
                render: (model) => (
                  <>
                    <strong>{model.display_name ?? model.public_model}</strong>
                    <small>{model.public_model}</small>
                  </>
                )
              },
              { key: "category", header: adminCopy.models.columns.category, render: (model) => model.category ?? "-" },
              {
                key: "capability",
                header: adminCopy.models.columns.capability,
                render: (model) => model.capabilities?.join(" / ") || model.capability
              },
              { key: "price", header: adminCopy.models.columns.price, render: priceSummary },
              {
                key: "coverage",
                header: adminCopy.models.columns.coverage,
                render: (model) => model.channel_coverage?.length ?? 0
              },
              {
                key: "state",
                header: adminCopy.models.columns.state,
                render: (model) =>
                  model.enabled ? adminCopy.models.state.enabled : adminCopy.models.state.disabled
              },
              {
                key: "actions",
                header: "操作",
                render: (model) => (
                  <span className="inline-actions">
                    <Button disabled={busy} onClick={() => loadSchema(model.public_model)} variant="ghost">
                      {adminCopy.models.actions.schema}
                    </Button>
                    <Button
                      disabled={busy || !csrfToken || !model.enabled}
                      onClick={() => mutateModel(model.public_model, "disable")}
                      variant="ghost"
                    >
                      {adminCopy.models.actions.disable}
                    </Button>
                    <Button
                      disabled={busy || !csrfToken || model.deprecated}
                      onClick={() => mutateModel(model.public_model, "deprecate")}
                      variant="ghost"
                    >
                      {adminCopy.models.actions.deprecate}
                    </Button>
                  </span>
                )
              }
            ]}
            empty={<EmptyState title="暂无模型">当前 BFF 没有返回模型目录。</EmptyState>}
            getRowKey={(model) => model.public_model}
            rowClassName="table-row model-row"
            rows={models}
          />
        </article>

        <article className="model-section">
          <h3>{adminCopy.models.sections.coverage}</h3>
          <div className="coverage-grid">
            {(selected?.channel_coverage ?? []).map((coverage) => (
              <div className="coverage-item" key={`${selected?.public_model}-${coverage.channel_id}`}>
                <strong>{selected?.public_model}</strong>
                <span>{coverage.channel_id}</span>
                <small>
                  {coverage.provider_type} · {coverage.upstream_model}
                </small>
                <small>
                  {adminCopy.models.columns.health} {statusLabel(coverage.health_status)} ·{" "}
                  {adminCopy.models.columns.cost} {statusLabel(coverage.cost_config_status)}
                </small>
              </div>
            ))}
            {(selected?.channel_coverage ?? []).length === 0 ? (
              <EmptyState title="暂无渠道覆盖">选择模型后可查看 BFF 返回的覆盖情况。</EmptyState>
            ) : null}
          </div>
        </article>

        <article className="model-section">
          <h3>{adminCopy.models.sections.portalPreview}</h3>
          {selected ? (
            <div className="portal-preview-grid">
              <div className="portal-preview-item">
                <strong>{selected.display_name ?? selected.public_model}</strong>
                <span>{selected.description}</span>
                <div className="tag-list">
                  {(selected.input_modalities ?? []).map((item) => (
                    <span key={`${selected.public_model}-${item}`}>{item}</span>
                  ))}
                </div>
              </div>
            </div>
          ) : (
            <EmptyState title="暂无预览">当前没有可展示的模型。</EmptyState>
          )}
        </article>

        <article className="model-section">
          <h3>{adminCopy.models.actions.schema}</h3>
          {schemaPreview ? (
            <Drawer open title="Schema safe preview">
              <pre className="operation-result">{JSON.stringify(schemaPreview.schema, null, 2)}</pre>
            </Drawer>
          ) : (
            <EmptyState title="未加载 schema">点击模型行的 schema 操作查看 safe preview。</EmptyState>
          )}
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
