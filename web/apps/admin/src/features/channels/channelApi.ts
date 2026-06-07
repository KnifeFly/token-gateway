import type { AdminBFFComponents } from "@token-gateway/api-client";

import { requestAdmin, type AdminMutationOptions } from "../../shared/api/client";

export type AdminChannelView = AdminBFFComponents["schemas"]["AdminChannelView"];
export type AdminChannelListResponse = AdminBFFComponents["schemas"]["AdminChannelListResponse"];
export type AdminChannelUpsertRequest = AdminBFFComponents["schemas"]["AdminChannelUpsertRequest"];
export type AdminChannelSyncRequest = AdminBFFComponents["schemas"]["AdminChannelSyncRequest"];
export type AdminChannelSyncPreview = AdminBFFComponents["schemas"]["AdminChannelSyncPreview"];
export type AdminChannelSyncApplyResult =
  AdminBFFComponents["schemas"]["AdminChannelSyncApplyResult"];
export type AdminChannelTestResult = AdminBFFComponents["schemas"]["AdminChannelTestResult"];
export type AdminChannelHealthEvent =
  AdminBFFComponents["schemas"]["AdminChannelHealthEvent"];

export function listAdminChannels(): Promise<AdminChannelListResponse> {
  return requestAdmin<AdminChannelListResponse>("/api/admin/v1/channels");
}

export function upsertAdminChannel(
  request: AdminChannelUpsertRequest,
  mutation: AdminMutationOptions
): Promise<AdminChannelView> {
  return requestAdmin<AdminChannelView>(
    "/api/admin/v1/channels",
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function patchAdminChannel(
  channelID: string,
  request: AdminChannelUpsertRequest,
  mutation: AdminMutationOptions
): Promise<AdminChannelView> {
  return requestAdmin<AdminChannelView>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}`,
    {
      method: "PATCH",
      body: request
    },
    mutation
  );
}

export function rotateAdminChannelCredential(
  channelID: string,
  apiKey: string,
  mutation: AdminMutationOptions
): Promise<AdminChannelView> {
  return requestAdmin<AdminChannelView>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/rotate-credential`,
    {
      method: "POST",
      body: { api_key: apiKey }
    },
    mutation
  );
}

export function testAdminChannel(
  channelID: string,
  mutation: AdminMutationOptions
): Promise<AdminChannelTestResult> {
  return requestAdmin<AdminChannelTestResult>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/test`,
    { method: "POST" },
    mutation
  );
}

export function previewAdminChannelSync(
  channelID: string,
  request: AdminChannelSyncRequest,
  mutation: AdminMutationOptions
): Promise<AdminChannelSyncPreview> {
  return requestAdmin<AdminChannelSyncPreview>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/sync-preview`,
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function applyAdminChannelSync(
  channelID: string,
  request: AdminChannelSyncRequest,
  mutation: AdminMutationOptions
): Promise<AdminChannelSyncApplyResult> {
  return requestAdmin<AdminChannelSyncApplyResult>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/sync-apply`,
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function listAdminChannelHealthEvents(
  channelID: string
): Promise<{ data: AdminChannelHealthEvent[] }> {
  return requestAdmin<{ data: AdminChannelHealthEvent[] }>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/health-events`
  );
}
