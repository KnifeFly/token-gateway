import { Button, EmptyState, LoadingState, StatusBadge, Toast } from "@token-gateway/ui";
import { useState } from "react";

import { adminCopy } from "../../shared/i18n";
import {
  exportAdminPlayground,
  previewAdminPlaygroundImport,
  runAdminPlayground,
  type AdminPlaygroundExport,
  type AdminPlaygroundImportPreview,
  type AdminPlaygroundRunResult
} from "./playgroundApi";

interface ToolsPlaygroundPanelProps {
  csrfToken: string;
}

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

function parsePayload(value: string): Record<string, unknown> {
  const parsed = JSON.parse(value) as unknown;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("payload must be a JSON object");
  }
  return parsed as Record<string, unknown>;
}

function describeError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function ToolsPlaygroundPanel({ csrfToken }: ToolsPlaygroundPanelProps) {
  const [model, setModel] = useState("gpt-4o-mini");
  const [mode, setMode] = useState("chat");
  const [stream, setStream] = useState(false);
  const [payload, setPayload] = useState('{"messages":[{"role":"user","content":"ping"}]}');
  const [result, setResult] = useState<AdminPlaygroundRunResult>();
  const [importPreview, setImportPreview] = useState<AdminPlaygroundImportPreview>();
  const [exportPreview, setExportPreview] = useState<AdminPlaygroundExport>();
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  function mutationReason(action: string) {
    return {
      csrfToken,
      reason: `P25 Admin playground ${action} ${model}`
    };
  }

  async function run(action: "run" | "import" | "export") {
    setBusy(true);
    setMessage("");
    try {
      const parsedPayload = parsePayload(payload);
      if (action === "run") {
        setResult(
          await runAdminPlayground(
            {
              debug: true,
              mode,
              model,
              payload: parsedPayload,
              stream
            },
            mutationReason("run")
          )
        );
      } else if (action === "import") {
        setImportPreview(await previewAdminPlaygroundImport({ payload: parsedPayload }, mutationReason("import")));
      } else {
        setExportPreview(await exportAdminPlayground({ mode, model, payload: parsedPayload }, mutationReason("export")));
      }
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="tools-panel panel" id="tools">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.tools.title}</h2>
          <p>{adminCopy.tools.subtitle}</p>
        </div>
        <StatusBadge tone="success">BFF connected</StatusBadge>
      </div>

      {message ? <Toast tone="danger">{message}</Toast> : null}
      {busy ? <LoadingState label="正在调用 playground BFF" /> : null}

      <div className="tools-layout">
        <article className="tools-section">
          <h3>{adminCopy.tools.sections.playground}</h3>
          <div className="stacked-form">
            <label>
              Model
              <input value={model} onChange={(event) => setModel(event.target.value)} />
            </label>
            <label>
              Mode
              <select value={mode} onChange={(event) => setMode(event.target.value)}>
                <option value="chat">chat</option>
                <option value="responses">responses</option>
                <option value="image">image</option>
              </select>
            </label>
            <label className="toggle-row">
              <input checked={stream} onChange={(event) => setStream(event.target.checked)} type="checkbox" />
              Stream
            </label>
            <label>
              Payload JSON
              <textarea rows={7} value={payload} onChange={(event) => setPayload(event.target.value)} />
            </label>
            <div className="inline-actions">
              <Button disabled={busy || !csrfToken} onClick={() => run("run")} variant="primary">
                {adminCopy.tools.actions.run}
              </Button>
              <Button disabled={busy || !csrfToken} onClick={() => run("import")} variant="ghost">
                {adminCopy.tools.actions.importPreview}
              </Button>
              <Button disabled={busy || !csrfToken} onClick={() => run("export")} variant="ghost">
                {adminCopy.tools.actions.export}
              </Button>
            </div>
          </div>
        </article>

        <article className="tools-section">
          <h3>{adminCopy.tools.sections.safeDebug}</h3>
          {result ? (
            <div className="debug-summary">
              <StatusBadge tone="success">{result.status}</StatusBadge>
              <strong>{result.result.summary}</strong>
              <dl>
                <div>
                  <dt>请求 ID</dt>
                  <dd>{result.request_id}</dd>
                </div>
                <div>
                  <dt>路由/渠道</dt>
                  <dd>
                    {result.debug.route_id ?? "-"} / {result.debug.channel_id ?? "-"}
                  </dd>
                </div>
                <div>
                  <dt>供应商</dt>
                  <dd>{result.debug.provider_type ?? "-"}</dd>
                </div>
                <div>
                  <dt>用量</dt>
                  <dd>{result.debug.usage.total_tokens} Token</dd>
                </div>
              </dl>
            </div>
          ) : (
            <EmptyState title="尚未运行">运行后展示 safe debug summary，不显示原始响应或凭证。</EmptyState>
          )}
        </article>
      </div>

      <div className="tools-layout">
        <article className="tools-section">
          <h3>{adminCopy.tools.sections.importExport}</h3>
          <pre className="operation-result">
            {JSON.stringify(
              {
                export: exportPreview,
                import_preview: importPreview
              },
              null,
              2
            )}
          </pre>
        </article>

        <article className="tools-section">
          <h3>{adminCopy.tools.sections.channelTest}</h3>
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
