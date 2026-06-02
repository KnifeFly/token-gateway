import { moneyValue } from "../../shared/format";
import { portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { CreditsResponse } from "../../shared/types";

export function CreditsView({ credits }: { credits?: CreditsResponse }) {
  const buckets = Object.entries(credits?.data ?? {});

  return (
    <section className="panel">
      <PanelHeading title={portalCopy.credits.title} />
      <div className="table" role="table">
        <div className="table-row table-head four" role="row">
          <span role="columnheader">{portalCopy.credits.bucketColumn}</span>
          <span role="columnheader">{portalCopy.credits.remainingColumn}</span>
          <span role="columnheader">{portalCopy.credits.usedColumn}</span>
          <span role="columnheader">{portalCopy.credits.heldColumn}</span>
        </div>
        {buckets.map(([name, bucket]) => (
          <div className="table-row four" key={name} role="row">
            <span role="cell">{name}</span>
            <span role="cell">
              {moneyValue(bucket.remaining_credits)} {bucket.currency}
            </span>
            <span role="cell">
              {moneyValue(bucket.used_credits)} {bucket.currency}
            </span>
            <span role="cell">
              {moneyValue(bucket.held_credits ?? 0)} {bucket.currency}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}
