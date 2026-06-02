import { formatISODateTime } from "@token-gateway/format";
import { StatusBadge } from "@token-gateway/ui";

import { scaffoldSession } from "../../shared/session/session";

const routeOwnershipRows = [
  { name: "BFF surface", owner: "cmd/console", status: "Reserved" },
  { name: "Machine control", owner: "cmd/control-api", status: "Unchanged" },
  { name: "Static admin UI", owner: "/admin-ui/*", status: "Scaffold" }
];

export function ConsoleBoundaryPanel() {
  return (
    <section className="panel">
      <div className="panel-heading">
        <div>
          <h2>Console Boundary</h2>
          <p>{formatISODateTime(scaffoldSession.expiresAt)}</p>
        </div>
        <StatusBadge tone="warning">P21</StatusBadge>
      </div>
      <div className="table" role="table" aria-label="Admin route ownership">
        <div className="table-row table-head" role="row">
          <span role="columnheader">Name</span>
          <span role="columnheader">Owner</span>
          <span role="columnheader">Status</span>
        </div>
        {routeOwnershipRows.map((row) => (
          <div className="table-row" role="row" key={row.name}>
            <span role="cell">{row.name}</span>
            <span role="cell">{row.owner}</span>
            <span role="cell">{row.status}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
