import type { AdminBFFComponents } from "@token-gateway/api-client";

import { adminClient } from "../../shared/api/client";

export type AdminPlaygroundRunRequest =
  AdminBFFComponents["schemas"]["AdminPlaygroundRunRequest"];
export type AdminPlaygroundRunResult =
  AdminBFFComponents["schemas"]["AdminPlaygroundRunResult"];
export type AdminPlaygroundImportPreviewRequest =
  AdminBFFComponents["schemas"]["AdminPlaygroundImportPreviewRequest"];
export type AdminPlaygroundImportPreview =
  AdminBFFComponents["schemas"]["AdminPlaygroundImportPreview"];
export type AdminPlaygroundExportRequest =
  AdminBFFComponents["schemas"]["AdminPlaygroundExportRequest"];
export type AdminPlaygroundExport = AdminBFFComponents["schemas"]["AdminPlaygroundExport"];

export function runAdminPlayground(
  request: AdminPlaygroundRunRequest
): Promise<AdminPlaygroundRunResult> {
  return adminClient.request<AdminPlaygroundRunResult>("/api/admin/v1/playground/run", {
    method: "POST",
    body: request
  });
}

export function previewAdminPlaygroundImport(
  request: AdminPlaygroundImportPreviewRequest
): Promise<AdminPlaygroundImportPreview> {
  return adminClient.request<AdminPlaygroundImportPreview>(
    "/api/admin/v1/playground/import-preview",
    {
      method: "POST",
      body: request
    }
  );
}

export function exportAdminPlayground(
  request: AdminPlaygroundExportRequest
): Promise<AdminPlaygroundExport> {
  return adminClient.request<AdminPlaygroundExport>("/api/admin/v1/playground/export", {
    method: "POST",
    body: request
  });
}
