import { formatInteger } from "@token-gateway/format";

import { moneyValue } from "../../shared/format";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { UsageResponse } from "../../shared/types";

export function UsageView({ usage }: { usage?: UsageResponse }) {
  return (
    <section className="panel">
      <PanelHeading title="Usage" meta={usage?.generated_at} />
      <div className="table" role="table">
        <div className="table-row table-head usage" role="row">
          <span role="columnheader">Status</span>
          <span role="columnheader">Model</span>
          <span role="columnheader">Input</span>
          <span role="columnheader">Output</span>
          <span role="columnheader">Credits</span>
        </div>
        {(usage?.items ?? []).map((item, index) => (
          <div
            className="table-row usage"
            key={`${item.request_id ?? item.status}-${index}`}
            role="row"
          >
            <span role="cell">{item.status}</span>
            <span role="cell">{item.model ?? item.capability ?? "ledger"}</span>
            <span role="cell">{formatInteger(item.input_tokens)}</span>
            <span role="cell">{formatInteger(item.output_tokens)}</span>
            <span role="cell">{moneyValue(item.credits_used)}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
