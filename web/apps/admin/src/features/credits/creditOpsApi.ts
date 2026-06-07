import type { AdminBFFComponents } from "@token-gateway/api-client";

import { requestAdmin } from "../../shared/api/client";

export type AdminCustomerCreditReport =
  AdminBFFComponents["schemas"]["AdminCustomerCreditReport"];
export type AdminCustomerReportExport =
  AdminBFFComponents["schemas"]["AdminCustomerReportExport"];

export function getAdminCustomerCreditReport(
  accountID: string
): Promise<AdminCustomerCreditReport> {
  return requestAdmin<AdminCustomerCreditReport>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(accountID)}/credit-report`
  );
}

export function exportAdminCustomerUsage(accountID: string): Promise<AdminCustomerReportExport> {
  return requestAdmin<AdminCustomerReportExport>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(accountID)}/usage/export`
  );
}

export function exportAdminCustomerLedger(accountID: string): Promise<AdminCustomerReportExport> {
  return requestAdmin<AdminCustomerReportExport>(
    `/api/admin/v1/customer-accounts/${encodeURIComponent(accountID)}/ledger/export`
  );
}
