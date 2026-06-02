import type { FormEvent } from "react";

import { portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { TaskObject } from "../../shared/types";
import { TaskListRows } from "./TaskListRows";

type ActivityFilters = {
  apiKeyID: string;
  requestID: string;
  model: string;
  providerType: string;
  channelID: string;
  status: string;
  from: string;
  to: string;
};

type TasksViewProps = {
  filters: ActivityFilters;
  onApplyFilters: (event: FormEvent<HTMLFormElement>) => void;
  onFiltersChange: (filters: ActivityFilters) => void;
  tasks: TaskObject[];
};

export function TasksView({ filters, onApplyFilters, onFiltersChange, tasks }: TasksViewProps) {
  function updateFilter(key: keyof ActivityFilters, value: string) {
    onFiltersChange({ ...filters, [key]: value });
  }

  return (
    <section className="panel">
      <PanelHeading title={portalCopy.tasks.title} />
      <form className="filter-grid" onSubmit={onApplyFilters}>
        <label>
          <span>{portalCopy.tasks.requestIDLabel}</span>
          <input
            onChange={(event) => updateFilter("requestID", event.target.value)}
            value={filters.requestID}
          />
        </label>
        <label>
          <span>{portalCopy.tasks.apiKeyLabel}</span>
          <input
            onChange={(event) => updateFilter("apiKeyID", event.target.value)}
            value={filters.apiKeyID}
          />
        </label>
        <label>
          <span>{portalCopy.tasks.modelColumn}</span>
          <input
            onChange={(event) => updateFilter("model", event.target.value)}
            value={filters.model}
          />
        </label>
        <label>
          <span>{portalCopy.tasks.channelColumn}</span>
          <input
            onChange={(event) => updateFilter("channelID", event.target.value)}
            value={filters.channelID}
          />
        </label>
        <label>
          <span>{portalCopy.tasks.statusColumn}</span>
          <input
            onChange={(event) => updateFilter("status", event.target.value)}
            value={filters.status}
          />
        </label>
        <button className="tg-button tg-button--secondary" type="submit">
          {portalCopy.tasks.filterAction}
        </button>
      </form>
      <TaskListRows tasks={tasks} />
    </section>
  );
}
