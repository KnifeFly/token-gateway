import type { AdminBFFComponents } from "@token-gateway/api-client";

import { adminClient } from "../../shared/api/client";

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
  return adminClient.request<AdminChannelListResponse>("/api/admin/v1/channels");
}

export function upsertAdminChannel(request: AdminChannelUpsertRequest): Promise<AdminChannelView> {
  return adminClient.request<AdminChannelView>("/api/admin/v1/channels", {
    method: "POST",
    body: request
  });
}

export function patchAdminChannel(
  channelID: string,
  request: AdminChannelUpsertRequest
): Promise<AdminChannelView> {
  return adminClient.request<AdminChannelView>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}`,
    {
      method: "PATCH",
      body: request
    }
  );
}

export function rotateAdminChannelCredential(
  channelID: string,
  apiKey: string
): Promise<AdminChannelView> {
  return adminClient.request<AdminChannelView>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/rotate-credential`,
    {
      method: "POST",
      body: { api_key: apiKey }
    }
  );
}

export function testAdminChannel(channelID: string): Promise<AdminChannelTestResult> {
  return adminClient.request<AdminChannelTestResult>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/test`,
    { method: "POST" }
  );
}

export function previewAdminChannelSync(
  channelID: string,
  request: AdminChannelSyncRequest
): Promise<AdminChannelSyncPreview> {
  return adminClient.request<AdminChannelSyncPreview>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/sync-preview`,
    {
      method: "POST",
      body: request
    }
  );
}

export function applyAdminChannelSync(
  channelID: string,
  request: AdminChannelSyncRequest
): Promise<AdminChannelSyncApplyResult> {
  return adminClient.request<AdminChannelSyncApplyResult>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/sync-apply`,
    {
      method: "POST",
      body: request
    }
  );
}

export function listAdminChannelHealthEvents(
  channelID: string
): Promise<{ data: AdminChannelHealthEvent[] }> {
  return adminClient.request<{ data: AdminChannelHealthEvent[] }>(
    `/api/admin/v1/channels/${encodeURIComponent(channelID)}/health-events`
  );
}
