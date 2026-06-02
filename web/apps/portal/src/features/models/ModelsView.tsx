import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { ModelList, ModelSchema } from "../../shared/types";

export function ModelsView({
  modelSchema,
  models,
  onSelectModel,
  selectedModel
}: {
  modelSchema?: ModelSchema;
  models?: ModelList;
  onSelectModel(modelID: string): void;
  selectedModel: string;
}) {
  return (
    <section className="panel-grid split">
      <article className="panel">
        <PanelHeading title="Models" />
        <div className="table" role="table">
          <div className="table-row table-head" role="row">
            <span role="columnheader">Model</span>
            <span role="columnheader">Type</span>
            <span role="columnheader">Mode</span>
          </div>
          {(models?.data ?? []).map((model) => (
            <button
              className="table-row button-row"
              key={model.id}
              onClick={() => onSelectModel(model.id)}
              role="row"
              type="button"
            >
              <span role="cell">{model.display_name}</span>
              <span role="cell">{model.type}</span>
              <span role="cell">{model.async ? "Async" : "Sync"}</span>
            </button>
          ))}
        </div>
      </article>
      <article className="panel">
        <PanelHeading title="Schema" meta={selectedModel} />
        <pre className="schema-box">
          {modelSchema ? JSON.stringify(modelSchema.schema, null, 2) : "{}"}
        </pre>
      </article>
    </section>
  );
}
