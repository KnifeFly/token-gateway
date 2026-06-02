import { modelModeLabel, portalCopy } from "../../shared/i18n";
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
        <PanelHeading title={portalCopy.models.title} />
        <div className="table" role="table">
          <div className="table-row table-head" role="row">
            <span role="columnheader">{portalCopy.models.modelColumn}</span>
            <span role="columnheader">{portalCopy.models.typeColumn}</span>
            <span role="columnheader">{portalCopy.models.modeColumn}</span>
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
              <span role="cell">{modelModeLabel(model.async)}</span>
            </button>
          ))}
        </div>
      </article>
      <article className="panel">
        <PanelHeading title={portalCopy.models.schemaTitle} meta={selectedModel} />
        <pre className="schema-box">
          {modelSchema ? JSON.stringify(modelSchema.schema, null, 2) : "{}"}
        </pre>
      </article>
    </section>
  );
}
