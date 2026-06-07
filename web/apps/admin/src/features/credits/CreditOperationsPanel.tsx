import { formatInteger, formatMoney } from "@token-gateway/format";
import { Button, EmptyState, FilterBar, LoadingState, StatusBadge } from "@token-gateway/ui";
import { useEffect, useMemo, useState } from "react";

import { listAdminCustomerAccounts } from "../accounts/accountApi";
import { adminCopy } from "../../shared/i18n";
import {
  exportAdminCustomerLedger,
  exportAdminCustomerUsage,
  getAdminCustomerCreditReport,
  type AdminCustomerCreditReport,
  type AdminCustomerReportExport
} from "./creditOpsApi";

function money(micros?: number, currency = "CNY") {
  return `${currency} ${formatMoney((micros ?? 0) / 1_000_000, 4)}`;
}

function holdLabel(status: string) {
  return (adminCopy.creditOps.state as Record<string, string>)[status] ?? status;
}

function exportLabel(kind: string) {
  return (adminCopy.creditOps.exportKind as Record<string, string>)[kind] ?? kind;
}

function describeError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function CreditOperationsPanel() {
  const [accountIDs, setAccountIDs] = useState<string[]>([]);
  const [selectedAccountID, setSelectedAccountID] = useState("");
  const [report, setReport] = useState<AdminCustomerCreditReport>();
  const [lastExport, setLastExport] = useState<AdminCustomerReportExport>();
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  async function loadAccounts() {
    setLoading(true);
    setMessage("");
    try {
      const response = await listAdminCustomerAccounts();
      const ids = response.data.map((account) => account.customer_account_id);
      setAccountIDs(ids);
      setSelectedAccountID((current) => current || ids[0] || "");
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadAccounts();
  }, []);

  useEffect(() => {
    if (!selectedAccountID) {
      setReport(undefined);
      return;
    }
    let active = true;
    async function loadReport() {
      setBusy(true);
      try {
        const response = await getAdminCustomerCreditReport(selectedAccountID);
        if (active) {
          setReport(response);
          setLastExport(undefined);
        }
      } catch (error) {
        if (active) {
          setMessage(describeError(error));
          setReport(undefined);
        }
      } finally {
        if (active) {
          setBusy(false);
        }
      }
    }
    void loadReport();
    return () => {
      active = false;
    };
  }, [selectedAccountID]);

  const credit = useMemo(() => report?.credits[0], [report?.credits]);

  async function runExport(kind: "usage" | "ledger") {
    if (!selectedAccountID) {
      return;
    }
    setBusy(true);
    setMessage("");
    try {
      setLastExport(
        kind === "usage"
          ? await exportAdminCustomerUsage(selectedAccountID)
          : await exportAdminCustomerLedger(selectedAccountID)
      );
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel credit-ops-panel" id="credit-operations">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.creditOps.title}</h2>
          <p>{adminCopy.creditOps.subtitle}</p>
        </div>
        <StatusBadge tone="success">BFF connected</StatusBadge>
      </div>

      <FilterBar>
        <label>
          客户账户
          <select value={selectedAccountID} onChange={(event) => setSelectedAccountID(event.target.value)}>
            {accountIDs.map((accountID) => (
              <option key={accountID} value={accountID}>
                {accountID}
              </option>
            ))}
          </select>
        </label>
      </FilterBar>

      {message ? <div className="inline-alert">{message}</div> : null}
      {loading || busy ? <LoadingState label="正在加载额度运营数据" /> : null}

      {!report ? <EmptyState title="暂无额度报告">选择客户账户后从 BFF 加载 credit report。</EmptyState> : null}

      {report ? (
        <>
          <div className="credit-ops-grid">
            <article className="credit-ops-section">
              <h3>{adminCopy.creditOps.sections.balance}</h3>
              <dl>
                <div>
                  <dt>{adminCopy.creditOps.columns.available}</dt>
                  <dd>{money(credit?.available_micros, credit?.currency)}</dd>
                </div>
                <div>
                  <dt>{adminCopy.creditOps.columns.held}</dt>
                  <dd>{money(credit?.held_micros, credit?.currency)}</dd>
                </div>
                <div>
                  <dt>{adminCopy.creditOps.columns.used}</dt>
                  <dd>{money(credit?.used_micros, credit?.currency)}</dd>
                </div>
              </dl>
            </article>

            <article className="credit-ops-section">
              <h3>{adminCopy.creditOps.sections.holds}</h3>
              <div className="credit-ops-list">
                {report.active_holds.map((hold) => (
                  <div key={hold.status}>
                    <strong>{holdLabel(hold.status)}</strong>
                    <span>{formatInteger(hold.count)}</span>
                  </div>
                ))}
              </div>
            </article>

            <article className="credit-ops-section">
              <h3>{adminCopy.creditOps.sections.repairs}</h3>
              <div className="credit-ops-list">
                {report.failed_settlements.map((failed) => (
                  <div key={failed.id}>
                    <strong>{failed.request_id}</strong>
                    <span>
                      {adminCopy.creditOps.columns.retry} {formatInteger(failed.retry_count)}
                    </span>
                    <small>{failed.last_error}</small>
                  </div>
                ))}
                {report.failed_settlements.length === 0 ? (
                  <EmptyState title="暂无待修复结算">BFF 未返回 failed settlement link。</EmptyState>
                ) : null}
              </div>
            </article>
          </div>

          <div className="credit-ops-grid">
            <article className="credit-ops-section wide">
              <h3>{adminCopy.creditOps.sections.ledger}</h3>
              <div className="credit-ops-list">
                {report.ledger.map((line) => (
                  <div key={line.id}>
                    <strong>{holdLabel(line.settlement_kind)}</strong>
                    <span>{line.reason ?? line.request_id}</span>
                    <small>{money(line.amount_micros, line.currency)}</small>
                  </div>
                ))}
                {report.ledger.length === 0 ? <EmptyState title="暂无账本">BFF 未返回账本行。</EmptyState> : null}
              </div>
            </article>

            <article className="credit-ops-section wide">
              <h3>{adminCopy.creditOps.sections.exports}</h3>
              <div className="endpoint-list">
                {report.exports.map((item) => (
                  <code key={item.path}>
                    {exportLabel(item.kind)} · {item.path}
                  </code>
                ))}
              </div>
              <div className="inline-actions">
                <Button disabled={busy} onClick={() => runExport("usage")} variant="ghost">
                  导出 usage
                </Button>
                <Button disabled={busy} onClick={() => runExport("ledger")} variant="ghost">
                  导出 ledger
                </Button>
              </div>
              {lastExport ? (
                <p className="panel-note">
                  {adminCopy.creditOps.hints.exportFile} {lastExport.filename}
                </p>
              ) : null}
            </article>
          </div>
        </>
      ) : null}

      <div className="tools-hint-grid">
        <span>{adminCopy.creditOps.hints.reason}</span>
        <span>{adminCopy.creditOps.hints.audit}</span>
        <span>{adminCopy.creditOps.hints.safeExport}</span>
        <span>{adminCopy.creditOps.hints.noPayment}</span>
      </div>
    </section>
  );
}
