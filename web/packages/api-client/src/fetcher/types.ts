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
