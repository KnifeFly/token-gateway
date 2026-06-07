import { modelModeLabel, portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { ModelDetail, ModelList, ModelSchema, PortalSchemas } from "../../shared/types";

type ModelSummary = PortalSchemas["ModelSummary"];

export function ModelsView({
  modelDetail,
  modelSchema,
  models,
  onSelectModel,
  selectedModel
}: {
  modelDetail?: ModelDetail;
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
            <span role="columnheader">{portalCopy.models.categoryColumn}</span>
            <span role="columnheader">{portalCopy.models.modeColumn}</span>
            <span role="columnheader">{portalCopy.models.priceColumn}</span>
          </div>
          {(models?.data ?? []).map((model) => (
            <button
              className="table-row button-row portal-model-row"
              key={model.id}
              onClick={() => onSelectModel(model.id)}
              role="row"
              type="button"
            >
              <span role="cell">{model.display_name}</span>
              <span role="cell">{model.category ?? model.type}</span>
              <span role="cell">{modelModeLabel(model.async)}</span>
              <span role="cell">{priceSummary(model)}</span>
            </button>
          ))}
        </div>
      </article>
      <article className="panel">
        <PanelHeading title={portalCopy.models.previewTitle} meta={selectedModel} />
        {modelDetail ? (
          <div className="model-preview">
            <strong>{modelDetail.model.display_name}</strong>
            <span>{modelDetail.model.description ?? portalCopy.models.descriptionFallback}</span>
            <div className="tag-list">
              {(modelDetail.model.capabilities ?? []).map((capability) => (
                <span key={capability}>{capability}</span>
              ))}
            </div>
            <dl className="model-preview-meta">
              <div>
                <dt>{portalCopy.models.categoryColumn}</dt>
                <dd>{modelDetail.model.category ?? modelDetail.model.type}</dd>
              </div>
              <div>
                <dt>{portalCopy.models.priceColumn}</dt>
                <dd>{priceSummary(modelDetail.model)}</dd>
              </div>
              <div>
                <dt>{portalCopy.models.contextColumn}</dt>
                <dd>{modelDetail.model.context_window ?? "-"}</dd>
              </div>
            </dl>
          </div>
        ) : null}
        <PanelHeading title={portalCopy.models.schemaTitle} meta={selectedModel} />
        <pre className="schema-box">
          {modelSchema ? JSON.stringify(modelSchema.schema, null, 2) : "{}"}
        </pre>
      </article>
    </section>
  );
}

function priceSummary(model: ModelSummary): string {
  const price = model.pricing_summary;
  if (!price.configured) {
    return portalCopy.models.unpriced;
  }
  const units = price.components?.map((component) => component.unit).join(" / ");
  return `${price.currency ?? "-"} · ${units || portalCopy.models.componentPrice}`;
}
