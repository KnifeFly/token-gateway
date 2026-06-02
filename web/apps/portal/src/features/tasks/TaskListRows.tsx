import { formatMaybeDate, taskString } from "../../shared/format";
import { displayStatusLabel, portalCopy } from "../../shared/i18n";
import type { TaskObject } from "../../shared/types";

export function TaskListRows({ tasks }: { tasks: TaskObject[] }) {
  return (
    <div className="table" role="table">
      <div className="table-row table-head tasks" role="row">
        <span role="columnheader">{portalCopy.tasks.taskColumn}</span>
        <span role="columnheader">{portalCopy.tasks.requestColumn}</span>
        <span role="columnheader">{portalCopy.tasks.statusColumn}</span>
        <span role="columnheader">{portalCopy.tasks.modelColumn}</span>
        <span role="columnheader">{portalCopy.tasks.channelColumn}</span>
        <span role="columnheader">{portalCopy.tasks.createdColumn}</span>
      </div>
      {tasks.length === 0 ? (
        <div className="table-row table-empty tasks" role="row">
          <span role="cell">{portalCopy.tasks.empty}</span>
        </div>
      ) : null}
      {tasks.map((task, index) => (
        <div className="table-row tasks" key={taskString(task, "id") || index} role="row">
          <span role="cell">{taskString(task, "id") || portalCopy.tasks.fallbackTask}</span>
          <span role="cell">{taskString(task, "request_id") || portalCopy.tasks.fallbackTask}</span>
          <span role="cell">{displayStatusLabel(taskString(task, "status"))}</span>
          <span role="cell">{taskString(task, "model")}</span>
          <span role="cell">
            {taskString(task, "provider_type") || taskString(task, "channel_id")
              ? `${taskString(task, "provider_type")} / ${taskString(task, "channel_id")}`
              : portalCopy.tasks.fallbackStatus}
          </span>
          <span role="cell">{formatMaybeDate(taskString(task, "created_at"))}</span>
        </div>
      ))}
    </div>
  );
}
