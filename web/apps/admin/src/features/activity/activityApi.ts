import type { AdminBFFComponents } from "@token-gateway/api-client";

import { adminClient } from "../../shared/api/client";

export type AdminUsageLogView = AdminBFFComponents["schemas"]["AdminUsageLogView"];
export type AdminUsageLogListResponse =
  AdminBFFComponents["schemas"]["AdminUsageLogListResponse"];
export type AdminUsageLogDetail = AdminBFFComponents["schemas"]["AdminUsageLogDetail"];
export type AdminTaskLogView = AdminBFFComponents["schemas"]["AdminTaskLogView"];
export type AdminTaskLogListResponse =
  AdminBFFComponents["schemas"]["AdminTaskLogListResponse"];
export type AdminTaskLogDetail = AdminBFFComponents["schemas"]["AdminTaskLogDetail"];

export function listAdminUsageLogs(): Promise<AdminUsageLogListResponse> {
  return adminClient.request<AdminUsageLogListResponse>("/api/admin/v1/usage-logs");
}

export function getAdminUsageLog(requestID: string): Promise<AdminUsageLogDetail> {
  return adminClient.request<AdminUsageLogDetail>(
    `/api/admin/v1/usage-logs/${encodeURIComponent(requestID)}`
  );
}

export function listAdminTaskLogs(): Promise<AdminTaskLogListResponse> {
  return adminClient.request<AdminTaskLogListResponse>("/api/admin/v1/task-logs");
}

export function getAdminTaskLog(taskID: string): Promise<AdminTaskLogDetail> {
  return adminClient.request<AdminTaskLogDetail>(
    `/api/admin/v1/task-logs/${encodeURIComponent(taskID)}`
  );
}
