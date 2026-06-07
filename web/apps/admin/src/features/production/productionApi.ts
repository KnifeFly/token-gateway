import type { AdminBFFComponents } from "@token-gateway/api-client";

import { requestAdmin, type AdminMutationOptions } from "../../shared/api/client";

export type AdminSession = AdminBFFComponents["schemas"]["AdminSessionResponse"];
export type AdminLoginResponse = AdminBFFComponents["schemas"]["AdminLoginResponse"];
export type AdminDashboard = AdminBFFComponents["schemas"]["AdminDashboardResponse"];
export type AdminListResponse = AdminBFFComponents["schemas"]["ListResponse"];

export interface SnapshotSummary {
  active?: Record<string, unknown>;
  previous?: Record<string, unknown>;
}

export interface SnapshotOperationResult {
  version?: string;
  checksum?: string;
  schema_version?: string;
  created_at?: string;
}

export interface ReplayResult {
  requested_id?: string;
  replayed: number;
}

export interface ProductionData {
  dashboard?: AdminDashboard;
  tenants: AdminListResponse;
  projects: AdminListResponse;
  routes: AdminListResponse;
  pricing: AdminListResponse;
  limits: AdminListResponse;
  snapshots?: SnapshotSummary;
  settlements: AdminListResponse;
  callbacks: AdminListResponse;
  workers: AdminListResponse;
  holds: AdminListResponse;
  audit: AdminListResponse;
  operators: AdminListResponse;
  errors: string[];
  loadedAt: string;
}

export function emptyList(): AdminListResponse {
  return { data: [] };
}

export async function adminLogin(email: string, password: string): Promise<AdminLoginResponse> {
  return requestAdmin<AdminLoginResponse>("/api/admin/v1/auth/login", {
    method: "POST",
    body: { email: email.trim(), password }
  });
}

export async function adminLogout(csrfToken: string): Promise<void> {
  await requestAdmin("/api/admin/v1/auth/logout", { method: "POST" }, { csrfToken });
}

export function getAdminSession(): Promise<AdminSession> {
  return requestAdmin<AdminSession>("/api/admin/v1/auth/me");
}

async function safeRequest<TResponse>(
  label: string,
  request: Promise<TResponse>,
  fallback: TResponse,
  errors: string[]
): Promise<TResponse> {
  try {
    return await request;
  } catch (error) {
    errors.push(`${label}: ${error instanceof Error ? error.message : String(error)}`);
    return fallback;
  }
}

export async function loadProductionData(): Promise<ProductionData> {
  const errors: string[] = [];
  const [
    dashboard,
    tenants,
    projects,
    routes,
    pricing,
    limits,
    snapshots,
    settlements,
    callbacks,
    workers,
    holds,
    audit,
    operators
  ] = await Promise.all([
    safeRequest("dashboard", requestAdmin<AdminDashboard>("/api/admin/v1/dashboard"), undefined, errors),
    safeRequest("tenants", requestAdmin<AdminListResponse>("/api/admin/v1/tenants"), emptyList(), errors),
    safeRequest("projects", requestAdmin<AdminListResponse>("/api/admin/v1/projects"), emptyList(), errors),
    safeRequest("routes", requestAdmin<AdminListResponse>("/api/admin/v1/routes"), emptyList(), errors),
    safeRequest("pricing", requestAdmin<AdminListResponse>("/api/admin/v1/pricing"), emptyList(), errors),
    safeRequest("limits", requestAdmin<AdminListResponse>("/api/admin/v1/limits"), emptyList(), errors),
    safeRequest(
      "snapshots",
      requestAdmin<SnapshotSummary>("/api/admin/v1/snapshots"),
      undefined,
      errors
    ),
    safeRequest(
      "settlements",
      requestAdmin<AdminListResponse>("/api/admin/v1/operations/settlements"),
      emptyList(),
      errors
    ),
    safeRequest(
      "callbacks",
      requestAdmin<AdminListResponse>("/api/admin/v1/operations/callbacks?limit=10"),
      emptyList(),
      errors
    ),
    safeRequest(
      "workers",
      requestAdmin<AdminListResponse>("/api/admin/v1/operations/workers"),
      emptyList(),
      errors
    ),
    safeRequest(
      "holds",
      requestAdmin<AdminListResponse>("/api/admin/v1/operations/holds"),
      emptyList(),
      errors
    ),
    safeRequest("audit", requestAdmin<AdminListResponse>("/api/admin/v1/audit?limit=12"), emptyList(), errors),
    safeRequest("operators", requestAdmin<AdminListResponse>("/api/admin/v1/operators"), emptyList(), errors)
  ]);

  return {
    dashboard,
    tenants,
    projects,
    routes,
    pricing,
    limits,
    snapshots,
    settlements,
    callbacks,
    workers,
    holds,
    audit,
    operators,
    errors,
    loadedAt: new Date().toISOString()
  };
}

export function validateSnapshot(mutation: AdminMutationOptions): Promise<Record<string, unknown>> {
  return requestAdmin<Record<string, unknown>>(
    "/api/admin/v1/snapshots/validate",
    { method: "POST", body: {} },
    mutation
  );
}

export function publishSnapshot(mutation: AdminMutationOptions): Promise<SnapshotOperationResult> {
  return requestAdmin<SnapshotOperationResult>(
    "/api/admin/v1/snapshots/publish",
    { method: "POST", body: {} },
    mutation
  );
}

export function rollbackSnapshot(mutation: AdminMutationOptions): Promise<SnapshotOperationResult> {
  return requestAdmin<SnapshotOperationResult>(
    "/api/admin/v1/snapshots/rollback",
    { method: "POST", body: {} },
    mutation
  );
}

export function replaySettlement(
  settlementID: string,
  mutation: AdminMutationOptions
): Promise<ReplayResult> {
  return requestAdmin<ReplayResult>(
    `/api/admin/v1/operations/settlements/${encodeURIComponent(settlementID)}/replay`,
    { method: "POST", body: {} },
    mutation
  );
}

export function retryCallback(
  callbackID: string,
  mutation: AdminMutationOptions
): Promise<ReplayResult> {
  return requestAdmin<ReplayResult>(
    `/api/admin/v1/operations/callbacks/${encodeURIComponent(callbackID)}/retry`,
    { method: "POST", body: {} },
    mutation
  );
}
