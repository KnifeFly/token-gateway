import { formatISODateTime, formatInteger } from "@token-gateway/format";
import { StatusBadge } from "@token-gateway/ui";

import type { AdminListResponse, ProductionData } from "./productionApi";

interface ProductionOverviewPanelProps {
  data?: ProductionData;
  busy: boolean;
  onRefresh: () => void;
}

const dashboardCounters = [
  ["tenants", "租户"],
  ["projects", "项目"],
  ["api_keys", "API 密钥"],
  ["models", "模型"],
  ["channels", "渠道"],
  ["routes", "路由"],
  ["pricing_rules", "价格规则"],
  ["limit_rules", "限流规则"],
  ["failed_settlements", "待修复结算"],
  ["due_callbacks", "待投递回调"]
] as const;

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function displayValue(value: unknown): string {
  if (value === null || value === undefined || value === "") {
    return "-";
  }
  if (typeof value === "boolean") {
    return value ? "是" : "否";
  }
  if (Array.isArray(value)) {
    return value.length === 0 ? "-" : value.map(displayValue).join(" / ");
  }
  if (typeof value === "object") {
    return JSON.stringify(value);
  }
  return String(value);
}

function preferredFields(row: Record<string, unknown>): [string, unknown][] {
  const fieldOrder = [
    "id",
    "tenant_id",
    "project_id",
    "name",
    "public_model",
    "channel_id",
    "route_id",
    "status",
    "enabled",
    "operator_id",
    "email",
    "action",
    "resource",
    "created_at",
    "updated_at"
  ];
  const pairs = fieldOrder
    .filter((field) => field in row)
    .map((field) => [field, row[field]] as [string, unknown]);
  return pairs.length > 0 ? pairs.slice(0, 4) : Object.entries(row).slice(0, 4);
}

function CompactList({
  title,
  response,
  emptyText
}: {
  title: string;
  response?: AdminListResponse;
  emptyText: string;
}) {
  const rows = response?.data ?? [];
  return (
    <article className="production-list">
      <div className="production-list-title">
        <h3>{title}</h3>
        <StatusBadge tone={rows.length > 0 ? "success" : "neutral"}>{formatInteger(rows.length)}</StatusBadge>
      </div>
      {rows.length === 0 ? (
        <p>{emptyText}</p>
      ) : (
        <div className="production-rows">
          {rows.slice(0, 4).map((row, index) => {
            const record = asRecord(row);
            return (
              <div className="production-row-card" key={`${title}-${index}`}>
                {preferredFields(record).map(([field, value]) => (
                  <span key={field}>
                    <strong>{field}</strong>
                    {displayValue(value)}
                  </span>
                ))}
              </div>
            );
          })}
        </div>
      )}
    </article>
  );
}

export function ProductionOverviewPanel({ data, busy, onRefresh }: ProductionOverviewPanelProps) {
  const counts = data?.dashboard?.counts ?? {};
  const activeSnapshot = asRecord(data?.snapshots?.active);
  const previousSnapshot = asRecord(data?.snapshots?.previous);

  return (
    <section className="panel production-panel" id="workbench">
      <div className="panel-heading">
        <div>
          <h2>Console 生产工作台</h2>
          <p>汇总 P22 要求的 Admin 页面、只读配置、运维队列、审计和操作员状态。</p>
        </div>
        <button className="tg-button tg-button--secondary" disabled={busy} onClick={onRefresh}>
          {busy ? "刷新中" : "刷新"}
        </button>
      </div>

      <div className="metric-grid">
        {dashboardCounters.map(([key, label]) => (
          <div className="metric-card" key={key}>
            <span>{label}</span>
            <strong>{formatInteger(Number(counts[key] ?? 0))}</strong>
          </div>
        ))}
      </div>

      <div className="production-section-grid">
        <CompactList title="Tenants" response={data?.tenants} emptyText="暂无租户 read model。" />
        <CompactList title="Projects" response={data?.projects} emptyText="暂无项目 read model。" />
        <CompactList title="Routes" response={data?.routes} emptyText="暂无路由配置。" />
        <CompactList title="Pricing" response={data?.pricing} emptyText="暂无价格规则。" />
        <CompactList title="Limits" response={data?.limits} emptyText="暂无限流规则。" />
        <CompactList title="Operators" response={data?.operators} emptyText="暂无操作员列表。" />
        <CompactList title="Audit" response={data?.audit} emptyText="暂无审计事件。" />
        <CompactList title="Workers" response={data?.workers} emptyText="暂无 worker 状态。" />
      </div>

      <div className="snapshot-summary">
        <article>
          <h3>Active Snapshot</h3>
          <p>{displayValue(activeSnapshot.version ?? activeSnapshot.Version ?? "not published")}</p>
          <small>{displayValue(activeSnapshot.checksum ?? activeSnapshot.Checksum)}</small>
        </article>
        <article>
          <h3>Rollback Snapshot</h3>
          <p>{displayValue(previousSnapshot.version ?? previousSnapshot.Version ?? "not available")}</p>
          <small>
            {data?.loadedAt ? `loaded ${formatISODateTime(data.loadedAt)}` : "not loaded"}
          </small>
        </article>
      </div>

      {data?.errors.length ? (
        <div className="error-list" role="status">
          <strong>部分 read model 暂不可用</strong>
          {data.errors.slice(0, 5).map((error) => (
            <span key={error}>{error}</span>
          ))}
        </div>
      ) : null}
    </section>
  );
}
