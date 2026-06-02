import type { AdminBFFComponents } from "@token-gateway/api-client";

import { adminClient } from "../../shared/api/client";

export type AdminCustomerCreditReport =
  AdminBFFComponents["schemas"]["AdminCustomerCreditReport"];
export type AdminCustomerReportExport =
  AdminBFFComponents["schemas"]["AdminCustomerReportExport"];

export function getAdminCustomerCreditReport(
  accountID: string
): Promise<AdminCustomerCreditReport> {
  return adminClient.request<AdminCustomerCreditReport>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(accountID)}/credit-report`
  );
}

export function exportAdminCustomerUsage(accountID: string): Promise<AdminCustomerReportExport> {
  return adminClient.request<AdminCustomerReportExport>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(accountID)}/usage/export`
  );
}

export function exportAdminCustomerLedger(accountID: string): Promise<AdminCustomerReportExport> {
  return adminClient.request<AdminCustomerReportExport>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(accountID)}/ledger/export`
  );
}
