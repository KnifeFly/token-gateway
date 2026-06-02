import { Button, StatusBadge } from "@token-gateway/ui";

import { adminClient } from "../api/client";
import { adminCopy, adminSessionLabel } from "../i18n";
import { scaffoldSession } from "../session/session";

export function AdminTopbar() {
  return (
    <header className="topbar">
      <div>
        <h1>{adminCopy.topbar.title}</h1>
        <p>
          {adminCopy.topbar.subtitle} {adminClient.baseURL}/api/admin/v1
        </p>
      </div>
      <div className="topbar-actions">
        <StatusBadge tone="neutral">{adminSessionLabel(scaffoldSession.authenticated)}</StatusBadge>
        <Button variant="secondary" disabled>
          {adminCopy.topbar.signInAction}
        </Button>
      </div>
    </header>
  );
}
