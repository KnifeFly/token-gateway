import { ConsoleAPIError } from "@token-gateway/api-client";
import { sessionStateLabel } from "@token-gateway/auth";
import { formatInteger } from "@token-gateway/format";
import { Button, StatusBadge } from "@token-gateway/ui";
import { type FormEvent, useEffect, useMemo, useState } from "react";

import { APIKeysView } from "../features/api-keys/APIKeysView";
import { LoginView } from "../features/auth/LoginView";
import { CreditsView } from "../features/credits/CreditsView";
import { DashboardView } from "../features/dashboard/DashboardView";
import { ModelsView } from "../features/models/ModelsView";
import { OnboardingView } from "../features/onboarding/OnboardingView";
import { SettingsView } from "../features/settings/SettingsView";
import { TasksView } from "../features/tasks/TasksView";
import { UsageView } from "../features/usage/UsageView";
import { errorMessage, moneyValue, primaryCreditBucket, splitModels } from "../shared/format";
import type {
  APIKey,
  APIKeyCreateResponse,
  CreditsResponse,
  Dashboard,
  ModelList,
  ModelSchema,
  Onboarding,
  PortalSchemas,
  ProjectSettings,
  Session,
  TaskList,
  UsageResponse
} from "../shared/types";
import { navItems, type ViewID } from "./routes";
import { portalClient, requestPortal } from "../shared/api/client";

export function App() {
  const [session, setSession] = useState<Session>({ authenticated: false });
  const [csrfToken, setCSRFToken] = useState("");
  const [apiKey, setAPIKey] = useState("");
  const [activeView, setActiveView] = useState<ViewID>("dashboard");
  const [busy, setBusy] = useState(true);
  const [message, setMessage] = useState("");
  const [dashboard, setDashboard] = useState<Dashboard>();
  const [models, setModels] = useState<ModelList>();
  const [selectedModel, setSelectedModel] = useState("");
  const [modelSchema, setModelSchema] = useState<ModelSchema>();
  const [credits, setCredits] = useState<CreditsResponse>();
  const [usage, setUsage] = useState<UsageResponse>();
  const [apiKeys, setAPIKeys] = useState<APIKey[]>([]);
  const [tasks, setTasks] = useState<TaskList>();
  const [onboarding, setOnboarding] = useState<Onboarding>();
  const [settings, setSettings] = useState<ProjectSettings>();
  const [derivedName, setDerivedName] = useState("");
  const [derivedModels, setDerivedModels] = useState("");
  const [createdKey, setCreatedKey] = useState<APIKeyCreateResponse>();

  async function portalRequest<TResponse>(
    path: string,
    init = {},
    csrfOverride = csrfToken
  ): Promise<TResponse> {
    return requestPortal<TResponse>(path, init, csrfOverride);
  }

  async function loadModelSchema(modelID: string): Promise<void> {
    if (!modelID) {
      setModelSchema(undefined);
      return;
    }
    const schema = await portalRequest<ModelSchema>(
      `/api/portal/v1/models/${encodeURIComponent(modelID)}/schema`
    );
    setSelectedModel(modelID);
    setModelSchema(schema);
  }

  async function loadPortalData(csrfOverride = csrfToken): Promise<void> {
    const [
      nextDashboard,
      nextOnboarding,
      nextModels,
      nextCredits,
      nextUsage,
      nextKeys,
      nextTasks,
      nextSettings
    ] = await Promise.all([
      portalRequest<Dashboard>("/api/portal/v1/dashboard", {}, csrfOverride),
      portalRequest<Onboarding>("/api/portal/v1/onboarding", {}, csrfOverride),
      portalRequest<ModelList>("/api/portal/v1/models", {}, csrfOverride),
      portalRequest<CreditsResponse>("/api/portal/v1/credits", {}, csrfOverride),
      portalRequest<UsageResponse>("/api/portal/v1/usage?limit=10", {}, csrfOverride),
      portalRequest<PortalSchemas["APIKeyListResponse"]>(
        "/api/portal/v1/api-keys",
        {},
        csrfOverride
      ),
      portalRequest<TaskList>("/api/portal/v1/tasks?limit=10", {}, csrfOverride),
      portalRequest<ProjectSettings>("/api/portal/v1/settings/project", {}, csrfOverride)
    ]);

    setDashboard(nextDashboard);
    setOnboarding(nextOnboarding);
    setModels(nextModels);
    setCredits(nextCredits);
    setUsage(nextUsage);
    setAPIKeys(nextKeys.data);
    setTasks(nextTasks);
    setSettings(nextSettings);

    const firstModel = nextModels.data[0]?.id ?? "";
    if (firstModel) {
      const schema = await portalRequest<ModelSchema>(
        `/api/portal/v1/models/${encodeURIComponent(firstModel)}/schema`,
        {},
        csrfOverride
      );
      setSelectedModel(firstModel);
      setModelSchema(schema);
    } else {
      setSelectedModel("");
      setModelSchema(undefined);
    }
  }

  useEffect(() => {
    let active = true;

    async function restoreSession() {
      try {
        const currentSession = await portalRequest<Session>("/api/portal/v1/auth/me");
        if (!active) {
          return;
        }
        setSession(currentSession);
        setCSRFToken(currentSession.csrf_token ?? "");
        await loadPortalData(currentSession.csrf_token ?? "");
      } catch (error) {
        if (!active) {
          return;
        }
        if (!(error instanceof ConsoleAPIError) || error.status !== 401) {
          setMessage(errorMessage(error));
        }
        setSession({ authenticated: false });
      } finally {
        if (active) {
          setBusy(false);
        }
      }
    }

    void restoreSession();
    return () => {
      active = false;
    };
  }, []);

  const summary = useMemo(
    () => [
      {
        label: "Credits",
        value: moneyValue(primaryCreditBucket(credits)?.remaining_credits),
        detail: primaryCreditBucket(credits)?.currency ?? "USD"
      },
      {
        label: "Requests",
        value: formatInteger(usage?.totals.requests ?? 0),
        detail: "Settled usage"
      },
      {
        label: "Active keys",
        value: formatInteger(dashboard?.active_key_count ?? 0),
        detail: `${formatInteger(apiKeys.length)} total`
      },
      {
        label: "Tasks",
        value: formatInteger(dashboard?.task_summary.total ?? 0),
        detail: `${formatInteger(dashboard?.task_summary.processing ?? 0)} processing`
      }
    ],
    [apiKeys.length, credits, dashboard, usage]
  );

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    setCreatedKey(undefined);
    try {
      const response = await portalRequest<PortalSchemas["LoginResponse"]>(
        "/api/portal/v1/auth/api-key-login",
        {
          method: "POST",
          body: { api_key: apiKey }
        }
      );
      setAPIKey("");
      setSession(response.session);
      setCSRFToken(response.csrf_token);
      await loadPortalData(response.csrf_token);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function handleLogout() {
    setBusy(true);
    setMessage("");
    try {
      await portalRequest<{ authenticated: false }>("/api/portal/v1/auth/logout", {
        method: "POST"
      });
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setSession({ authenticated: false });
      setCSRFToken("");
      setDashboard(undefined);
      setModels(undefined);
      setModelSchema(undefined);
      setCredits(undefined);
      setUsage(undefined);
      setAPIKeys([]);
      setTasks(undefined);
      setOnboarding(undefined);
      setSettings(undefined);
      setCreatedKey(undefined);
      setBusy(false);
    }
  }

  async function handleCreateKey(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    setCreatedKey(undefined);
    try {
      const allowedModels = splitModels(derivedModels);
      const response = await portalRequest<APIKeyCreateResponse>("/api/portal/v1/api-keys", {
        method: "POST",
        body: {
          name: derivedName || "derived key",
          ...(allowedModels.length > 0 ? { allowed_models: allowedModels } : {})
        }
      });
      setCreatedKey(response);
      setDerivedName("");
      setDerivedModels("");
      await loadPortalData();
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function handleDisableKey(keyID: string) {
    setBusy(true);
    setMessage("");
    try {
      await portalRequest<APIKey>(`/api/portal/v1/api-keys/${encodeURIComponent(keyID)}/disable`, {
        method: "POST"
      });
      await loadPortalData();
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function handleSelectModel(modelID: string) {
    setBusy(true);
    setMessage("");
    try {
      await loadModelSchema(modelID);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  if (!session.authenticated) {
    return (
      <LoginView
        apiKey={apiKey}
        busy={busy}
        message={message}
        onAPIKeyChange={setAPIKey}
        onSubmit={handleLogin}
      />
    );
  }

  return (
    <main className="app-shell">
      <aside className="sidebar" aria-label="Portal navigation">
        <div className="brand">
          <span className="brand-mark">TG</span>
          <span>Portal</span>
        </div>
        <nav>
          {navItems.map((item) => (
            <button
              className={activeView === item.id ? "active" : ""}
              key={item.id}
              onClick={() => setActiveView(item.id)}
              type="button"
            >
              {item.label}
            </button>
          ))}
        </nav>
      </aside>

      <section className="workspace" aria-label="Portal workspace">
        <header className="topbar">
          <div>
            <h1>Portal</h1>
            <p>{portalClient.baseURL}/api/portal/v1</p>
          </div>
          <div className="topbar-actions">
            <StatusBadge tone={busy ? "warning" : "success"}>
              {busy ? "Syncing" : sessionStateLabel({ authenticated: true })}
            </StatusBadge>
            <Button disabled={busy} onClick={handleLogout} variant="secondary">
              Logout
            </Button>
          </div>
        </header>

        {message ? <div className="error-banner">{message}</div> : null}

        <section className="summary-grid" aria-label="Portal summary">
          {summary.map((item) => (
            <article className="metric-card" key={item.label}>
              <span>{item.label}</span>
              <strong>{item.value}</strong>
              <small>{item.detail}</small>
            </article>
          ))}
        </section>

        {activeView === "dashboard" ? (
          <DashboardView dashboard={dashboard} usage={usage} tasks={tasks?.data ?? []} />
        ) : null}
        {activeView === "models" ? (
          <ModelsView
            modelSchema={modelSchema}
            models={models}
            onSelectModel={handleSelectModel}
            selectedModel={selectedModel}
          />
        ) : null}
        {activeView === "credits" ? <CreditsView credits={credits} /> : null}
        {activeView === "usage" ? <UsageView usage={usage} /> : null}
        {activeView === "api-keys" ? (
          <APIKeysView
            apiKeys={apiKeys}
            createdKey={createdKey}
            currentKeyID={session.api_key_id ?? ""}
            derivedModels={derivedModels}
            derivedName={derivedName}
            onCreateKey={handleCreateKey}
            onDisableKey={handleDisableKey}
            setDerivedModels={setDerivedModels}
            setDerivedName={setDerivedName}
          />
        ) : null}
        {activeView === "tasks" ? <TasksView tasks={tasks?.data ?? []} /> : null}
        {activeView === "onboarding" ? <OnboardingView onboarding={onboarding} /> : null}
        {activeView === "settings" ? <SettingsView settings={settings} session={session} /> : null}
      </section>
    </main>
  );
}
