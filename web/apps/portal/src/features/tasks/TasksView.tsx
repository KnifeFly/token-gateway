import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { TaskObject } from "../../shared/types";
import { TaskListRows } from "./TaskListRows";

export function TasksView({ tasks }: { tasks: TaskObject[] }) {
  return (
    <section className="panel">
      <PanelHeading title="Tasks" />
      <TaskListRows tasks={tasks} />
    </section>
  );
}
