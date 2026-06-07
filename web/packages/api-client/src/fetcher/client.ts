import { encodeBody } from "./body";
import { errorFromResponse } from "./response";
import type { ConsoleClient, ConsoleClientOptions, ConsoleRequestInit } from "./types";
import { defaultConsoleBaseURL, joinURL, trimTrailingSlash } from "./url";

export function createConsoleClient(options: ConsoleClientOptions = {}): ConsoleClient {
  const baseURL = trimTrailingSlash(options.baseURL ?? defaultConsoleBaseURL());
  const fetchImpl = options.fetch ?? fetch;

  return {
    baseURL,
    request<TResponse>(path: string, init: ConsoleRequestInit = {}) {
      return requestJSON<TResponse>(baseURL, fetchImpl, path, init, options.csrfToken);
    }
  };
}

async function requestJSON<TResponse>(
  baseURL: string,
  fetchImpl: typeof fetch,
  path: string,
  init: ConsoleRequestInit,
  csrfToken?: string
): Promise<TResponse> {
  const headers = new Headers(init.headers);
  const body = encodeBody(init.body, headers);
  if (csrfToken && !headers.has("X-CSRF-Token")) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const response = await fetchImpl(joinURL(baseURL, path), {
    ...init,
    body,
    credentials: init.credentials ?? "include",
    headers
  });
  if (!response.ok) {
    throw await errorFromResponse(response);
  }
  if (response.status === 204) {
    return undefined as TResponse;
  }
  return (await response.json()) as TResponse;
}
