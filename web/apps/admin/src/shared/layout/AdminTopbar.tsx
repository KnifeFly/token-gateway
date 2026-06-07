import { Button, StatusBadge } from "@token-gateway/ui";

import { adminClient } from "../api/client";
import { adminCopy, adminSessionLabel } from "../i18n";

interface AdminTopbarProps {
  authenticated: boolean;
  email?: string;
  roles?: string[];
  onLogout: () => void;
}

export function AdminTopbar({ authenticated, email, roles = [], onLogout }: AdminTopbarProps) {
  return (
    <header className="topbar">
      <div>
        <h1>{adminCopy.topbar.title}</h1>
        <p>
          {adminCopy.topbar.subtitle} {adminClient.baseURL}/api/admin/v1
        </p>
      </div>
      <div className="topbar-actions">
        <StatusBadge tone={authenticated ? "success" : "neutral"}>
          {adminSessionLabel(authenticated)}
        </StatusBadge>
        {email ? <span className="operator-chip">{email}</span> : null}
        {roles.length ? <span className="operator-chip">{roles.join(" / ")}</span> : null}
        <Button variant="secondary" onClick={onLogout}>
          {adminCopy.topbar.signOutAction}
        </Button>
      </div>
    </header>
  );
}
