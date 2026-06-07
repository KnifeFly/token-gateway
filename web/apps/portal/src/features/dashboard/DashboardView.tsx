import { formatInteger } from "@token-gateway/format";
import { StatusBadge } from "@token-gateway/ui";

import { TaskListRows } from "../tasks/TaskListRows";
import { portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { Dashboard, TaskObject, UsageResponse } from "../../shared/types";

export function DashboardView({
  dashboard,
  usage,
  tasks
}: {
  dashboard?: Dashboard;
  usage?: UsageResponse;
  tasks: TaskObject[];
}) {
  return (
    <section className="panel-grid">
      <article className="panel">
        <PanelHeading title={portalCopy.dashboard.usageTitle} meta={dashboard?.generated_at} />
        <div className="stat-row">
          <span>{portalCopy.dashboard.requests}</span>
          <strong>{formatInteger(usage?.totals.requests ?? 0)}</strong>
        </div>
        <div className="stat-row">
          <span>{portalCopy.dashboard.inputTokens}</span>
          <strong>{formatInteger(usage?.totals.input_tokens ?? 0)}</strong>
        </div>
        <div className="stat-row">
          <span>{portalCopy.dashboard.outputTokens}</span>
          <strong>{formatInteger(usage?.totals.output_tokens ?? 0)}</strong>
        </div>
      </article>
      <article className="panel">
        <PanelHeading title={portalCopy.dashboard.tasksTitle} />
        <div className="task-summary">
          <StatusBadge tone="neutral">
            {portalCopy.dashboard.queued(formatInteger(dashboard?.task_summary.queued ?? 0))}
          </StatusBadge>
          <StatusBadge tone="warning">
            {portalCopy.dashboard.processing(
              formatInteger(dashboard?.task_summary.processing ?? 0)
            )}
          </StatusBadge>
          <StatusBadge tone="success">
            {portalCopy.dashboard.completed(formatInteger(dashboard?.task_summary.completed ?? 0))}
          </StatusBadge>
        </div>
        <TaskListRows tasks={tasks.slice(0, 4)} />
      </article>
    </section>
  );
}
