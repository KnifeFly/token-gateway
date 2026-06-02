import { portalCopy } from "../../shared/i18n";
import { PanelHeading } from "../../shared/layout/PanelHeading";
import type { TaskObject } from "../../shared/types";
import { TaskListRows } from "./TaskListRows";

export function TasksView({ tasks }: { tasks: TaskObject[] }) {
  return (
    <section className="panel">
      <PanelHeading title={portalCopy.tasks.title} />
      <TaskListRows tasks={tasks} />
    </section>
  );
}
