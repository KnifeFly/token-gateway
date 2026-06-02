import type { AdminBFFComponents } from "@token-gateway/api-client";

import { adminClient } from "../../shared/api/client";

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
  return adminClient.request<AdminModelListResponse>("/api/admin/v1/models");
}

export function getAdminModel(modelID: string): Promise<AdminModelView> {
  return adminClient.request<AdminModelView>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}`
  );
}

export function upsertAdminModel(request: AdminModelUpsertRequest): Promise<AdminModelView> {
  return adminClient.request<AdminModelView>("/api/admin/v1/models", {
    method: "POST",
    body: request
  });
}

export function patchAdminModel(
  modelID: string,
  request: AdminModelUpsertRequest
): Promise<AdminModelView> {
  return adminClient.request<AdminModelView>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}`,
    {
      method: "PATCH",
      body: request
    }
  );
}

export function listAdminModelChannels(
  modelID: string
): Promise<{ data: AdminModelChannelCoverage[] }> {
  return adminClient.request<{ data: AdminModelChannelCoverage[] }>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}/channels`
  );
}

export function getAdminModelSchemaPreview(modelID: string): Promise<AdminModelSchemaPreview> {
  return adminClient.request<AdminModelSchemaPreview>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}/schema-preview`
  );
}

export function disableAdminModel(modelID: string): Promise<AdminModelView> {
  return adminClient.request<AdminModelView>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}/disable`,
    { method: "POST" }
  );
}

export function deprecateAdminModel(modelID: string): Promise<AdminModelView> {
  return adminClient.request<AdminModelView>(
    `/api/admin/v1/models/${encodeURIComponent(modelID)}/deprecate`,
    { method: "POST" }
  );
}

export function previewAdminModelCatalogSync(
  request: AdminModelCatalogSyncPreviewRequest
): Promise<AdminModelCatalogSyncPreview> {
  return adminClient.request<AdminModelCatalogSyncPreview>("/api/admin/v1/models/sync-preview", {
    method: "POST",
    body: request
  });
}
