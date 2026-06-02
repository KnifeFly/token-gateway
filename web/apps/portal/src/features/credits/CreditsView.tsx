import { moneyValue } from "../../shared/format";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { CreditsResponse } from "../../shared/types";

export function CreditsView({ credits }: { credits?: CreditsResponse }) {
  const buckets = Object.entries(credits?.data ?? {});

  return (
    <section className="panel">
      <PanelHeading title="Credits" />
      <div className="table" role="table">
        <div className="table-row table-head four" role="row">
          <span role="columnheader">Bucket</span>
          <span role="columnheader">Remaining</span>
          <span role="columnheader">Used</span>
          <span role="columnheader">Held</span>
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
