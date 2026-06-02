import type { AdminBFFComponents } from "@token-gateway/api-client";

import { adminClient } from "../../shared/api/client";

export type AdminAPIKeyView = AdminBFFComponents["schemas"]["AdminAPIKeyView"];
export type AdminAPIKeyListResponse =
  AdminBFFComponents["schemas"]["AdminAPIKeyListResponse"];
export type AdminAPIKeyCreateRequest =
  AdminBFFComponents["schemas"]["AdminAPIKeyCreateRequest"];
export type AdminAPIKeyCreateResponse =
  AdminBFFComponents["schemas"]["AdminAPIKeyCreateResponse"];
export type AdminAPIKeyUpdateRequest =
  AdminBFFComponents["schemas"]["AdminAPIKeyUpdateRequest"];
export type AdminAPIKeyRotateResponse =
  AdminBFFComponents["schemas"]["AdminAPIKeyRotateResponse"];

export function listAdminAPIKeys(): Promise<AdminAPIKeyListResponse> {
  return adminClient.request<AdminAPIKeyListResponse>("/api/admin/v1/api-keys");
}

export function createAdminAPIKey(
  request: AdminAPIKeyCreateRequest
): Promise<AdminAPIKeyCreateResponse> {
  return adminClient.request<AdminAPIKeyCreateResponse>("/api/admin/v1/api-keys", {
    method: "POST",
    body: request
  });
}

export function updateAdminAPIKey(
  keyID: string,
  request: AdminAPIKeyUpdateRequest
): Promise<AdminAPIKeyView> {
  return adminClient.request<AdminAPIKeyView>(
    `/api/admin/v1/api-keys/${encodeURIComponent(keyID)}/update`,
    {
      method: "POST",
      body: request
    }
  );
}

export function enableAdminAPIKey(keyID: string): Promise<AdminAPIKeyView> {
  return adminClient.request<AdminAPIKeyView>(
    `/api/admin/v1/api-keys/${encodeURIComponent(keyID)}/enable`,
    { method: "POST" }
  );
}

export function disableAdminAPIKey(keyID: string): Promise<AdminAPIKeyView> {
  return adminClient.request<AdminAPIKeyView>(
    `/api/admin/v1/api-keys/${encodeURIComponent(keyID)}/disable`,
    { method: "POST" }
  );
}

export function rotateAdminAPIKey(keyID: string): Promise<AdminAPIKeyRotateResponse> {
  return adminClient.request<AdminAPIKeyRotateResponse>(
    `/api/admin/v1/api-keys/${encodeURIComponent(keyID)}/rotate`,
    { method: "POST", body: {} }
  );
}
