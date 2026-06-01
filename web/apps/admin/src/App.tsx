import { createConsoleClient } from "@token-gateway/api-client";
import { sessionStateLabel } from "@token-gateway/auth";
import { formatISODateTime } from "@token-gateway/format";
import { Button, StatusBadge } from "@token-gateway/ui";

const client = createConsoleClient();

const navItems = [
  "Overview",
  "Tenants",
  "Models",
  "Channels",
  "Routes",
  "Pricing",
  "Operations",
  "Audit"
];

const rows = [
  { name: "BFF surface", owner: "cmd/console", status: "Reserved" },
  { name: "Machine control", owner: "cmd/control-api", status: "Unchanged" },
  { name: "Static admin UI", owner: "/admin-ui/*", status: "Scaffold" }
];

export function App() {
  const session = { authenticated: false, expiresAt: new Date().toISOString() };

  return (
    <main className="admin-shell">
      <aside className="rail" aria-label="Admin navigation">
        <div className="brand">
          <span className="brand-mark">TG</span>
          <span>Admin</span>
        </div>
        <nav>
          {navItems.map((item) => (
            <a href={`#${item.toLowerCase()}`} key={item}>
              {item}
            </a>
          ))}
        </nav>
      </aside>

      <section className="workspace" aria-label="Admin workspace">
        <header className="topbar">
          <div>
            <h1>Admin</h1>
            <p>{client.baseURL}/api/admin/v1</p>
          </div>
          <div className="topbar-actions">
            <StatusBadge tone="neutral">{sessionStateLabel(session)}</StatusBadge>
            <Button variant="secondary" disabled>
              Operator sign in
            </Button>
          </div>
        </header>

        <section className="panel">
          <div className="panel-heading">
            <div>
              <h2>Console Boundary</h2>
              <p>{formatISODateTime(session.expiresAt)}</p>
            </div>
            <StatusBadge tone="warning">P21</StatusBadge>
          </div>
          <div className="table" role="table" aria-label="Admin route ownership">
            <div className="table-row table-head" role="row">
              <span role="columnheader">Name</span>
              <span role="columnheader">Owner</span>
              <span role="columnheader">Status</span>
            </div>
            {rows.map((row) => (
              <div className="table-row" role="row" key={row.name}>
                <span role="cell">{row.name}</span>
                <span role="cell">{row.owner}</span>
                <span role="cell">{row.status}</span>
              </div>
            ))}
          </div>
        </section>
      </section>
    </main>
  );
}
