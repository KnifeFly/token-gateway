import { formatMaybeDate, taskString } from "../../shared/format";
import { displayStatusLabel, portalCopy } from "../../shared/i18n";
import type { TaskObject } from "../../shared/types";

export function TaskListRows({ tasks }: { tasks: TaskObject[] }) {
  return (
    <div className="table" role="table">
      <div className="table-row table-head tasks" role="row">
        <span role="columnheader">{portalCopy.tasks.taskColumn}</span>
        <span role="columnheader">{portalCopy.tasks.statusColumn}</span>
        <span role="columnheader">{portalCopy.tasks.modelColumn}</span>
        <span role="columnheader">{portalCopy.tasks.createdColumn}</span>
      </div>
      {tasks.map((task, index) => (
        <div className="table-row tasks" key={taskString(task, "id") || index} role="row">
          <span role="cell">{taskString(task, "id") || portalCopy.tasks.fallbackTask}</span>
          <span role="cell">{displayStatusLabel(taskString(task, "status"))}</span>
          <span role="cell">{taskString(task, "model") || taskString(task, "endpoint")}</span>
          <span role="cell">{formatMaybeDate(taskString(task, "created_at"))}</span>
        </div>
      ))}
    </div>
  );
}
