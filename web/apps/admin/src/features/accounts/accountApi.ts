import type { AdminBFFComponents } from "@token-gateway/api-client";

import { adminClient } from "../../shared/api/client";

export type AdminCustomerAccountView =
  AdminBFFComponents["schemas"]["AdminCustomerAccountView"];
export type AdminCustomerAccountDetail =
  AdminBFFComponents["schemas"]["AdminCustomerAccountDetail"];
export type AdminCustomerAccountListResponse =
  AdminBFFComponents["schemas"]["AdminCustomerAccountListResponse"];
export type AdminCustomerAccountCreateRequest =
  AdminBFFComponents["schemas"]["AdminCustomerAccountCreateRequest"];
export type AdminCustomerCreditAdjustmentRequest =
  AdminBFFComponents["schemas"]["AdminCustomerCreditAdjustmentRequest"];
export type AdminCustomerCreditAdjustmentResult =
  AdminBFFComponents["schemas"]["AdminCustomerCreditAdjustmentResult"];
export type AdminCustomerSessionResetResult =
  AdminBFFComponents["schemas"]["AdminCustomerSessionResetResult"];

export function listAdminCustomerAccounts(): Promise<AdminCustomerAccountListResponse> {
  return adminClient.request<AdminCustomerAccountListResponse>("/api/admin/v1/customer-accounts");
}

export function getAdminCustomerAccount(
  customerAccountID: string
): Promise<AdminCustomerAccountDetail> {
  return adminClient.request<AdminCustomerAccountDetail>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}`
  );
}

export function createAdminCustomerAccount(
  request: AdminCustomerAccountCreateRequest
): Promise<AdminCustomerAccountDetail> {
  return adminClient.request<AdminCustomerAccountDetail>("/api/admin/v1/customer-accounts", {
    method: "POST",
    body: request
  });
}

export function disableAdminCustomerAccount(
  customerAccountID: string
): Promise<AdminCustomerAccountDetail> {
  return adminClient.request<AdminCustomerAccountDetail>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}/disable`,
    { method: "POST" }
  );
}

export function enableAdminCustomerAccount(
  customerAccountID: string
): Promise<AdminCustomerAccountDetail> {
  return adminClient.request<AdminCustomerAccountDetail>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}/enable`,
    { method: "POST" }
  );
}

export function adjustAdminCustomerCredits(
  customerAccountID: string,
  request: AdminCustomerCreditAdjustmentRequest
): Promise<AdminCustomerCreditAdjustmentResult> {
  return adminClient.request<AdminCustomerCreditAdjustmentResult>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}/manual-adjustment`,
    {
      method: "POST",
      body: request
    }
  );
}

export function resetAdminCustomerPortalSessions(
  customerAccountID: string,
  apiKeyID?: string
): Promise<AdminCustomerSessionResetResult> {
  return adminClient.request<AdminCustomerSessionResetResult>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}/reset-session`,
    {
      method: "POST",
      body: apiKeyID ? { api_key_id: apiKeyID } : {}
    }
  );
}
