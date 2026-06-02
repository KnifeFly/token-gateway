import { RoutePlanPanel } from "../features/route-plan/RoutePlanPanel";
import { AdminNavigation } from "../shared/layout/AdminNavigation";
import { AdminTopbar } from "../shared/layout/AdminTopbar";
import { adminCopy } from "../shared/i18n";

export function App() {
  return (
    <main className="admin-shell">
      <AdminNavigation />

      <section className="workspace" aria-label={adminCopy.workspaceLabel}>
        <AdminTopbar />
        <RoutePlanPanel />
      </section>
    </main>
  );
}
