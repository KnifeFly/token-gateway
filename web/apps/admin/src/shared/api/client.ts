import { createConsoleClient } from "@token-gateway/api-client";
import type { ConsoleRequestInit } from "@token-gateway/api-client";

export const adminClient = createConsoleClient();

export interface AdminMutationOptions {
  csrfToken?: string;
  reason?: string;
  idempotencyKey?: string;
}

function randomID(prefix: string): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return `${prefix}_${crypto.randomUUID()}`;
  }
  return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2)}`;
}

export function requestAdmin<TResponse>(
  path: string,
  init: ConsoleRequestInit = {},
  mutation: AdminMutationOptions = {}
): Promise<TResponse> {
  const headers = new Headers(init.headers);
  if (mutation.csrfToken && !headers.has("X-CSRF-Token")) {
    headers.set("X-CSRF-Token", mutation.csrfToken);
  }
  if (mutation.reason && !headers.has("X-Reason")) {
    headers.set("X-Reason", mutation.reason);
  }
  if ((init.method ?? "GET").toUpperCase() !== "GET" && !headers.has("Idempotency-Key")) {
    headers.set("Idempotency-Key", mutation.idempotencyKey ?? randomID("admin_mutation"));
  }

  return adminClient.request<TResponse>(path, { ...init, headers });
}
