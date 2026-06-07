import { ConsoleAPIError } from "@token-gateway/api-client";
import { useEffect, useMemo, useState } from "react";

import { CustomerAccountsPanel } from "../features/accounts/CustomerAccountsPanel";
import { ActivityLogsPanel } from "../features/activity/ActivityLogsPanel";
import { APIKeyManagementPanel } from "../features/api-keys/APIKeyManagementPanel";
import { AdminLoginView } from "../features/auth/AdminLoginView";
import { ChannelManagementPanel } from "../features/channels/ChannelManagementPanel";
import { CreditOperationsPanel } from "../features/credits/CreditOperationsPanel";
import { ModelManagementPanel } from "../features/models/ModelManagementPanel";
import { DangerOperationsPanel } from "../features/production/DangerOperationsPanel";
import { ProductionOverviewPanel } from "../features/production/ProductionOverviewPanel";
import {
  adminLogin,
  adminLogout,
  getAdminSession,
  loadProductionData,
  type AdminSession,
  type ProductionData
} from "../features/production/productionApi";
import { RoutePlanPanel } from "../features/route-plan/RoutePlanPanel";
import { ToolsPlaygroundPanel } from "../features/tools/ToolsPlaygroundPanel";
import { AdminNavigation } from "../shared/layout/AdminNavigation";
import { AdminTopbar } from "../shared/layout/AdminTopbar";
import { adminRouteSections, type AdminRouteID } from "./routes";

function routeFromLocation(): AdminRouteID {
  const path = window.location.pathname;
  return adminRouteSections.find((section) => path.startsWith(section.routePrefix))?.id ?? "workbench";
}

export function App() {
  const [activeRouteID, setActiveRouteID] = useState<AdminRouteID>(() => routeFromLocation());
  const [session, setSession] = useState<AdminSession>({ authenticated: false });
  const [csrfToken, setCSRFToken] = useState("");
  const [productionData, setProductionData] = useState<ProductionData>();
  const [busy, setBusy] = useState(true);
  const [message, setMessage] = useState("");

  const routeTitle = useMemo(
    () => adminRouteSections.find((section) => section.id === activeRouteID)?.label ?? "工作台",
    [activeRouteID]
  );

  async function loadData() {
    setBusy(true);
    try {
      const nextData = await loadProductionData();
      setProductionData(nextData);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    let active = true;
    async function restore() {
      try {
        const current = await getAdminSession();
        if (!active) {
          return;
        }
        setSession(current);
        setCSRFToken(current.csrf_token ?? "");
        await loadData();
      } catch (error) {
        if (!active) {
          return;
        }
        if (!(error instanceof ConsoleAPIError) || error.status !== 401) {
          setMessage(error instanceof Error ? error.message : String(error));
        }
        setSession({ authenticated: false });
        setBusy(false);
      }
    }

    void restore();
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    function syncRouteFromHistory() {
      setActiveRouteID(routeFromLocation());
    }
    window.addEventListener("popstate", syncRouteFromHistory);
    return () => {
      window.removeEventListener("popstate", syncRouteFromHistory);
    };
  }, []);

  function navigate(routeID: string) {
    const nextRoute =
      adminRouteSections.find((section) => section.id === routeID)?.id ?? ("workbench" as AdminRouteID);
    const nextPath =
      adminRouteSections.find((section) => section.id === nextRoute)?.routePrefix ?? "/admin-ui/workbench";
    setActiveRouteID(nextRoute);
    window.history.pushState({}, "", nextPath);
  }

  async function handleLogin(email: string, password: string) {
    setBusy(true);
    setMessage("");
    try {
      const response = await adminLogin(email, password);
      setSession(response.session);
      setCSRFToken(response.csrf_token || response.session.csrf_token || "");
      await loadData();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
      setSession({ authenticated: false });
      setBusy(false);
    }
  }

  async function handleLogout() {
    setBusy(true);
    try {
      await adminLogout(csrfToken);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setSession({ authenticated: false });
      setCSRFToken("");
      setProductionData(undefined);
      setBusy(false);
    }
  }

  if (!session.authenticated) {
    return <AdminLoginView busy={busy} message={message} onLogin={handleLogin} />;
  }

  return (
    <main className="admin-shell">
      <AdminNavigation activeRouteID={activeRouteID} onNavigate={navigate} />

      <section className="workspace" aria-label={routeTitle}>
        <AdminTopbar
          authenticated={session.authenticated}
          email={session.email}
          onLogout={handleLogout}
          roles={session.roles}
        />
        {message ? <div className="inline-alert">{message}</div> : null}

        {activeRouteID === "workbench" ? (
          <>
            <ProductionOverviewPanel busy={busy} data={productionData} onRefresh={loadData} />
            <DangerOperationsPanel csrfToken={csrfToken} data={productionData} onCompleted={loadData} />
          </>
        ) : null}
        {activeRouteID === "catalog" ? <ModelManagementPanel csrfToken={csrfToken} /> : null}
        {activeRouteID === "routing" ? <ChannelManagementPanel csrfToken={csrfToken} /> : null}
        {activeRouteID === "accounts" ? (
          <>
            <CustomerAccountsPanel csrfToken={csrfToken} />
            <CreditOperationsPanel />
            <APIKeyManagementPanel csrfToken={csrfToken} />
          </>
        ) : null}
        {activeRouteID === "activity" ? <ActivityLogsPanel /> : null}
        {activeRouteID === "tools" ? <ToolsPlaygroundPanel csrfToken={csrfToken} /> : null}
        {activeRouteID === "settings" ? (
          <>
            <RoutePlanPanel />
            <ProductionOverviewPanel busy={busy} data={productionData} onRefresh={loadData} />
          </>
        ) : null}
      </section>
    </main>
  );
}
