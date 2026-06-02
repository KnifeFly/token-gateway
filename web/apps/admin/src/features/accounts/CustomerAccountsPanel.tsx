import { formatISODateTime, formatInteger, formatMoney } from "@token-gateway/format";
import { StatusBadge } from "@token-gateway/ui";

import { adminCopy } from "../../shared/i18n";
import type { AdminCustomerAccountDetail, AdminCustomerAccountView } from "./accountApi";

const sampleAccounts: AdminCustomerAccountDetail[] = [
  {
    account: {
      customer_account_id: "tenant_acme:project_platform",
      tenant_id: "tenant_acme",
      tenant_name: "Acme 智能客服",
      project_id: "project_platform",
      project_name: "生产调用",
      display_name: "Acme 主账户",
      email: "ops@acme.example",
      status: "active",
      role: "owner",
      notes: "生产项目，允许默认聊天与中文备用模型。",
      tenant_enabled: true,
      project_enabled: true,
      api_key_count: 2,
      active_api_key_count: 1,
      allowed_models_summary: {
        models: ["gpt-4o-mini", "qwen-plus"],
        wildcard: false,
        unique_count: 2
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
      recent_usage: {
        requests: 1842,
        input_tokens: 2205000,
        output_tokens: 642000,
        revenue_micros: 85000000,
        currency: "CNY"
      },
      last_seen_at: "2026-06-02T08:20:00Z",
      created_at: "2026-05-20T03:12:00Z",
      updated_at: "2026-06-02T08:30:00Z"
    },
    api_keys: [
      {
        id: "key_acme_live",
        tenant_id: "tenant_acme",
        project_id: "project_platform",
        name: "生产环境",
        enabled: true,
        allowed_models: ["gpt-4o-mini", "qwen-plus"],
        usage_summary: {
          requests: 1620,
          input_tokens: 1984000,
          output_tokens: 588000,
          revenue_micros: 76000000,
          currency: "CNY"
        },
        created_at: "2026-05-20T03:13:00Z",
        updated_at: "2026-06-01T10:42:00Z"
      },
      {
        id: "key_acme_disabled",
        tenant_id: "tenant_acme",
        project_id: "project_platform",
        name: "旧密钥",
        enabled: false,
        allowed_models: ["gpt-4o-mini"],
        usage_summary: {
          requests: 222,
          input_tokens: 221000,
          output_tokens: 54000,
          revenue_micros: 9000000,
          currency: "CNY"
        },
        revoked_at: "2026-06-01T10:40:00Z",
        created_at: "2026-05-20T03:13:00Z",
        updated_at: "2026-06-01T10:40:00Z"
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
      },
      {
        model: "qwen-plus",
        provider_type: "dashscope",
        channel_id: "channel_qwen_backup",
        currency: "CNY",
        requests: 222,
        input_tokens: 221000,
        output_tokens: 54000,
        total_tokens: 275000,
        amount_micros: 9000000
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
      },
      {
        id: "ledger_acme_usage",
        request_id: "req_acme_8271",
        settlement_kind: "usage_settlement",
        account_id: "balance_acme_cny",
        currency: "CNY",
        amount_micros: -85000000,
        balance_after_micros: 785000000,
        reason: "模型调用结算",
        created_at: "2026-06-02T08:25:00Z"
      }
    ]
  },
  {
    account: {
      customer_account_id: "tenant_nova:project_sandbox",
      tenant_id: "tenant_nova",
      tenant_name: "Nova 数据实验室",
      project_id: "project_sandbox",
      project_name: "验证环境",
      display_name: "Nova 验证账户",
      email: "developer@nova.example",
      status: "disabled",
      role: "developer",
      notes: "验证环境暂停，保留额度和审计记录。",
      tenant_enabled: true,
      project_enabled: false,
      api_key_count: 1,
      active_api_key_count: 0,
      allowed_models_summary: {
        models: ["gpt-4o-mini"],
        wildcard: false,
        unique_count: 1
      },
      credits: [
        {
          account_id: "balance_nova_cny",
          currency: "CNY",
          available_micros: 240000000,
          held_micros: 0,
          opening_micros: 300000000,
          total_granted_micros: 300000000,
          used_micros: 60000000
        }
      ],
      recent_usage: {
        requests: 96,
        input_tokens: 82000,
        output_tokens: 19000,
        revenue_micros: 60000000,
        currency: "CNY"
      },
      last_seen_at: "2026-05-30T11:04:00Z",
      created_at: "2026-05-18T04:48:00Z",
      updated_at: "2026-06-01T14:18:00Z"
    },
    api_keys: [
      {
        id: "key_nova_sandbox",
        tenant_id: "tenant_nova",
        project_id: "project_sandbox",
        name: "验证密钥",
        enabled: false,
        allowed_models: ["gpt-4o-mini"],
        usage_summary: {
          requests: 96,
          input_tokens: 82000,
          output_tokens: 19000,
          revenue_micros: 60000000,
          currency: "CNY"
        },
        revoked_at: "2026-06-01T14:18:00Z",
        created_at: "2026-05-18T04:49:00Z",
        updated_at: "2026-06-01T14:18:00Z"
      }
    ],
    usage: [],
    ledger: []
  }
];

const workflowRows = [
  { label: adminCopy.accounts.actions.create, endpoint: "POST /api/admin/v1/customer-accounts" },
  {
    label: adminCopy.accounts.actions.enable,
    endpoint: "POST /api/admin/v1/customer-accounts/{id}/enable"
  },
  {
    label: adminCopy.accounts.actions.disable,
    endpoint: "POST /api/admin/v1/customer-accounts/{id}/disable"
  },
  {
    label: adminCopy.accounts.actions.adjustCredit,
    endpoint: "POST /api/admin/v1/customer-accounts/{id}/manual-adjustment"
  },
  {
    label: adminCopy.accounts.actions.resetSession,
    endpoint: "POST /api/admin/v1/customer-accounts/{id}/reset-session"
  }
];

function accountStatusLabel(status: AdminCustomerAccountView["status"]): string {
  return adminCopy.accounts.state[status] ?? status;
}

function accountRoleLabel(role: AdminCustomerAccountView["role"]): string {
  return adminCopy.accounts.role[role] ?? role;
}

function creditValue(micros: number | undefined, currency = "CNY"): string {
  return `${currency} ${formatMoney((micros ?? 0) / 1_000_000, 4)}`;
}

function primaryCredit(account: AdminCustomerAccountView) {
  return account.credits?.[0];
}

function modelScope(account: AdminCustomerAccountView): string {
  if (account.allowed_models_summary.wildcard) {
    return adminCopy.accounts.state.allModels;
  }
  if (account.allowed_models_summary.unique_count === 0) {
    return adminCopy.accounts.state.noModels;
  }
  return `${account.allowed_models_summary.unique_count} ${adminCopy.accounts.state.modelCount}`;
}

function safeDate(value?: string | null): string {
  return value ? formatISODateTime(value) : adminCopy.accounts.state.never;
}

export function CustomerAccountsPanel() {
  const selected = sampleAccounts[0];
  const selectedCredit = primaryCredit(selected.account);

  return (
    <section className="panel account-panel" id="accounts">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.accounts.title}</h2>
          <p>{adminCopy.accounts.subtitle}</p>
        </div>
        <StatusBadge tone="warning">{adminCopy.accounts.status}</StatusBadge>
      </div>

      <div className="account-layout">
        <article className="account-section">
          <h3>{adminCopy.accounts.sections.list}</h3>
          <div className="table account-table" role="table">
            <div className="table-row table-head account-row" role="row">
              <span role="columnheader">{adminCopy.accounts.columns.account}</span>
              <span role="columnheader">{adminCopy.accounts.columns.status}</span>
              <span role="columnheader">{adminCopy.accounts.columns.role}</span>
              <span role="columnheader">{adminCopy.accounts.columns.credits}</span>
              <span role="columnheader">{adminCopy.accounts.columns.keys}</span>
              <span role="columnheader">{adminCopy.accounts.columns.models}</span>
              <span role="columnheader">{adminCopy.accounts.columns.usage}</span>
            </div>
            {sampleAccounts.map(({ account }) => {
              const credit = primaryCredit(account);
              return (
                <div className="table-row account-row" key={account.customer_account_id} role="row">
                  <span role="cell">
                    <strong>{account.display_name ?? account.project_name}</strong>
                    <small>
                      {account.tenant_name} / {account.project_name}
                    </small>
                  </span>
                  <span role="cell">{accountStatusLabel(account.status)}</span>
                  <span role="cell">{accountRoleLabel(account.role)}</span>
                  <span role="cell">{creditValue(credit?.available_micros, credit?.currency)}</span>
                  <span role="cell">
                    {account.active_api_key_count}/{account.api_key_count}
                  </span>
                  <span role="cell">{modelScope(account)}</span>
                  <span role="cell">{formatInteger(account.recent_usage.requests)}</span>
                </div>
              );
            })}
          </div>
        </article>

        <div className="account-detail-grid">
          <article className="account-section">
            <h3>{adminCopy.accounts.sections.overview}</h3>
            <dl className="account-summary-grid">
              <div>
                <dt>{adminCopy.accounts.columns.tenantProject}</dt>
                <dd>
                  {selected.account.tenant_id} / {selected.account.project_id}
                </dd>
              </div>
              <div>
                <dt>{adminCopy.accounts.columns.lastSeen}</dt>
                <dd>{safeDate(selected.account.last_seen_at)}</dd>
              </div>
              <div>
                <dt>{adminCopy.accounts.columns.email}</dt>
                <dd>{selected.account.email}</dd>
              </div>
              <div>
                <dt>{adminCopy.accounts.columns.notes}</dt>
                <dd>{selected.account.notes}</dd>
              </div>
            </dl>
          </article>

          <article className="account-section">
            <h3>{adminCopy.accounts.sections.credits}</h3>
            <dl className="account-summary-grid">
              <div>
                <dt>{adminCopy.accounts.columns.available}</dt>
                <dd>{creditValue(selectedCredit?.available_micros, selectedCredit?.currency)}</dd>
              </div>
              <div>
                <dt>{adminCopy.accounts.columns.held}</dt>
                <dd>{creditValue(selectedCredit?.held_micros, selectedCredit?.currency)}</dd>
              </div>
              <div>
                <dt>{adminCopy.accounts.columns.granted}</dt>
                <dd>{creditValue(selectedCredit?.total_granted_micros, selectedCredit?.currency)}</dd>
              </div>
              <div>
                <dt>{adminCopy.accounts.columns.used}</dt>
                <dd>{creditValue(selectedCredit?.used_micros, selectedCredit?.currency)}</dd>
              </div>
            </dl>
            <div className="account-action-strip">
              <strong>{adminCopy.accounts.actions.adjustCredit}</strong>
              <span>{adminCopy.accounts.hints.adjustment}</span>
            </div>
          </article>
        </div>

        <article className="account-section">
          <h3>{adminCopy.accounts.sections.keys}</h3>
          <div className="coverage-grid">
            {selected.api_keys.map((key) => (
              <div className="coverage-item" key={key.id}>
                <strong>{key.name}</strong>
                <span>{key.id}</span>
                <small>
                  {key.enabled ? adminCopy.accounts.state.enabled : adminCopy.accounts.state.disabled} ·{" "}
                  {safeDate(key.updated_at)}
                </small>
                <small>{(key.allowed_models ?? []).join(" / ") || adminCopy.accounts.state.noModels}</small>
              </div>
            ))}
          </div>
        </article>

        <div className="account-detail-grid">
          <article className="account-section">
            <h3>{adminCopy.accounts.sections.usage}</h3>
            <div className="account-metric-list">
              {(selected.usage ?? []).map((row) => (
                <div key={`${row.model}-${row.channel_id}`}>
                  <strong>{row.model}</strong>
                  <span>
                    {row.channel_id} · {formatInteger(row.requests)} {adminCopy.accounts.columns.requests}
                  </span>
                  <small>{creditValue(row.amount_micros, row.currency)}</small>
                </div>
              ))}
            </div>
          </article>

          <article className="account-section">
            <h3>{adminCopy.accounts.sections.ledger}</h3>
            <div className="account-metric-list">
              {(selected.ledger ?? []).map((line) => (
                <div key={line.id}>
                  <strong>{line.settlement_kind}</strong>
                  <span>{line.reason ?? line.request_id ?? line.id}</span>
                  <small>{creditValue(line.amount_micros, line.currency)}</small>
                </div>
              ))}
            </div>
          </article>
        </div>

        <article className="guardrail-band account-guardrail">
          <h3>{adminCopy.accounts.sections.workflow}</h3>
          <div className="tag-list">
            <span>{adminCopy.accounts.hints.safeKey}</span>
            <span>{adminCopy.accounts.hints.session}</span>
            <span>{adminCopy.accounts.hints.audit}</span>
          </div>
          <div className="endpoint-list">
            {workflowRows.map((action) => (
              <code key={action.endpoint}>
                {action.label} · {action.endpoint}
              </code>
            ))}
          </div>
        </article>
      </div>
    </section>
  );
}
