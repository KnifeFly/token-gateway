import type { AdminBFFComponents } from "@token-gateway/api-client";

import { requestAdmin, type AdminMutationOptions } from "../../shared/api/client";

export type AdminModelView = AdminBFFComponents["schemas"]["AdminModelView"];
export type AdminModelListResponse = AdminBFFComponents["schemas"]["AdminModelListResponse"];
export type AdminModelUpsertRequest =
  AdminBFFComponents["schemas"]["AdminModelUpsertRequest"];
export type AdminModelSchemaPreview =
  AdminBFFComponents["schemas"]["AdminModelSchemaPreview"];
export type AdminModelChannelCoverage =
  AdminBFFComponents["schemas"]["AdminModelChannelCoverage"];
export type AdminModelCatalogSyncPreviewRequest =
  AdminBFFComponents["schemas"]["AdminModelCatalogSyncPreviewRequest"];
export type AdminModelCatalogSyncPreview =
  AdminBFFComponents["schemas"]["AdminModelCatalogSyncPreview"];

export function listAdminModels(): Promise<AdminModelListResponse> {
  return requestAdmin<AdminModelListResponse>("/api/admin/v1/models");
}

export function getAdminModel(modelID: string): Promise<AdminModelView> {
  return requestAdmin<AdminModelView>(`/api/admin/v1/models/${encodeURIComponent(modelID)}`);
}

export function upsertAdminModel(
  request: AdminModelUpsertRequest,
  mutation: AdminMutationOptions
): Promise<AdminModelView> {
  return requestAdmin<AdminModelView>(
    "/api/admin/v1/models",
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function patchAdminModel(
  modelID: string,
  request: AdminModelUpsertRequest,
  mutation: AdminMutationOptions
): Promise<AdminModelView> {
  return requestAdmin<AdminModelView>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}`,
    {
      method: "PATCH",
      body: request
    },
    mutation
  );
}

export function listAdminModelChannels(
  modelID: string
): Promise<{ data: AdminModelChannelCoverage[] }> {
  return requestAdmin<{ data: AdminModelChannelCoverage[] }>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}/channels`
  );
}

export function getAdminModelSchemaPreview(modelID: string): Promise<AdminModelSchemaPreview> {
  return requestAdmin<AdminModelSchemaPreview>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}/schema-preview`
  );
}

export function disableAdminModel(
  modelID: string,
  mutation: AdminMutationOptions
): Promise<AdminModelView> {
  return requestAdmin<AdminModelView>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}/disable`,
    { method: "POST" },
    mutation
  );
}

export function deprecateAdminModel(
  modelID: string,
  mutation: AdminMutationOptions
): Promise<AdminModelView> {
  return requestAdmin<AdminModelView>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}/deprecate`,
    { method: "POST" },
    mutation
  );
}

export function previewAdminModelCatalogSync(
  request: AdminModelCatalogSyncPreviewRequest,
  mutation: AdminMutationOptions
): Promise<AdminModelCatalogSyncPreview> {
  return requestAdmin<AdminModelCatalogSyncPreview>(
    "/api/admin/v1/models/sync-preview",
    {
      method: "POST",
      body: request
    },
    mutation
  );
}
