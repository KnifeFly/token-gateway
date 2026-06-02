import { formatMaybeDate, taskString } from "../../shared/format";
import type { TaskObject } from "../../shared/types";

export function TaskListRows({ tasks }: { tasks: TaskObject[] }) {
  return (
    <div className="table" role="table">
      <div className="table-row table-head tasks" role="row">
        <span role="columnheader">Task</span>
        <span role="columnheader">Status</span>
        <span role="columnheader">Model</span>
        <span role="columnheader">Created</span>
      </div>
      {tasks.map((task, index) => (
        <div className="table-row tasks" key={taskString(task, "id") || index} role="row">
          <span role="cell">{taskString(task, "id") || "task"}</span>
          <span role="cell">{taskString(task, "status") || "unknown"}</span>
          <span role="cell">{taskString(task, "model") || taskString(task, "endpoint")}</span>
          <span role="cell">{formatMaybeDate(taskString(task, "created_at"))}</span>
        </div>
      ))}
    </div>
  );
}
