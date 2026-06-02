import { createConsoleClient, type ConsoleRequestInit } from "@token-gateway/api-client";

export const portalClient = createConsoleClient();

export function requestPortal<TResponse>(
  path: string,
  init: ConsoleRequestInit = {},
  csrfToken?: string
): Promise<TResponse> {
  return createConsoleClient({ csrfToken }).request<TResponse>(path, init);
}
