import { CustomerAccountsPanel } from "../features/accounts/CustomerAccountsPanel";
import { ActivityLogsPanel } from "../features/activity/ActivityLogsPanel";
import { APIKeyManagementPanel } from "../features/api-keys/APIKeyManagementPanel";
import { ChannelManagementPanel } from "../features/channels/ChannelManagementPanel";
import { CreditOperationsPanel } from "../features/credits/CreditOperationsPanel";
import { ModelManagementPanel } from "../features/models/ModelManagementPanel";
import { RoutePlanPanel } from "../features/route-plan/RoutePlanPanel";
import { ToolsPlaygroundPanel } from "../features/tools/ToolsPlaygroundPanel";
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
        <ModelManagementPanel />
        <ChannelManagementPanel />
        <CustomerAccountsPanel />
        <CreditOperationsPanel />
        <APIKeyManagementPanel />
        <ActivityLogsPanel />
        <ToolsPlaygroundPanel />
      </section>
    </main>
  );
}
