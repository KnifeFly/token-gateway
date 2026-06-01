import {
  ConsoleAPIError,
  createConsoleClient,
  type PortalBFFComponents
} from "@token-gateway/api-client";
import { sessionStateLabel } from "@token-gateway/auth";
import { formatISODateTime, formatInteger } from "@token-gateway/format";
import { Button, StatusBadge } from "@token-gateway/ui";
import { FormEvent, useEffect, useMemo, useState } from "react";

type PortalSchemas = PortalBFFComponents["schemas"];
type APIKey = PortalSchemas["APIKey"];
type APIKeyCreateResponse = PortalSchemas["APIKeyCreateResponse"];
type CreditsResponse = PortalSchemas["CreditsResponse"];
type Dashboard = PortalSchemas["PortalDashboardResponse"];
type ModelList = PortalSchemas["ModelListResponse"];
type ModelSchema = PortalSchemas["ModelSchemaResponse"];
type Onboarding = PortalSchemas["OnboardingState"];
type ProjectSettings = PortalSchemas["ProjectSettings"];
type Session = PortalSchemas["PortalSessionResponse"];
type TaskList = PortalSchemas["TaskListResponse"];
type TaskObject = PortalSchemas["TaskObject"];
type UsageResponse = PortalSchemas["UsageResponse"];

type ViewID =
  | "dashboard"
  | "models"
  | "credits"
  | "usage"
  | "api-keys"
  | "tasks"
  | "onboarding"
  | "settings";

const client = createConsoleClient();

const navItems: Array<{ id: ViewID; label: string }> = [
  { id: "dashboard", label: "Dashboard" },
  { id: "models", label: "Models" },
  { id: "credits", label: "Credits" },
  { id: "usage", label: "Usage" },
  { id: "api-keys", label: "API Keys" },
  { id: "tasks", label: "Tasks" },
  { id: "onboarding", label: "Onboarding" },
  { id: "settings", label: "Settings" }
];

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
    return createConsoleClient({ csrfToken: csrfOverride }).request<TResponse>(path, init);
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
      <main className="login-shell">
        <form className="login-panel" onSubmit={handleLogin}>
          <div className="brand login-brand">
            <span className="brand-mark">TG</span>
            <span>Portal</span>
          </div>
          <label className="field">
            <span>Customer API key</span>
            <input
              autoComplete="off"
              name="api_key"
              onChange={(event) => setAPIKey(event.target.value)}
              placeholder="tg-..."
              type="password"
              value={apiKey}
            />
          </label>
          {message ? <div className="error-banner">{message}</div> : null}
          <Button disabled={busy || apiKey.trim() === ""} type="submit" variant="primary">
            {busy ? "Signing in" : "Sign in"}
          </Button>
        </form>
      </main>
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
            <p>{client.baseURL}/api/portal/v1</p>
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

function DashboardView({
  dashboard,
  usage,
  tasks
}: {
  dashboard?: Dashboard;
  usage?: UsageResponse;
  tasks: TaskObject[];
}) {
  return (
    <section className="panel-grid">
      <article className="panel">
        <PanelHeading title="Usage" meta={dashboard?.generated_at} />
        <div className="stat-row">
          <span>Requests</span>
          <strong>{formatInteger(usage?.totals.requests ?? 0)}</strong>
        </div>
        <div className="stat-row">
          <span>Input tokens</span>
          <strong>{formatInteger(usage?.totals.input_tokens ?? 0)}</strong>
        </div>
        <div className="stat-row">
          <span>Output tokens</span>
          <strong>{formatInteger(usage?.totals.output_tokens ?? 0)}</strong>
        </div>
      </article>
      <article className="panel">
        <PanelHeading title="Tasks" />
        <div className="task-summary">
          <StatusBadge tone="neutral">{`Queued ${formatInteger(dashboard?.task_summary.queued ?? 0)}`}</StatusBadge>
          <StatusBadge tone="warning">{`Processing ${formatInteger(dashboard?.task_summary.processing ?? 0)}`}</StatusBadge>
          <StatusBadge tone="success">{`Completed ${formatInteger(dashboard?.task_summary.completed ?? 0)}`}</StatusBadge>
        </div>
        <TaskListRows tasks={tasks.slice(0, 4)} />
      </article>
    </section>
  );
}

function ModelsView({
  modelSchema,
  models,
  onSelectModel,
  selectedModel
}: {
  modelSchema?: ModelSchema;
  models?: ModelList;
  onSelectModel(modelID: string): void;
  selectedModel: string;
}) {
  return (
    <section className="panel-grid split">
      <article className="panel">
        <PanelHeading title="Models" />
        <div className="table" role="table">
          <div className="table-row table-head" role="row">
            <span role="columnheader">Model</span>
            <span role="columnheader">Type</span>
            <span role="columnheader">Mode</span>
          </div>
          {(models?.data ?? []).map((model) => (
            <button
              className="table-row button-row"
              key={model.id}
              onClick={() => onSelectModel(model.id)}
              role="row"
              type="button"
            >
              <span role="cell">{model.display_name}</span>
              <span role="cell">{model.type}</span>
              <span role="cell">{model.async ? "Async" : "Sync"}</span>
            </button>
          ))}
        </div>
      </article>
      <article className="panel">
        <PanelHeading title="Schema" meta={selectedModel} />
        <pre className="schema-box">
          {modelSchema ? JSON.stringify(modelSchema.schema, null, 2) : "{}"}
        </pre>
      </article>
    </section>
  );
}

function CreditsView({ credits }: { credits?: CreditsResponse }) {
  const buckets = Object.entries(credits?.data ?? {});
  return (
    <section className="panel">
      <PanelHeading title="Credits" />
      <div className="table" role="table">
        <div className="table-row table-head four" role="row">
          <span role="columnheader">Bucket</span>
          <span role="columnheader">Remaining</span>
          <span role="columnheader">Used</span>
          <span role="columnheader">Held</span>
        </div>
        {buckets.map(([name, bucket]) => (
          <div className="table-row four" key={name} role="row">
            <span role="cell">{name}</span>
            <span role="cell">
              {moneyValue(bucket.remaining_credits)} {bucket.currency}
            </span>
            <span role="cell">
              {moneyValue(bucket.used_credits)} {bucket.currency}
            </span>
            <span role="cell">
              {moneyValue(bucket.held_credits ?? 0)} {bucket.currency}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}

function UsageView({ usage }: { usage?: UsageResponse }) {
  return (
    <section className="panel">
      <PanelHeading title="Usage" meta={usage?.generated_at} />
      <div className="table" role="table">
        <div className="table-row table-head usage" role="row">
          <span role="columnheader">Status</span>
          <span role="columnheader">Model</span>
          <span role="columnheader">Input</span>
          <span role="columnheader">Output</span>
          <span role="columnheader">Credits</span>
        </div>
        {(usage?.items ?? []).map((item, index) => (
          <div
            className="table-row usage"
            key={`${item.request_id ?? item.status}-${index}`}
            role="row"
          >
            <span role="cell">{item.status}</span>
            <span role="cell">{item.model ?? item.capability ?? "ledger"}</span>
            <span role="cell">{formatInteger(item.input_tokens)}</span>
            <span role="cell">{formatInteger(item.output_tokens)}</span>
            <span role="cell">{moneyValue(item.credits_used)}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function APIKeysView({
  apiKeys,
  createdKey,
  currentKeyID,
  derivedModels,
  derivedName,
  onCreateKey,
  onDisableKey,
  setDerivedModels,
  setDerivedName
}: {
  apiKeys: APIKey[];
  createdKey?: APIKeyCreateResponse;
  currentKeyID: string;
  derivedModels: string;
  derivedName: string;
  onCreateKey(event: FormEvent<HTMLFormElement>): void;
  onDisableKey(keyID: string): void;
  setDerivedModels(value: string): void;
  setDerivedName(value: string): void;
}) {
  return (
    <section className="panel-grid split">
      <article className="panel">
        <PanelHeading title="API Keys" />
        <div className="table" role="table">
          <div className="table-row table-head keys" role="row">
            <span role="columnheader">Name</span>
            <span role="columnheader">Models</span>
            <span role="columnheader">State</span>
            <span role="columnheader">Action</span>
          </div>
          {apiKeys.map((key) => (
            <div className="table-row keys" key={key.id} role="row">
              <span role="cell">{key.name}</span>
              <span role="cell">{key.allowed_models.join(", ")}</span>
              <span role="cell">{key.enabled ? "Enabled" : "Disabled"}</span>
              <span role="cell">
                <Button
                  disabled={!key.enabled || key.id === currentKeyID}
                  onClick={() => onDisableKey(key.id)}
                  variant="ghost"
                >
                  Disable
                </Button>
              </span>
            </div>
          ))}
        </div>
      </article>
      <article className="panel">
        <PanelHeading title="Create Key" />
        <form className="stacked-form" onSubmit={onCreateKey}>
          <label className="field">
            <span>Name</span>
            <input onChange={(event) => setDerivedName(event.target.value)} value={derivedName} />
          </label>
          <label className="field">
            <span>Allowed models</span>
            <input
              onChange={(event) => setDerivedModels(event.target.value)}
              placeholder="gpt-public"
              value={derivedModels}
            />
          </label>
          <Button type="submit" variant="primary">
            Create
          </Button>
        </form>
        {createdKey ? (
          <div className="secret-box">
            <span>{createdKey.api_key.name}</span>
            <code>{createdKey.plaintext_key}</code>
          </div>
        ) : null}
      </article>
    </section>
  );
}

function TasksView({ tasks }: { tasks: TaskObject[] }) {
  return (
    <section className="panel">
      <PanelHeading title="Tasks" />
      <TaskListRows tasks={tasks} />
    </section>
  );
}

function TaskListRows({ tasks }: { tasks: TaskObject[] }) {
  return (
    <div className="table" role="table">
      <div className="table-row table-head tasks" role="row">
        <span role="columnheader">Task</span>
        <span role="columnheader">Status</span>
        <span role="columnheader">Model</span>
        <span role="columnheader">Created</span>
      </div>
      {tasks.map((task, index) => (
        <div className="table-row tasks" key={taskString(task, "id") || index} role="row">
          <span role="cell">{taskString(task, "id") || "task"}</span>
          <span role="cell">{taskString(task, "status") || "unknown"}</span>
          <span role="cell">{taskString(task, "model") || taskString(task, "endpoint")}</span>
          <span role="cell">{formatMaybeDate(taskString(task, "created_at"))}</span>
        </div>
      ))}
    </div>
  );
}

function OnboardingView({ onboarding }: { onboarding?: Onboarding }) {
  return (
    <section className="panel">
      <PanelHeading title="Onboarding" meta={onboarding?.generated_at} />
      <div className="step-list">
        {(onboarding?.steps ?? []).map((step) => (
          <div className="step-row" key={step.id}>
            <StatusBadge tone={step.complete ? "success" : "warning"}>
              {step.complete ? "Done" : "Open"}
            </StatusBadge>
            <strong>{step.title}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function SettingsView({ session, settings }: { session: Session; settings?: ProjectSettings }) {
  return (
    <section className="panel">
      <PanelHeading title="Project" meta={settings?.generated_at} />
      <dl className="settings-list">
        <div>
          <dt>Tenant</dt>
          <dd>{settings?.tenant_id ?? session.tenant_id}</dd>
        </div>
        <div>
          <dt>Project</dt>
          <dd>{settings?.project_id ?? session.project_id}</dd>
        </div>
        <div>
          <dt>API key</dt>
          <dd>{settings?.api_key_id ?? session.api_key_id}</dd>
        </div>
        <div>
          <dt>Allowed models</dt>
          <dd>{(settings?.allowed_models ?? session.allowed_models ?? []).join(", ")}</dd>
        </div>
      </dl>
    </section>
  );
}

function PanelHeading({ meta, title }: { meta?: string; title: string }) {
  return (
    <div className="panel-heading">
      <h2>{title}</h2>
      {meta ? <span>{formatMaybeDate(meta)}</span> : null}
    </div>
  );
}

function primaryCreditBucket(credits?: CreditsResponse) {
  return credits?.data.token ?? Object.values(credits?.data ?? {})[0];
}

function moneyValue(value = 0): string {
  return new Intl.NumberFormat("en", { maximumFractionDigits: 4 }).format(value);
}

function formatMaybeDate(value?: string): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return formatISODateTime(date);
}

function splitModels(value: string): string[] {
  return value
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

function taskString(task: TaskObject, key: string): string {
  const value = task[key];
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number") {
    return String(value);
  }
  return "";
}

function errorMessage(error: unknown): string {
  if (error instanceof ConsoleAPIError) {
    return `${error.code}: ${error.message}`;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "request failed";
}
