import { formatInteger } from "@token-gateway/format";
import type { FormEvent } from "react";

import { moneyValue } from "../../shared/format";
import { displayStatusLabel, portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { UsageResponse } from "../../shared/types";

type ActivityFilters = {
  apiKeyID: string;
  requestID: string;
  model: string;
  providerType: string;
  channelID: string;
  status: string;
  from: string;
  to: string;
};

type UsageViewProps = {
  filters: ActivityFilters;
  onApplyFilters: (event: FormEvent<HTMLFormElement>) => void;
  onFiltersChange: (filters: ActivityFilters) => void;
  usage?: UsageResponse;
};

export function UsageView({ filters, onApplyFilters, onFiltersChange, usage }: UsageViewProps) {
  function updateFilter(key: keyof ActivityFilters, value: string) {
    onFiltersChange({ ...filters, [key]: value });
  }

  return (
    <section className="panel">
      <PanelHeading title={portalCopy.usage.title} meta={usage?.generated_at} />
      <form className="filter-grid" onSubmit={onApplyFilters}>
        <label>
          <span>{portalCopy.usage.requestIDLabel}</span>
          <input
            onChange={(event) => updateFilter("requestID", event.target.value)}
            value={filters.requestID}
          />
        </label>
        <label>
          <span>{portalCopy.usage.apiKeyLabel}</span>
          <input
            onChange={(event) => updateFilter("apiKeyID", event.target.value)}
            value={filters.apiKeyID}
          />
        </label>
        <label>
          <span>{portalCopy.usage.modelColumn}</span>
          <input
            onChange={(event) => updateFilter("model", event.target.value)}
            value={filters.model}
          />
        </label>
        <label>
          <span>{portalCopy.usage.channelColumn}</span>
          <input
            onChange={(event) => updateFilter("channelID", event.target.value)}
            value={filters.channelID}
          />
        </label>
        <label>
          <span>{portalCopy.usage.statusColumn}</span>
          <input
            onChange={(event) => updateFilter("status", event.target.value)}
            value={filters.status}
          />
        </label>
        <button className="tg-button tg-button--secondary" type="submit">
          {portalCopy.usage.filterAction}
        </button>
      </form>
      <div className="table" role="table">
        <div className="table-row table-head usage" role="row">
          <span role="columnheader">{portalCopy.usage.requestColumn}</span>
          <span role="columnheader">{portalCopy.usage.statusColumn}</span>
          <span role="columnheader">{portalCopy.usage.modelColumn}</span>
          <span role="columnheader">{portalCopy.usage.channelColumn}</span>
          <span role="columnheader">{portalCopy.usage.inputColumn}</span>
          <span role="columnheader">{portalCopy.usage.outputColumn}</span>
          <span role="columnheader">{portalCopy.usage.creditsColumn}</span>
        </div>
        {(usage?.items ?? []).length === 0 ? (
          <div className="table-row table-empty usage" role="row">
            <span role="cell">{portalCopy.usage.empty}</span>
          </div>
        ) : null}
        {(usage?.items ?? []).map((item, index) => (
          <div
            className="table-row usage"
            key={`${item.request_id ?? item.status}-${index}`}
            role="row"
          >
            <span role="cell">{item.request_id ?? portalCopy.usage.aggregateRow}</span>
            <span role="cell">{displayStatusLabel(item.status)}</span>
            <span role="cell">{item.model ?? item.capability ?? portalCopy.defaults.ledger}</span>
            <span role="cell">{item.channel_id ?? item.provider_type ?? portalCopy.usage.aggregateRow}</span>
            <span role="cell">{formatInteger(item.input_tokens)}</span>
            <span role="cell">{formatInteger(item.output_tokens)}</span>
            <span role="cell">{moneyValue(item.credits_used)}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
