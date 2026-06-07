import { formatISODateTime, formatInteger, formatMoney } from "@token-gateway/format";
import { CopyButton, DataTable, EmptyState, FilterBar, LoadingState, StatusBadge } from "@token-gateway/ui";
import { useEffect, useMemo, useState } from "react";

import { adminCopy } from "../../shared/i18n";
import {
  listAdminTaskLogs,
  listAdminUsageLogs,
  type AdminTaskLogView,
  type AdminUsageLogView
} from "./activityApi";

const usageFilterRows = [
  "request_id",
  "tenant_id / project_id",
  "api_key_id",
  "model",
  "channel_id",
  "provider_type",
  "status",
  "from / to"
];

const taskFilterRows = [
  "task_id",
  "request_id",
  "tenant_id / project_id",
  "api_key_id",
  "model",
  "channel_id",
  "provider_type",
  "status"
];

function amountLabel(row: AdminUsageLogView): string {
  return `${row.currency} ${formatMoney(row.amount_micros / 1_000_000, 4)}`;
}

function tokenLabel(row: AdminUsageLogView): string {
  return `${formatInteger(row.total_tokens)} Token`;
}

function timeLabel(value?: string): string {
  return value ? formatISODateTime(value) : adminCopy.activity.state.unknown;
}

function describeError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function matchesFilter(values: Array<string | undefined>, query: string): boolean {
  const normalized = query.trim().toLowerCase();
  if (!normalized) {
    return true;
  }
  return values.some((value) => (value ?? "").toLowerCase().includes(normalized));
}

export function ActivityLogsPanel() {
  const [usageLogs, setUsageLogs] = useState<AdminUsageLogView[]>([]);
  const [taskLogs, setTaskLogs] = useState<AdminTaskLogView[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");

  async function loadActivity() {
    setLoading(true);
    setMessage("");
    try {
      const [usageResponse, taskResponse] = await Promise.all([
        listAdminUsageLogs(),
        listAdminTaskLogs()
      ]);
      setUsageLogs(usageResponse.data);
      setTaskLogs(taskResponse.data);
    } catch (error) {
      setMessage(describeError(error));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadActivity();
  }, []);

  const filteredUsage = useMemo(
    () =>
      usageLogs.filter((row) =>
        matchesFilter(
          [
            row.request_id,
            row.tenant_id,
            row.project_id,
            row.api_key_id,
            row.model,
            row.provider_type,
            row.channel_id,
            row.status,
            row.settlement_status
          ],
          query
        )
      ),
    [query, usageLogs]
  );

  const filteredTasks = useMemo(
    () =>
      taskLogs.filter((row) =>
        matchesFilter(
          [
            row.task_id,
            row.request_id,
            row.tenant_id,
            row.project_id,
            row.api_key_id,
            row.model,
            row.provider_type,
            row.channel_id,
            row.status,
            row.settlement_status
          ],
          query
        )
      ),
    [query, taskLogs]
  );

  return (
    <section className="panel activity-panel" id="activity">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.activity.title}</h2>
          <p>{adminCopy.activity.subtitle}</p>
        </div>
        <StatusBadge tone="success">BFF connected</StatusBadge>
      </div>

      <FilterBar>
        <label>
          搜索
          <input
            placeholder="request_id / task_id / model / channel / status"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </label>
      </FilterBar>

      {message ? <div className="inline-alert">{message}</div> : null}
      {loading ? <LoadingState label="正在加载活动日志" /> : null}

      <div className="activity-layout">
        <article className="activity-section">
          <h3>{adminCopy.activity.sections.usage}</h3>
          <DataTable
            ariaLabel={adminCopy.activity.sections.usage}
            className="activity-usage-table"
            columns={[
              {
                key: "request",
                header: adminCopy.activity.columns.request,
                render: (row) => (
                  <>
                    <strong>{row.request_id}</strong>
                    <small>{timeLabel(row.created_at)}</small>
                    <CopyButton label="复制 ID" value={row.request_id} />
                  </>
                )
              },
              {
                key: "scope",
                header: adminCopy.activity.columns.scope,
                render: (row) => `${row.tenant_id} / ${row.project_id}`
              },
              { key: "model", header: adminCopy.activity.columns.model, render: (row) => row.model },
              {
                key: "route",
                header: adminCopy.activity.columns.route,
                render: (row) => `${row.provider_type} / ${row.channel_id}`
              },
              { key: "tokens", header: adminCopy.activity.columns.tokens, render: tokenLabel },
              { key: "cost", header: adminCopy.activity.columns.cost, render: amountLabel },
              {
                key: "status",
                header: adminCopy.activity.columns.status,
                render: (row) => row.settlement_status ?? row.status
              }
            ]}
            empty={<EmptyState title="暂无用量日志">当前筛选条件没有 BFF 用量记录。</EmptyState>}
            getRowKey={(row) => row.request_id}
            rowClassName="table-row activity-usage-row"
            rows={filteredUsage}
          />
        </article>

        <article className="activity-section">
          <h3>{adminCopy.activity.sections.tasks}</h3>
          <DataTable
            ariaLabel={adminCopy.activity.sections.tasks}
            className="activity-task-table"
            columns={[
              {
                key: "task",
                header: adminCopy.activity.columns.task,
                render: (row) => (
                  <>
                    <strong>{row.task_id}</strong>
                    <small>{row.request_id}</small>
                    <CopyButton label="复制 ID" value={row.task_id} />
                  </>
                )
              },
              {
                key: "scope",
                header: adminCopy.activity.columns.scope,
                render: (row) => `${row.tenant_id} / ${row.project_id}`
              },
              { key: "model", header: adminCopy.activity.columns.model, render: (row) => row.model },
              {
                key: "route",
                header: adminCopy.activity.columns.route,
                render: (row) => `${row.provider_type ?? "-"} / ${row.channel_id ?? "-"}`
              },
              { key: "progress", header: adminCopy.activity.columns.progress, render: (row) => `${row.progress}%` },
              {
                key: "callback",
                header: adminCopy.activity.columns.callback,
                render: (row) =>
                  row.callback_configured
                    ? adminCopy.activity.state.configured
                    : adminCopy.activity.state.notConfigured
              },
              { key: "status", header: adminCopy.activity.columns.status, render: (row) => row.status }
            ]}
            empty={<EmptyState title="暂无任务日志">当前筛选条件没有 BFF 任务记录。</EmptyState>}
            getRowKey={(row) => row.task_id}
            rowClassName="table-row activity-task-row"
            rows={filteredTasks}
          />
        </article>

        <div className="activity-detail-grid">
          <article className="activity-section">
            <h3>{adminCopy.activity.sections.filters}</h3>
            <div className="activity-filter-list">
              <div>
                <strong>{adminCopy.activity.sections.usage}</strong>
                <span>{usageFilterRows.join(" / ")}</span>
              </div>
              <div>
                <strong>{adminCopy.activity.sections.tasks}</strong>
                <span>{taskFilterRows.join(" / ")}</span>
              </div>
            </div>
          </article>

          <article className="activity-section">
            <h3>{adminCopy.activity.sections.safeDetail}</h3>
            <div className="tag-list">
              <span>{adminCopy.activity.hints.noRawPayload}</span>
              <span>{adminCopy.activity.hints.safeMetadata}</span>
              <span>{adminCopy.activity.hints.exportReady}</span>
            </div>
          </article>
        </div>
      </div>
    </section>
  );
}
