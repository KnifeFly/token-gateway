import { sessionStateLabel } from "@token-gateway/auth";
import { Button, StatusBadge } from "@token-gateway/ui";

import { adminClient } from "../api/client";
import { scaffoldSession } from "../session/session";

export function AdminTopbar() {
  return (
    <header className="topbar">
      <div>
        <h1>Admin</h1>
        <p>{adminClient.baseURL}/api/admin/v1</p>
      </div>
      <div className="topbar-actions">
        <StatusBadge tone="neutral">{sessionStateLabel(scaffoldSession)}</StatusBadge>
        <Button variant="secondary" disabled>
          Operator sign in
        </Button>
      </div>
    </header>
  );
}
