import { formatInteger } from "@token-gateway/format";
import { StatusBadge } from "@token-gateway/ui";
import { type FormEvent, useEffect, useMemo, useState } from "react";

import { portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { ModelList, ModelSchema, PlaygroundRunResult, PortalSchemas } from "../../shared/types";

type PlaygroundViewProps = {
  busy: boolean;
  modelSchema?: ModelSchema;
  models?: ModelList;
  onRun: (request: PortalSchemas["PlaygroundRunRequest"]) => void;
  onSelectModel: (modelID: string) => void;
  result?: PlaygroundRunResult;
  selectedModel: string;
};

type SchemaField = {
  id: string;
  label: string;
  required: boolean;
};

const fieldLabels: Record<string, string> = {
  input: "输入",
  model_params: "模型参数"
};

export function PlaygroundView({
  busy,
  modelSchema,
  models,
  onRun,
  onSelectModel,
  result,
  selectedModel
}: PlaygroundViewProps) {
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({});
  const [mode, setMode] = useState("chat");
  const [stream, setStream] = useState(false);

  const fields = useMemo(() => schemaFields(modelSchema), [modelSchema]);

  useEffect(() => {
    setFieldValues({});
  }, [selectedModel, modelSchema]);

  function updateField(field: string, value: string) {
    setFieldValues((current) => ({ ...current, [field]: value }));
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const payload: Record<string, unknown> = { model: selectedModel };
    for (const field of fields) {
      const value = fieldValues[field.id]?.trim();
      if (value) {
        payload[field.id] = value;
      }
    }
    onRun({ model: selectedModel, mode, stream, debug: true, payload });
  }

  return (
    <section className="panel-grid split">
      <article className="panel">
        <PanelHeading title={portalCopy.playground.title} />
        <p className="panel-note">{portalCopy.playground.description}</p>
        <form className="playground-form" onSubmit={handleSubmit}>
          <label>
            <span>{portalCopy.playground.modelLabel}</span>
            <select
              onChange={(event) => onSelectModel(event.target.value)}
              value={selectedModel}
            >
              {(models?.data ?? []).map((model) => (
                <option key={model.id} value={model.id}>
                  {model.display_name || model.id}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>{portalCopy.playground.modeLabel}</span>
            <select onChange={(event) => setMode(event.target.value)} value={mode}>
              <option value="chat">聊天</option>
              <option value="responses">响应</option>
              <option value="embeddings">向量</option>
              <option value="images">图片</option>
              <option value="audio">音频</option>
            </select>
          </label>
          <label className="checkbox-line">
            <input
              checked={stream}
              onChange={(event) => setStream(event.target.checked)}
              type="checkbox"
            />
            <span>{portalCopy.playground.streamLabel}</span>
            <small>{portalCopy.playground.streamHint}</small>
          </label>
          <div className="schema-fields">
            <div className="section-title">{portalCopy.playground.schemaTitle}</div>
            {fields.length === 0 ? (
              <p className="panel-note">{portalCopy.playground.noSchemaFields}</p>
            ) : null}
            {fields.map((field) => (
              <label key={field.id}>
                <span>
                  {field.label}
                  <small>
                    {field.required
                      ? portalCopy.playground.requiredBadge
                      : portalCopy.playground.optionalBadge}
                  </small>
                </span>
                <input
                  onChange={(event) => updateField(field.id, event.target.value)}
                  placeholder={portalCopy.playground.fieldPlaceholder}
                  value={fieldValues[field.id] ?? ""}
                />
              </label>
            ))}
          </div>
          <button className="tg-button" disabled={busy || !selectedModel} type="submit">
            {portalCopy.playground.runAction}
          </button>
        </form>
      </article>

      <article className="panel">
        <PanelHeading title={portalCopy.playground.resultTitle} />
        {result ? <PlaygroundResult result={result} /> : <p>{portalCopy.playground.noResult}</p>}
        <PanelHeading title={portalCopy.playground.guardrailTitle} />
        <div className="tag-list">
          {portalCopy.playground.guardrailItems.map((item) => (
            <span key={item}>{item}</span>
          ))}
        </div>
      </article>
    </section>
  );
}

function PlaygroundResult({ result }: { result: PlaygroundRunResult }) {
  return (
    <div className="playground-result">
      <div className="result-line">
        <StatusBadge tone={result.status === "ready" ? "success" : "warning"}>
          {statusLabel(result.status)}
        </StatusBadge>
        <strong>{result.result.summary}</strong>
      </div>
      <dl>
        <div>
          <dt>{portalCopy.playground.routeLabel}</dt>
          <dd>{result.debug.route_id || result.request_id}</dd>
        </div>
        <div>
          <dt>{portalCopy.playground.channelLabel}</dt>
          <dd>{result.debug.channel_id || "-"}</dd>
        </div>
        <div>
          <dt>{portalCopy.playground.providerLabel}</dt>
          <dd>{result.debug.provider_type || "-"}</dd>
        </div>
        <div>
          <dt>{portalCopy.playground.latencyLabel}</dt>
          <dd>{formatInteger(result.debug.latency_ms)} ms</dd>
        </div>
        <div>
          <dt>{portalCopy.playground.usageLabel}</dt>
          <dd>{formatInteger(result.debug.usage.total_tokens)}</dd>
        </div>
      </dl>
      {result.debug.safe_error_message ? (
        <p className="panel-note">{result.debug.safe_error_message}</p>
      ) : null}
    </div>
  );
}

function schemaFields(modelSchema?: ModelSchema): SchemaField[] {
  const schema = modelSchema?.schema as
    | { properties?: Record<string, unknown>; required?: string[] }
    | undefined;
  const properties = schema?.properties ?? {};
  const required = new Set(schema?.required ?? []);
  return Object.keys(properties)
    .filter((field) => field !== "model" && !sensitiveField(field))
    .sort()
    .map((field) => ({
      id: field,
      label: fieldLabels[field] ?? field,
      required: required.has(field)
    }));
}

function sensitiveField(field: string): boolean {
  const normalized = field.toLowerCase();
  return (
    normalized.includes("key") ||
    normalized.includes("secret") ||
    normalized.includes("credential") ||
    normalized.includes("token")
  );
}

function statusLabel(status: string): string {
  if (status === "ready") {
    return portalCopy.playground.statusReady;
  }
  if (status === "invalid") {
    return portalCopy.playground.statusInvalid;
  }
  return portalCopy.playground.statusWarning;
}
