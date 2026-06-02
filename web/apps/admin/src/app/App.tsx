import { ConsoleBoundaryPanel } from "../features/console-boundary/ConsoleBoundaryPanel";
import { AdminNavigation } from "../shared/layout/AdminNavigation";
import { AdminTopbar } from "../shared/layout/AdminTopbar";

export function App() {
  return (
    <main className="admin-shell">
      <AdminNavigation />

      <section className="workspace" aria-label="Admin workspace">
        <AdminTopbar />
        <ConsoleBoundaryPanel />
      </section>
    </main>
  );
}
