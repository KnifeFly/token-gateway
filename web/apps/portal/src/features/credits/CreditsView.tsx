import { FilterBar, Pagination } from "@token-gateway/ui";

import { formatMaybeDate, moneyValue } from "../../shared/format";
import { portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { CreditLedgerResponse, CreditsResponse, UsageExportResponse } from "../../shared/types";

type CreditsViewProps = {
  credits?: CreditsResponse;
  ledger?: CreditLedgerResponse;
  ledgerLimit: number;
  onLedgerLimitChange: (limit: number) => void;
  usageExport?: UsageExportResponse;
};

export function CreditsView({
  credits,
  ledger,
  ledgerLimit,
  onLedgerLimitChange,
  usageExport
}: CreditsViewProps) {
  const buckets = Object.entries(credits?.data ?? {});

  return (
    <section className="panel credits-panel">
      <PanelHeading title={portalCopy.credits.title} meta={usageExport?.generated_at} />
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

      <div className="credits-detail-grid">
        <article>
          <h3>{portalCopy.credits.ledgerTitle}</h3>
          <FilterBar>
            <Pagination
              limit={ledgerLimit}
              onLimitChange={onLedgerLimitChange}
              summary={`${ledger?.items?.length ?? 0} rows`}
            />
          </FilterBar>
          <div className="credits-ledger-list">
            {(ledger?.items ?? []).length === 0 ? (
              <p className="panel-note">{portalCopy.credits.emptyLedger}</p>
            ) : null}
            {(ledger?.items ?? []).map((item) => (
              <div key={item.id}>
                <strong>{ledgerKindLabel(item.settlement_kind)}</strong>
                <span>{item.reason || item.request_id || item.id}</span>
                <small>
                  {moneyValue(item.amount_credits)} {item.currency} · {formatMaybeDate(item.created_at)}
                </small>
              </div>
            ))}
          </div>
        </article>

        <article>
          <h3>{portalCopy.credits.exportTitle}</h3>
          <dl className="credits-export-summary">
            <div>
              <dt>{portalCopy.credits.exportFileColumn}</dt>
              <dd>{usageExport?.filename ?? "-"}</dd>
            </div>
            <div>
              <dt>{portalCopy.credits.exportFormatColumn}</dt>
              <dd>{usageExport?.format ?? "-"}</dd>
            </div>
            <div>
              <dt>{portalCopy.credits.exportRowsColumn}</dt>
              <dd>{(usageExport?.usage ?? []).length + (usageExport?.ledger ?? []).length}</dd>
            </div>
          </dl>
          <div className="tag-list">
            {portalCopy.credits.guardrails.map((item) => (
              <span key={item}>{item}</span>
            ))}
          </div>
        </article>
      </div>
    </section>
  );
}

function ledgerKindLabel(kind: string): string {
  return (portalCopy.credits.kindLabel as Record<string, string>)[kind] ?? kind;
}
