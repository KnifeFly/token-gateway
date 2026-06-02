import { formatInteger } from "@token-gateway/format";

import { moneyValue } from "../../shared/format";
import { displayStatusLabel, portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { UsageResponse } from "../../shared/types";

export function UsageView({ usage }: { usage?: UsageResponse }) {
  return (
    <section className="panel">
      <PanelHeading title={portalCopy.usage.title} meta={usage?.generated_at} />
      <div className="table" role="table">
        <div className="table-row table-head usage" role="row">
          <span role="columnheader">{portalCopy.usage.statusColumn}</span>
          <span role="columnheader">{portalCopy.usage.modelColumn}</span>
          <span role="columnheader">{portalCopy.usage.inputColumn}</span>
          <span role="columnheader">{portalCopy.usage.outputColumn}</span>
          <span role="columnheader">{portalCopy.usage.creditsColumn}</span>
        </div>
        {(usage?.items ?? []).map((item, index) => (
          <div
            className="table-row usage"
            key={`${item.request_id ?? item.status}-${index}`}
            role="row"
          >
            <span role="cell">{displayStatusLabel(item.status)}</span>
            <span role="cell">{item.model ?? item.capability ?? portalCopy.defaults.ledger}</span>
            <span role="cell">{formatInteger(item.input_tokens)}</span>
            <span role="cell">{formatInteger(item.output_tokens)}</span>
            <span role="cell">{moneyValue(item.credits_used)}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
