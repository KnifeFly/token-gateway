import { ConsoleAPIError } from "../errors";

export async function errorFromResponse(response: Response): Promise<ConsoleAPIError> {
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
