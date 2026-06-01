export interface ConsoleClientOptions {
  baseURL?: string;
  csrfToken?: string;
  fetch?: typeof fetch;
}

export interface ConsoleRequestInit extends Omit<RequestInit, "body" | "credentials"> {
  body?: BodyInit | Record<string, unknown> | null;
  credentials?: RequestCredentials;
}

export interface ConsoleClient {
  readonly baseURL: string;
  request<TResponse>(path: string, init?: ConsoleRequestInit): Promise<TResponse>;
}

export class ConsoleAPIError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestID?: string;

  constructor(status: number, code: string, message: string, requestID?: string) {
    super(message);
    this.name = "ConsoleAPIError";
    this.status = status;
    this.code = code;
    this.requestID = requestID;
  }
}

export function defaultConsoleBaseURL(): string {
  if (typeof window === "undefined") {
    return "http://localhost:9505";
  }
  return window.location.origin;
}

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

function encodeBody(body: ConsoleRequestInit["body"], headers: Headers): BodyInit | null | undefined {
  if (body == null) {
    return body;
  }
  if (typeof body === "string" || body instanceof FormData || body instanceof URLSearchParams) {
    return body;
  }
  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  return JSON.stringify(body);
}

async function errorFromResponse(response: Response): Promise<ConsoleAPIError> {
  const fallbackRequestID = response.headers.get("X-Request-ID") ?? undefined;
  try {
    const payload = (await response.json()) as {
      error?: { code?: string; message?: string; request_id?: string };
    };
    const error = payload.error ?? {};
    return new ConsoleAPIError(
      response.status,
      error.code ?? "console_api_error",
      error.message ?? response.statusText,
      error.request_id ?? fallbackRequestID
    );
  } catch {
    return new ConsoleAPIError(
      response.status,
      "console_api_error",
      response.statusText,
      fallbackRequestID
    );
  }
}

function joinURL(baseURL: string, path: string): string {
  if (/^https?:\/\//.test(path)) {
    return path;
  }
  return `${baseURL}/${path.replace(/^\/+/, "")}`;
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
