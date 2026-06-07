import type { AdminBFFComponents } from "@token-gateway/api-client";

import { requestAdmin, type AdminMutationOptions } from "../../shared/api/client";

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
  return requestAdmin<AdminCustomerAccountListResponse>("/api/admin/v1/customer-accounts");
}

export function getAdminCustomerAccount(
  customerAccountID: string
): Promise<AdminCustomerAccountDetail> {
  return requestAdmin<AdminCustomerAccountDetail>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}`
  );
}

export function createAdminCustomerAccount(
  request: AdminCustomerAccountCreateRequest,
  mutation: AdminMutationOptions
): Promise<AdminCustomerAccountDetail> {
  return requestAdmin<AdminCustomerAccountDetail>(
    "/api/admin/v1/customer-accounts",
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function disableAdminCustomerAccount(
  customerAccountID: string,
  mutation: AdminMutationOptions
): Promise<AdminCustomerAccountDetail> {
  return requestAdmin<AdminCustomerAccountDetail>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}/disable`,
    { method: "POST" },
    mutation
  );
}

export function enableAdminCustomerAccount(
  customerAccountID: string,
  mutation: AdminMutationOptions
): Promise<AdminCustomerAccountDetail> {
  return requestAdmin<AdminCustomerAccountDetail>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}/enable`,
    { method: "POST" },
    mutation
  );
}

export function adjustAdminCustomerCredits(
  customerAccountID: string,
  request: AdminCustomerCreditAdjustmentRequest,
  mutation: AdminMutationOptions
): Promise<AdminCustomerCreditAdjustmentResult> {
  return requestAdmin<AdminCustomerCreditAdjustmentResult>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}/manual-adjustment`,
    {
      method: "POST",
      body: request
    },
    mutation
  );
}

export function resetAdminCustomerPortalSessions(
  customerAccountID: string,
  apiKeyID: string | undefined,
  mutation: AdminMutationOptions
): Promise<AdminCustomerSessionResetResult> {
  return requestAdmin<AdminCustomerSessionResetResult>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(customerAccountID)}/reset-session`,
    {
      method: "POST",
      body: apiKeyID ? { api_key_id: apiKeyID } : {}
    },
    mutation
  );
}
