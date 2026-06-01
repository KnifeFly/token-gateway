import { createConsoleClient } from "@token-gateway/api-client";
import { sessionStateLabel } from "@token-gateway/auth";
import { formatISODateTime, formatInteger } from "@token-gateway/format";
import { Button, StatusBadge } from "@token-gateway/ui";

const client = createConsoleClient();

const navItems = ["Dashboard", "Models", "Credits", "Usage", "API Keys", "Tasks"];
const summary = [
  { label: "Models", value: formatInteger(0), tone: "neutral" as const },
  { label: "Credits", value: "Pending", tone: "warning" as const },
  { label: "Usage", value: "Pending", tone: "warning" as const }
];

export function App() {
  const session = { authenticated: false, expiresAt: new Date().toISOString() };

  return (
    <main className="app-shell">
      <aside className="sidebar" aria-label="Portal navigation">
        <div className="brand">
          <span className="brand-mark">TG</span>
          <span>Portal</span>
        </div>
        <nav>
          {navItems.map((item) => (
            <a href={`#${item.toLowerCase().replaceAll(" ", "-")}`} key={item}>
              {item}
            </a>
          ))}
        </nav>
      </aside>

      <section className="workspace" aria-label="Portal workspace">
        <header className="topbar">
          <div>
            <h1>Portal</h1>
            <p>{client.baseURL}/api/portal/v1</p>
          </div>
          <StatusBadge tone="neutral">{sessionStateLabel(session)}</StatusBadge>
        </header>

        <div className="summary-grid">
          {summary.map((item) => (
            <article className="metric-card" key={item.label}>
              <span>{item.label}</span>
              <strong>{item.value}</strong>
              <StatusBadge tone={item.tone}>P20</StatusBadge>
            </article>
          ))}
        </div>

        <section className="panel">
          <div>
            <h2>Console Foundation</h2>
            <p>Generated API types and shared UI packages are wired for Portal Web.</p>
          </div>
          <div className="panel-actions">
            <span>{formatISODateTime(session.expiresAt)}</span>
            <Button variant="primary" disabled>
              Sign in
            </Button>
          </div>
        </section>
      </section>
    </main>
  );
}
