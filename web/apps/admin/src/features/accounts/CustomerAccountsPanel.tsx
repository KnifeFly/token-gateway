import { formatISODateTime, formatInteger, formatMoney } from "@token-gateway/format";
import { Button, DataTable, EmptyState, LoadingState, StatusBadge } from "@token-gateway/ui";
import { useEffect, useMemo, useState } from "react";

import { adminCopy } from "../../shared/i18n";
import {
  disableAdminCustomerAccount,
  enableAdminCustomerAccount,
  getAdminCustomerAccount,
  listAdminCustomerAccounts,
  resetAdminCustomerPortalSessions,
  type AdminCustomerAccountDetail,
  type AdminCustomerAccountView
} from "./accountApi";

interface CustomerAccountsPanelProps {
  csrfToken: string;
}

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

function primaryCredit(account?: AdminCustomerAccountView) {
  return account?.credits?.[0];
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

function describeError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function CustomerAccountsPanel({ csrfToken }: CustomerAccountsPanelProps) {
  const [accounts, setAccounts] = useState<AdminCustomerAccountView[]>([]);
  const [selectedID, setSelectedID] = useState("");
  const [detail, setDetail] = useState<AdminCustomerAccountDetail>();
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  async function loadAccounts() {
    setLoading(true);
    setMessage("");
    try {
      const response = await listAdminCustomerAccounts();
      setAccounts(response.data);
      setSelectedID((current) => current || response.data[0]?.customer_account_id || "");
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadAccounts();
  }, []);

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.customer_account_id === selectedID) ?? accounts[0],
    [accounts, selectedID]
  );

  useEffect(() => {
    if (!selectedAccount?.customer_account_id) {
      setDetail(undefined);
      return;
    }
    let active = true;
    async function loadDetail() {
      setDetailLoading(true);
      try {
        const response = await getAdminCustomerAccount(selectedAccount.customer_account_id);
        if (active) {
          setDetail(response);
        }
      } catch (error) {
        if (active) {
          setMessage(describeError(error));
          setDetail(undefined);
        }
      } finally {
        if (active) {
          setDetailLoading(false);
        }
      }
    }
    void loadDetail();
    return () => {
      active = false;
    };
  }, [selectedAccount?.customer_account_id]);

  async function mutateAccount(accountID: string, action: "enable" | "disable" | "reset-session") {
    setBusy(true);
    setMessage("");
    const mutation = {
      csrfToken,
      reason: `P25 Admin customer account ${action} ${accountID}`
    };
    try {
      if (action === "enable") {
        await enableAdminCustomerAccount(accountID, mutation);
      } else if (action === "disable") {
        await disableAdminCustomerAccount(accountID, mutation);
      } else {
        await resetAdminCustomerPortalSessions(accountID, undefined, mutation);
      }
      await loadAccounts();
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setBusy(false);
    }
  }

  const selected = detail;
  const selectedCredit = primaryCredit(selected?.account ?? selectedAccount);

  return (
    <section className="panel account-panel" id="accounts">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.accounts.title}</h2>
          <p>{adminCopy.accounts.subtitle}</p>
        </div>
        <StatusBadge tone="success">BFF connected</StatusBadge>
      </div>

      {message ? <div className="inline-alert">{message}</div> : null}
      {loading ? <LoadingState label="正在加载客户账户" /> : null}

      <div className="account-layout">
        <article className="account-section">
          <h3>{adminCopy.accounts.sections.list}</h3>
          <DataTable
            ariaLabel={adminCopy.accounts.sections.list}
            className="account-table"
            columns={[
              {
                key: "account",
                header: adminCopy.accounts.columns.account,
                render: (account) => (
                  <>
                    <strong>{account.display_name ?? account.project_name}</strong>
                    <small>
                      {account.tenant_name} / {account.project_name}
                    </small>
                  </>
                )
              },
              { key: "status", header: adminCopy.accounts.columns.status, render: (account) => accountStatusLabel(account.status) },
              { key: "role", header: adminCopy.accounts.columns.role, render: (account) => accountRoleLabel(account.role) },
              {
                key: "credits",
                header: adminCopy.accounts.columns.credits,
                render: (account) => {
                  const credit = primaryCredit(account);
                  return creditValue(credit?.available_micros, credit?.currency);
                }
              },
              {
                key: "keys",
                header: adminCopy.accounts.columns.keys,
                render: (account) => `${account.active_api_key_count}/${account.api_key_count}`
              },
              { key: "models", header: adminCopy.accounts.columns.models, render: modelScope },
              {
                key: "usage",
                header: adminCopy.accounts.columns.usage,
                render: (account) => formatInteger(account.recent_usage.requests)
              },
              {
                key: "actions",
                header: "操作",
                render: (account) => (
                  <span className="inline-actions">
                    <Button onClick={() => setSelectedID(account.customer_account_id)} variant="ghost">
                      详情
                    </Button>
                    <Button
                      disabled={busy || !csrfToken || account.status === "active"}
                      onClick={() => mutateAccount(account.customer_account_id, "enable")}
                      variant="ghost"
                    >
                      {adminCopy.accounts.actions.enable}
                    </Button>
                    <Button
                      disabled={busy || !csrfToken || account.status !== "active"}
                      onClick={() => mutateAccount(account.customer_account_id, "disable")}
                      variant="ghost"
                    >
                      {adminCopy.accounts.actions.disable}
                    </Button>
                  </span>
                )
              }
            ]}
            empty={<EmptyState title="暂无客户账户">当前 BFF 没有返回客户账户。</EmptyState>}
            getRowKey={(account) => account.customer_account_id}
            rowClassName="table-row account-row"
            rows={accounts}
          />
        </article>

        {detailLoading ? <LoadingState label="正在加载账户详情" /> : null}

        <div className="account-detail-grid">
          <article className="account-section">
            <h3>{adminCopy.accounts.sections.overview}</h3>
            {selected?.account ? (
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
                  <dd>{selected.account.email ?? "-"}</dd>
                </div>
                <div>
                  <dt>{adminCopy.accounts.columns.notes}</dt>
                  <dd>{selected.account.notes ?? "-"}</dd>
                </div>
              </dl>
            ) : (
              <EmptyState title="暂无详情">选择一个客户账户查看详情。</EmptyState>
            )}
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
              <strong>{adminCopy.accounts.actions.resetSession}</strong>
              <span>{adminCopy.accounts.hints.session}</span>
              <Button
                disabled={busy || !csrfToken || !selected?.account.customer_account_id}
                onClick={() =>
                  selected?.account.customer_account_id
                    ? mutateAccount(selected.account.customer_account_id, "reset-session")
                    : undefined
                }
                variant="ghost"
              >
                {adminCopy.accounts.actions.resetSession}
              </Button>
            </div>
          </article>
        </div>

        <article className="account-section">
          <h3>{adminCopy.accounts.sections.keys}</h3>
          <div className="coverage-grid">
            {(selected?.api_keys ?? []).map((key) => (
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
            {(selected?.api_keys ?? []).length === 0 ? (
              <EmptyState title="暂无 API key">该账户没有返回 API key summary。</EmptyState>
            ) : null}
          </div>
        </article>

        <div className="account-detail-grid">
          <article className="account-section">
            <h3>{adminCopy.accounts.sections.usage}</h3>
            <div className="account-metric-list">
              {(selected?.usage ?? []).map((row) => (
                <div key={`${row.model}-${row.channel_id}`}>
                  <strong>{row.model ?? "-"}</strong>
                  <span>
                    {row.channel_id ?? "-"} · {formatInteger(row.requests)} {adminCopy.accounts.columns.requests}
                  </span>
                  <small>{creditValue(row.amount_micros, row.currency)}</small>
                </div>
              ))}
              {(selected?.usage ?? []).length === 0 ? (
                <EmptyState title="暂无用量">该账户没有返回近期用量。</EmptyState>
              ) : null}
            </div>
          </article>

          <article className="account-section">
            <h3>{adminCopy.accounts.sections.ledger}</h3>
            <div className="account-metric-list">
              {(selected?.ledger ?? []).map((line) => (
                <div key={line.id}>
                  <strong>{line.settlement_kind}</strong>
                  <span>{line.reason ?? line.request_id ?? line.id}</span>
                  <small>{creditValue(line.amount_micros, line.currency)}</small>
                </div>
              ))}
              {(selected?.ledger ?? []).length === 0 ? (
                <EmptyState title="暂无账本">该账户没有返回账本明细。</EmptyState>
              ) : null}
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
