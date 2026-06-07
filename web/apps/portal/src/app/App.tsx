import { Button, StatusBadge } from "@token-gateway/ui";

import { APIKeysView } from "../features/api-keys/APIKeysView";
import { LoginView } from "../features/auth/LoginView";
import { CreditsView } from "../features/credits/CreditsView";
import { DashboardView } from "../features/dashboard/DashboardView";
import { ModelsView } from "../features/models/ModelsView";
import { PlaygroundView } from "../features/playground/PlaygroundView";
import { SettingsView } from "../features/settings/SettingsView";
import { TasksView } from "../features/tasks/TasksView";
import { UsageView } from "../features/usage/UsageView";
import { portalClient } from "../shared/api/client";
import { portalCopy } from "../shared/i18n";
import { navItems } from "./routes";
import { usePortalConsoleData } from "./usePortalConsoleData";
import { usePortalRoute } from "./usePortalRoute";

export function App() {
  const { activeView, navigateView } = usePortalRoute();
  const portal = usePortalConsoleData();
  const activeRoute = navItems.find((item) => item.id === activeView);

  if (!portal.session.authenticated) {
    return (
      <LoginView
        apiKey={portal.apiKey}
        busy={portal.busy}
        message={portal.message}
        onAPIKeyChange={portal.setAPIKey}
        onSubmit={portal.handleLogin}
      />
    );
  }

  return (
    <main className="app-shell">
      <aside className="sidebar" aria-label={portalCopy.navigationLabel}>
        <div className="brand">
          <span className="brand-mark">TG</span>
          <span>{portalCopy.brand}</span>
        </div>
        <nav>
          {navItems.map((item) => (
            <button
              className={activeView === item.id ? "active" : ""}
              key={item.id}
              onClick={() => navigateView(item.id)}
              type="button"
            >
              {item.label}
            </button>
          ))}
        </nav>
      </aside>

      <section className="workspace" aria-label={portalCopy.workspaceLabel}>
        <header className="topbar">
          <div>
            <h1>{activeRoute?.label ?? portalCopy.topbar.title}</h1>
            <p>
              {activeRoute?.purpose} {portalClient.baseURL}/api/portal/v1
            </p>
          </div>
          <div className="topbar-actions">
            <StatusBadge tone={portal.busy ? "warning" : "success"}>
              {portal.busy ? portalCopy.topbar.syncing : portalCopy.topbar.signedIn}
            </StatusBadge>
            <Button disabled={portal.busy} onClick={portal.handleLogout} variant="secondary">
              {portalCopy.topbar.logout}
            </Button>
          </div>
        </header>

        {portal.message ? <div className="error-banner">{portal.message}</div> : null}

        <section className="summary-grid" aria-label={portalCopy.summaryLabel}>
          {portal.summary.map((item) => (
            <article className="metric-card" key={item.label}>
              <span>{item.label}</span>
              <strong>{item.value}</strong>
              <small>{item.detail}</small>
            </article>
          ))}
        </section>

        {activeView === "dashboard" ? (
          <DashboardView dashboard={portal.dashboard} usage={portal.usage} tasks={portal.tasks?.data ?? []} />
        ) : null}
        {activeView === "models" ? (
          <ModelsView
            modelDetail={portal.modelDetail}
            modelSchema={portal.modelSchema}
            models={portal.models}
            onSelectModel={portal.handleSelectModel}
            selectedModel={portal.selectedModel}
          />
        ) : null}
        {activeView === "playground" ? (
          <PlaygroundView
            busy={portal.busy}
            modelSchema={portal.modelSchema}
            models={portal.models}
            onRun={portal.handleRunPlayground}
            onSelectModel={portal.handleSelectModel}
            result={portal.playgroundResult}
            selectedModel={portal.selectedModel}
          />
        ) : null}
        {activeView === "api-keys" ? (
          <APIKeysView
            apiKeys={portal.apiKeys}
            createdKey={portal.createdKey}
            currentKeyID={portal.session.api_key_id ?? ""}
            derivedExpiresAt={portal.derivedExpiresAt}
            derivedIPAllowlist={portal.derivedIPAllowlist}
            derivedModels={portal.derivedModels}
            derivedName={portal.derivedName}
            onCreateKey={portal.handleCreateKey}
            onDisableKey={portal.handleDisableKey}
            onRotateKey={portal.handleRotateKey}
            setDerivedExpiresAt={portal.setDerivedExpiresAt}
            setDerivedIPAllowlist={portal.setDerivedIPAllowlist}
            setDerivedModels={portal.setDerivedModels}
            setDerivedName={portal.setDerivedName}
          />
        ) : null}
        {activeView === "usage" ? (
          <UsageView
            filters={portal.usageFilters}
            limit={portal.usageLimit}
            onApplyFilters={portal.handleApplyUsageFilters}
            onFiltersChange={portal.setUsageFilters}
            onLimitChange={portal.handleUsageLimitChange}
            usage={portal.usage}
          />
        ) : null}
        {activeView === "tasks" ? (
          <TasksView
            filters={portal.taskFilters}
            limit={portal.taskLimit}
            onApplyFilters={portal.handleApplyTaskFilters}
            onFiltersChange={portal.setTaskFilters}
            onLimitChange={portal.handleTaskLimitChange}
            tasks={portal.tasks?.data ?? []}
          />
        ) : null}
        {activeView === "credits" ? (
          <CreditsView
            credits={portal.credits}
            ledger={portal.creditLedger}
            ledgerLimit={portal.ledgerLimit}
            onLedgerLimitChange={portal.handleLedgerLimitChange}
            usageExport={portal.usageExport}
          />
        ) : null}
        {activeView === "settings" ? (
          <SettingsView settings={portal.settings} session={portal.session} />
        ) : null}
      </section>
    </main>
  );
}
