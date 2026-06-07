import type { AdminBFFComponents } from "@token-gateway/api-client";

import { requestAdmin, type AdminMutationOptions } from "../../shared/api/client";

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
  request: AdminPlaygroundRunRequest,
  mutation: AdminMutationOptions
): Promise<AdminPlaygroundRunResult> {
  return requestAdmin<AdminPlaygroundRunResult>(
    "/api/admin/v1/playground/run",
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function previewAdminPlaygroundImport(
  request: AdminPlaygroundImportPreviewRequest,
  mutation: AdminMutationOptions
): Promise<AdminPlaygroundImportPreview> {
  return requestAdmin<AdminPlaygroundImportPreview>(
    "/api/admin/v1/playground/import-preview",
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function exportAdminPlayground(
  request: AdminPlaygroundExportRequest,
  mutation: AdminMutationOptions
): Promise<AdminPlaygroundExport> {
  return requestAdmin<AdminPlaygroundExport>(
    "/api/admin/v1/playground/export",
    {
      method: "POST",
      body: request
    },
    mutation
  );
}
