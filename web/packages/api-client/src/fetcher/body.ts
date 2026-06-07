import type { ConsoleRequestInit } from "./types";

export function encodeBody(
  body: ConsoleRequestInit["body"],
  headers: Headers
): BodyInit | null | undefined {
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
