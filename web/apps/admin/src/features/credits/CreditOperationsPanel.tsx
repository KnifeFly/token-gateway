import { formatInteger, formatMoney } from "@token-gateway/format";
import { StatusBadge } from "@token-gateway/ui";

import { adminCopy } from "../../shared/i18n";
import type { AdminCustomerCreditReport, AdminCustomerReportExport } from "./creditOpsApi";

const sampleReport: AdminCustomerCreditReport = {
  generated_at: new Date().toISOString(),
  account: {
    customer_account_id: "tenant_acme:project_platform",
    tenant_id: "tenant_acme",
    tenant_name: "Acme 智能客服",
    project_id: "project_platform",
    project_name: "生产调用",
    status: "active",
    role: "owner",
    tenant_enabled: true,
    project_enabled: true,
    api_key_count: 2,
    active_api_key_count: 1,
    allowed_models_summary: { wildcard: false, unique_count: 2, models: ["gpt-4o-mini", "qwen-plus"] },
    recent_usage: {
      requests: 1842,
      input_tokens: 2205000,
      output_tokens: 642000,
      revenue_micros: 85000000,
      currency: "CNY"
    }
  },
  credits: [
    {
      account_id: "balance_acme_cny",
      currency: "CNY",
      available_micros: 1285000000,
      held_micros: 5000000,
      opening_micros: 1000000000,
      total_granted_micros: 1500000000,
      used_micros: 215000000
    }
  ],
  usage: [
    {
      model: "gpt-4o-mini",
      provider_type: "openai",
      channel_id: "channel_openai_primary",
      currency: "CNY",
      requests: 1620,
      input_tokens: 1984000,
      output_tokens: 588000,
      total_tokens: 2568000,
      amount_micros: 76000000
    }
  ],
  ledger: [
    {
      id: "ledger_acme_adjust",
      settlement_kind: "manual_adjustment",
      account_id: "balance_acme_cny",
      currency: "CNY",
      amount_micros: 500000000,
      balance_after_micros: 1285000000,
      reason: "月初运营补充",
      created_at: "2026-06-01T09:00:00Z"
    }
  ],
  active_holds: [
    { status: "active", count: 1 },
    { status: "protected_task_holds", count: 0 }
  ],
  failed_settlements: [
    {
      id: "fail_req_7251",
      request_id: "req_acme_repair",
      status: "pending",
      retry_count: 1,
      last_error: "结算修复等待重放",
      updated_at: "2026-06-02T08:35:00Z"
    }
  ],
  exports: [
    {
      kind: "usage",
      path: "/api/admin/v1/customer-accounts/tenant_acme:project_platform/usage/export",
      format: "json",
      safe_fields: ["model", "provider_type", "channel_id", "requests", "tokens", "amount_micros"]
    },
    {
      kind: "ledger",
      path: "/api/admin/v1/customer-accounts/tenant_acme:project_platform/ledger/export",
      format: "json",
      safe_fields: ["request_id", "settlement_kind", "currency", "amount_micros", "balance_after_micros"]
    }
  ]
};

const sampleExport: AdminCustomerReportExport = {
  generated_at: new Date().toISOString(),
  kind: "ledger",
  format: "json",
  filename: "customer_tenant_acme_project_platform_ledger.json",
  customer_account_id: "tenant_acme:project_platform",
  tenant_id: "tenant_acme",
  project_id: "project_platform",
  currency: "CNY",
  ledger: sampleReport.ledger,
  totals: sampleReport.account.recent_usage,
  safe_fields: sampleReport.exports[1]?.safe_fields ?? []
};

function money(micros?: number, currency = "CNY") {
  return `${currency} ${formatMoney((micros ?? 0) / 1_000_000, 4)}`;
}

function holdLabel(status: string) {
  return (adminCopy.creditOps.state as Record<string, string>)[status] ?? status;
}

function exportLabel(kind: string) {
  return (adminCopy.creditOps.exportKind as Record<string, string>)[kind] ?? kind;
}

export function CreditOperationsPanel() {
  const credit = sampleReport.credits[0];

  return (
    <section className="panel credit-ops-panel" id="credit-operations">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.creditOps.title}</h2>
          <p>{adminCopy.creditOps.subtitle}</p>
        </div>
        <StatusBadge tone="warning">{adminCopy.creditOps.status}</StatusBadge>
      </div>

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
            {sampleReport.active_holds.map((hold) => (
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
            {sampleReport.failed_settlements.map((failed) => (
              <div key={failed.id}>
                <strong>{failed.request_id}</strong>
                <span>
                  {adminCopy.creditOps.columns.retry} {formatInteger(failed.retry_count)}
                </span>
                <small>{failed.last_error}</small>
              </div>
            ))}
          </div>
        </article>
      </div>

      <div className="credit-ops-grid">
        <article className="credit-ops-section wide">
          <h3>{adminCopy.creditOps.sections.ledger}</h3>
          <div className="credit-ops-list">
            {sampleReport.ledger.map((line) => (
              <div key={line.id}>
                <strong>{holdLabel(line.settlement_kind)}</strong>
                <span>{line.reason ?? line.request_id}</span>
                <small>{money(line.amount_micros, line.currency)}</small>
              </div>
            ))}
          </div>
        </article>

        <article className="credit-ops-section wide">
          <h3>{adminCopy.creditOps.sections.exports}</h3>
          <div className="endpoint-list">
            {sampleReport.exports.map((item) => (
              <code key={item.path}>
                {exportLabel(item.kind)} · {item.path}
              </code>
            ))}
          </div>
          <p className="panel-note">
            {adminCopy.creditOps.hints.exportFile} {sampleExport.filename}
          </p>
        </article>
      </div>

      <div className="tools-hint-grid">
        <span>{adminCopy.creditOps.hints.reason}</span>
        <span>{adminCopy.creditOps.hints.audit}</span>
        <span>{adminCopy.creditOps.hints.safeExport}</span>
        <span>{adminCopy.creditOps.hints.noPayment}</span>
      </div>
    </section>
  );
}
