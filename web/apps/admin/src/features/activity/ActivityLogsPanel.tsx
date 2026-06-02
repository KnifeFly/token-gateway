import { formatISODateTime, formatInteger, formatMoney } from "@token-gateway/format";
import { StatusBadge } from "@token-gateway/ui";

import { adminCopy } from "../../shared/i18n";
import type { AdminTaskLogView, AdminUsageLogView } from "./activityApi";

const sampleUsageLogs: AdminUsageLogView[] = [
  {
    request_id: "req_acme_8271",
    tenant_id: "tenant_acme",
    project_id: "project_platform",
    api_key_id: "key_acme_live",
    model: "gpt-4o-mini",
    provider_type: "openai",
    channel_id: "channel_openai_primary",
    status: "settled",
    settlement_status: "settled",
    ledger_entry_id: "ledger_acme_usage",
    settlement_kind: "usage_debit",
    input_tokens: 1800,
    output_tokens: 420,
    total_tokens: 2220,
    amount_micros: 1280000,
    currency: "CNY",
    balance_after_micros: 1285000000,
    created_at: "2026-06-02T08:25:00Z",
    settled_at: "2026-06-02T08:25:02Z"
  },
  {
    request_id: "req_nova_3104",
    tenant_id: "tenant_nova",
    project_id: "project_sandbox",
    api_key_id: "key_nova_sandbox",
    model: "qwen-plus",
    provider_type: "dashscope",
    channel_id: "channel_qwen_backup",
    status: "settled",
    settlement_status: "settled",
    input_tokens: 920,
    output_tokens: 180,
    total_tokens: 1100,
    amount_micros: 540000,
    currency: "CNY",
    created_at: "2026-06-01T10:18:00Z"
  }
];

const sampleTaskLogs: AdminTaskLogView[] = [
  {
    task_id: "task_img_1001",
    request_id: "req_task_1001",
    tenant_id: "tenant_acme",
    project_id: "project_platform",
    api_key_id: "key_acme_live",
    kind: "image.generation",
    media_type: "image",
    model: "gpt-image-1",
    status: "running",
    progress: 64,
    provider_type: "openai",
    channel_id: "channel_openai_primary",
    provider_task_id: "pt_991",
    callback_configured: true,
    settlement_status: "pending",
    created_at: "2026-06-02T08:21:00Z",
    updated_at: "2026-06-02T08:23:00Z"
  },
  {
    task_id: "task_video_0902",
    request_id: "req_task_0902",
    tenant_id: "tenant_nova",
    project_id: "project_sandbox",
    api_key_id: "key_nova_sandbox",
    kind: "video.generation",
    media_type: "video",
    model: "runway-gen3",
    status: "failed",
    progress: 100,
    provider_type: "replicate",
    channel_id: "channel_video_backup",
    provider_task_id: "rep_441",
    callback_configured: false,
    settlement_status: "terminal_without_usage",
    error_code: "provider_timeout",
    created_at: "2026-06-01T14:12:00Z",
    updated_at: "2026-06-01T14:20:00Z"
  }
];

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

export function ActivityLogsPanel() {
  return (
    <section className="panel activity-panel" id="activity">
      <div className="panel-heading">
        <div>
          <h2>{adminCopy.activity.title}</h2>
          <p>{adminCopy.activity.subtitle}</p>
        </div>
        <StatusBadge tone="warning">{adminCopy.activity.status}</StatusBadge>
      </div>

      <div className="activity-layout">
        <article className="activity-section">
          <h3>{adminCopy.activity.sections.usage}</h3>
          <div className="table activity-usage-table" role="table">
            <div className="table-row table-head activity-usage-row" role="row">
              <span role="columnheader">{adminCopy.activity.columns.request}</span>
              <span role="columnheader">{adminCopy.activity.columns.scope}</span>
              <span role="columnheader">{adminCopy.activity.columns.model}</span>
              <span role="columnheader">{adminCopy.activity.columns.route}</span>
              <span role="columnheader">{adminCopy.activity.columns.tokens}</span>
              <span role="columnheader">{adminCopy.activity.columns.cost}</span>
              <span role="columnheader">{adminCopy.activity.columns.status}</span>
            </div>
            {sampleUsageLogs.map((row) => (
              <div className="table-row activity-usage-row" key={row.request_id} role="row">
                <span role="cell">
                  <strong>{row.request_id}</strong>
                  <small>{timeLabel(row.created_at)}</small>
                </span>
                <span role="cell">
                  {row.tenant_id} / {row.project_id}
                </span>
                <span role="cell">{row.model}</span>
                <span role="cell">
                  {row.provider_type} / {row.channel_id}
                </span>
                <span role="cell">{tokenLabel(row)}</span>
                <span role="cell">{amountLabel(row)}</span>
                <span role="cell">{row.settlement_status ?? row.status}</span>
              </div>
            ))}
          </div>
        </article>

        <article className="activity-section">
          <h3>{adminCopy.activity.sections.tasks}</h3>
          <div className="table activity-task-table" role="table">
            <div className="table-row table-head activity-task-row" role="row">
              <span role="columnheader">{adminCopy.activity.columns.task}</span>
              <span role="columnheader">{adminCopy.activity.columns.scope}</span>
              <span role="columnheader">{adminCopy.activity.columns.model}</span>
              <span role="columnheader">{adminCopy.activity.columns.route}</span>
              <span role="columnheader">{adminCopy.activity.columns.progress}</span>
              <span role="columnheader">{adminCopy.activity.columns.callback}</span>
              <span role="columnheader">{adminCopy.activity.columns.status}</span>
            </div>
            {sampleTaskLogs.map((row) => (
              <div className="table-row activity-task-row" key={row.task_id} role="row">
                <span role="cell">
                  <strong>{row.task_id}</strong>
                  <small>{row.request_id}</small>
                </span>
                <span role="cell">
                  {row.tenant_id} / {row.project_id}
                </span>
                <span role="cell">{row.model}</span>
                <span role="cell">
                  {row.provider_type} / {row.channel_id}
                </span>
                <span role="cell">{row.progress}%</span>
                <span role="cell">
                  {row.callback_configured
                    ? adminCopy.activity.state.configured
                    : adminCopy.activity.state.notConfigured}
                </span>
                <span role="cell">{row.status}</span>
              </div>
            ))}
          </div>
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
