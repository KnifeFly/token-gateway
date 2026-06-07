import type { AdminBFFComponents } from "@token-gateway/api-client";

import { requestAdmin, type AdminMutationOptions } from "../../shared/api/client";

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
  return requestAdmin<AdminAPIKeyListResponse>("/api/admin/v1/api-keys");
}

export function createAdminAPIKey(
  request: AdminAPIKeyCreateRequest,
  mutation: AdminMutationOptions
): Promise<AdminAPIKeyCreateResponse> {
  return requestAdmin<AdminAPIKeyCreateResponse>(
    "/api/admin/v1/api-keys",
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function updateAdminAPIKey(
  keyID: string,
  request: AdminAPIKeyUpdateRequest,
  mutation: AdminMutationOptions
): Promise<AdminAPIKeyView> {
  return requestAdmin<AdminAPIKeyView>(
    `/api/admin/v1/api-keys/${encodeURIComponent(keyID)}/update`,
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function enableAdminAPIKey(
  keyID: string,
  mutation: AdminMutationOptions
): Promise<AdminAPIKeyView> {
  return requestAdmin<AdminAPIKeyView>(
    `/api/admin/v1/api-keys/${encodeURIComponent(keyID)}/enable`,
    { method: "POST" },
    mutation
  );
}

export function disableAdminAPIKey(
  keyID: string,
  mutation: AdminMutationOptions
): Promise<AdminAPIKeyView> {
  return requestAdmin<AdminAPIKeyView>(
    `/api/admin/v1/api-keys/${encodeURIComponent(keyID)}/disable`,
    { method: "POST" },
    mutation
  );
}

export function rotateAdminAPIKey(
  keyID: string,
  mutation: AdminMutationOptions
): Promise<AdminAPIKeyRotateResponse> {
  return requestAdmin<AdminAPIKeyRotateResponse>(
    `/api/admin/v1/api-keys/${encodeURIComponent(keyID)}/rotate`,
    { method: "POST", body: {} },
    mutation
  );
}
