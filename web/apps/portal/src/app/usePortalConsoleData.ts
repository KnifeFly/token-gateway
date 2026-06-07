import { ConsoleAPIError } from "@token-gateway/api-client";
import { formatInteger } from "@token-gateway/format";
import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";

import { requestPortal } from "../shared/api/client";
import { errorMessage, moneyValue, primaryCreditBucket, splitModels } from "../shared/format";
import { portalCopy } from "../shared/i18n";
import type {
  APIKey,
  APIKeyCreateResponse,
  APIKeyRotateResponse,
  CreditLedgerResponse,
  CreditsResponse,
  Dashboard,
  ModelDetail,
  ModelList,
  ModelSchema,
  PlaygroundRunResult,
  PortalSchemas,
  ProjectSettings,
  Session,
  TaskList,
  UsageExportResponse,
  UsageResponse
} from "../shared/types";

type ActivityFilters = {
  apiKeyID: string;
  requestID: string;
  model: string;
  providerType: string;
  channelID: string;
  status: string;
  from: string;
  to: string;
};

const emptyActivityFilters: ActivityFilters = {
  apiKeyID: "",
  requestID: "",
  model: "",
  providerType: "",
  channelID: "",
  status: "",
  from: "",
  to: ""
};

function activityQueryPath(basePath: string, filters: ActivityFilters, limit = 10): string {
  const params = new URLSearchParams({ limit: String(limit) });
  const entries: Array<[string, string]> = [
    ["api_key_id", filters.apiKeyID],
    ["request_id", filters.requestID],
    ["model", filters.model],
    ["provider_type", filters.providerType],
    ["channel_id", filters.channelID],
    ["status", filters.status],
    ["from", filters.from],
    ["to", filters.to]
  ];
  for (const [key, value] of entries) {
    const trimmed = value.trim();
    if (trimmed) {
      params.set(key, trimmed);
    }
  }
  return `${basePath}?${params.toString()}`;
}

export function usePortalConsoleData() {
  const [session, setSession] = useState<Session>({ authenticated: false });
  const [csrfToken, setCSRFToken] = useState("");
  const [apiKey, setAPIKey] = useState("");
  const [busy, setBusy] = useState(true);
  const [message, setMessage] = useState("");
  const [dashboard, setDashboard] = useState<Dashboard>();
  const [models, setModels] = useState<ModelList>();
  const [selectedModel, setSelectedModel] = useState("");
  const [modelDetail, setModelDetail] = useState<ModelDetail>();
  const [modelSchema, setModelSchema] = useState<ModelSchema>();
  const [credits, setCredits] = useState<CreditsResponse>();
  const [creditLedger, setCreditLedger] = useState<CreditLedgerResponse>();
  const [usageExport, setUsageExport] = useState<UsageExportResponse>();
  const [usage, setUsage] = useState<UsageResponse>();
  const [apiKeys, setAPIKeys] = useState<APIKey[]>([]);
  const [tasks, setTasks] = useState<TaskList>();
  const [usageFilters, setUsageFilters] = useState<ActivityFilters>(emptyActivityFilters);
  const [taskFilters, setTaskFilters] = useState<ActivityFilters>(emptyActivityFilters);
  const [usageLimit, setUsageLimit] = useState(10);
  const [taskLimit, setTaskLimit] = useState(10);
  const [ledgerLimit, setLedgerLimit] = useState(20);
  const [playgroundResult, setPlaygroundResult] = useState<PlaygroundRunResult>();
  const [settings, setSettings] = useState<ProjectSettings>();
  const [derivedName, setDerivedName] = useState("");
  const [derivedModels, setDerivedModels] = useState("");
  const [derivedIPAllowlist, setDerivedIPAllowlist] = useState("");
  const [derivedExpiresAt, setDerivedExpiresAt] = useState("");
  const [createdKey, setCreatedKey] = useState<APIKeyCreateResponse | APIKeyRotateResponse>();

  async function portalRequest<TResponse>(
    path: string,
    init = {},
    csrfOverride = csrfToken
  ): Promise<TResponse> {
    return requestPortal<TResponse>(path, init, csrfOverride);
  }

  async function loadModelDetail(modelID: string, csrfOverride = csrfToken): Promise<void> {
    if (!modelID) {
      setModelDetail(undefined);
      setModelSchema(undefined);
      return;
    }
    const detail = await portalRequest<ModelDetail>(
      `/api/portal/v1/models/${encodeURIComponent(modelID)}`,
      {},
      csrfOverride
    );
    setSelectedModel(modelID);
    setModelDetail(detail);
    setModelSchema(detail.schema);
  }

  async function loadPortalData(csrfOverride = csrfToken): Promise<void> {
    const [
      nextDashboard,
      nextModels,
      nextCredits,
      nextCreditLedger,
      nextUsageExport,
      nextUsage,
      nextKeys,
      nextTasks,
      nextSettings
    ] = await Promise.all([
      portalRequest<Dashboard>("/api/portal/v1/dashboard", {}, csrfOverride),
      portalRequest<ModelList>("/api/portal/v1/models", {}, csrfOverride),
      portalRequest<CreditsResponse>("/api/portal/v1/credits", {}, csrfOverride),
      portalRequest<CreditLedgerResponse>(
        `/api/portal/v1/credits/ledger?limit=${ledgerLimit}`,
        {},
        csrfOverride
      ),
      portalRequest<UsageExportResponse>(
        `/api/portal/v1/usage/export?limit=${ledgerLimit}`,
        {},
        csrfOverride
      ),
      portalRequest<UsageResponse>(
        activityQueryPath("/api/portal/v1/usage", usageFilters, usageLimit),
        {},
        csrfOverride
      ),
      portalRequest<PortalSchemas["APIKeyListResponse"]>(
        "/api/portal/v1/api-keys",
        {},
        csrfOverride
      ),
      portalRequest<TaskList>(
        activityQueryPath("/api/portal/v1/tasks", taskFilters, taskLimit),
        {},
        csrfOverride
      ),
      portalRequest<ProjectSettings>("/api/portal/v1/settings/project", {}, csrfOverride)
    ]);

    setDashboard(nextDashboard);
    setModels(nextModels);
    setCredits(nextCredits);
    setCreditLedger(nextCreditLedger);
    setUsageExport(nextUsageExport);
    setUsage(nextUsage);
    setAPIKeys(nextKeys.data);
    setTasks(nextTasks);
    setSettings(nextSettings);

    const firstModel = nextModels.data[0]?.id ?? "";
    if (firstModel) {
      await loadModelDetail(firstModel, csrfOverride);
    } else {
      setSelectedModel("");
      setModelDetail(undefined);
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
        label: portalCopy.summary.credits,
        value: moneyValue(primaryCreditBucket(credits)?.remaining_credits),
        detail: primaryCreditBucket(credits)?.currency ?? portalCopy.summary.creditsFallback
      },
      {
        label: portalCopy.summary.requests,
        value: formatInteger(usage?.totals.requests ?? 0),
        detail: portalCopy.summary.requestsDetail
      },
      {
        label: portalCopy.summary.activeKeys,
        value: formatInteger(dashboard?.active_key_count ?? 0),
        detail: portalCopy.summary.totalKeys(formatInteger(apiKeys.length))
      },
      {
        label: portalCopy.summary.tasks,
        value: formatInteger(dashboard?.task_summary.total ?? 0),
        detail: portalCopy.summary.processingTasks(
          formatInteger(dashboard?.task_summary.processing ?? 0)
        )
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
      setModelDetail(undefined);
      setModelSchema(undefined);
      setCredits(undefined);
      setCreditLedger(undefined);
      setUsageExport(undefined);
      setUsage(undefined);
      setAPIKeys([]);
      setTasks(undefined);
      setUsageFilters(emptyActivityFilters);
      setTaskFilters(emptyActivityFilters);
      setUsageLimit(10);
      setTaskLimit(10);
      setLedgerLimit(20);
      setPlaygroundResult(undefined);
      setSettings(undefined);
      setDerivedName("");
      setDerivedModels("");
      setDerivedIPAllowlist("");
      setDerivedExpiresAt("");
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
      const ipAllowlist = splitModels(derivedIPAllowlist);
      const expiresAt = derivedExpiresAt ? new Date(derivedExpiresAt).toISOString() : undefined;
      const response = await portalRequest<APIKeyCreateResponse>("/api/portal/v1/api-keys", {
        method: "POST",
        body: {
          name: derivedName || portalCopy.defaults.derivedKeyName,
          ...(allowedModels.length > 0 ? { allowed_models: allowedModels } : {}),
          ...(ipAllowlist.length > 0 ? { ip_allowlist: ipAllowlist } : {}),
          ...(expiresAt ? { expires_at: expiresAt } : {})
        }
      });
      setCreatedKey(response);
      setDerivedName("");
      setDerivedModels("");
      setDerivedIPAllowlist("");
      setDerivedExpiresAt("");
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

  async function handleRotateKey(keyID: string) {
    setBusy(true);
    setMessage("");
    setCreatedKey(undefined);
    try {
      const response = await portalRequest<APIKeyRotateResponse>(
        `/api/portal/v1/api-keys/${encodeURIComponent(keyID)}/rotate`,
        {
          method: "POST"
        }
      );
      setCreatedKey(response);
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
    setPlaygroundResult(undefined);
    try {
      await loadModelDetail(modelID);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function handleApplyUsageFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    try {
      const nextUsage = await portalRequest<UsageResponse>(
        activityQueryPath("/api/portal/v1/usage", usageFilters, usageLimit)
      );
      setUsage(nextUsage);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function handleApplyTaskFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    try {
      const nextTasks = await portalRequest<TaskList>(
        activityQueryPath("/api/portal/v1/tasks", taskFilters, taskLimit)
      );
      setTasks(nextTasks);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function handleUsageLimitChange(nextLimit: number) {
    setUsageLimit(nextLimit);
    setBusy(true);
    setMessage("");
    try {
      const nextUsage = await portalRequest<UsageResponse>(
        activityQueryPath("/api/portal/v1/usage", usageFilters, nextLimit)
      );
      setUsage(nextUsage);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function handleTaskLimitChange(nextLimit: number) {
    setTaskLimit(nextLimit);
    setBusy(true);
    setMessage("");
    try {
      const nextTasks = await portalRequest<TaskList>(
        activityQueryPath("/api/portal/v1/tasks", taskFilters, nextLimit)
      );
      setTasks(nextTasks);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function handleLedgerLimitChange(nextLimit: number) {
    setLedgerLimit(nextLimit);
    setBusy(true);
    setMessage("");
    try {
      const [nextLedger, nextExport] = await Promise.all([
        portalRequest<CreditLedgerResponse>(`/api/portal/v1/credits/ledger?limit=${nextLimit}`),
        portalRequest<UsageExportResponse>(`/api/portal/v1/usage/export?limit=${nextLimit}`)
      ]);
      setCreditLedger(nextLedger);
      setUsageExport(nextExport);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function handleRunPlayground(request: PortalSchemas["PlaygroundRunRequest"]) {
    setBusy(true);
    setMessage("");
    setPlaygroundResult(undefined);
    try {
      const result = await portalRequest<PlaygroundRunResult>("/api/portal/v1/playground/run", {
        method: "POST",
        body: request
      });
      setPlaygroundResult(result);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  return {
    apiKey,
    apiKeys,
    busy,
    createdKey,
    creditLedger,
    credits,
    dashboard,
    derivedExpiresAt,
    derivedIPAllowlist,
    derivedModels,
    derivedName,
    handleApplyTaskFilters,
    handleApplyUsageFilters,
    handleCreateKey,
    handleDisableKey,
    handleLedgerLimitChange,
    handleLogin,
    handleLogout,
    handleRotateKey,
    handleRunPlayground,
    handleSelectModel,
    handleTaskLimitChange,
    handleUsageLimitChange,
    ledgerLimit,
    message,
    modelDetail,
    modelSchema,
    models,
    playgroundResult,
    selectedModel,
    session,
    setAPIKey,
    setDerivedExpiresAt,
    setDerivedIPAllowlist,
    setDerivedModels,
    setDerivedName,
    setTaskFilters,
    setUsageFilters,
    settings,
    summary,
    taskFilters,
    taskLimit,
    tasks,
    usage,
    usageExport,
    usageFilters,
    usageLimit
  };
}
