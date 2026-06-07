import { describe, expect, it } from "vitest";

import { createConsoleClient } from "./client";

describe("createConsoleClient", () => {
  it("uses include credentials and forwards csrf headers", async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = [];
    const client = createConsoleClient({
      baseURL: "http://localhost:9505/",
      csrfToken: "csrf_test",
      fetch: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ ok: true }), {
          status: 200,
          headers: { "Content-Type": "application/json" }
        });
      }
    });

    const response = await client.request<{ ok: boolean }>("/api/portal/v1/auth/me");

    expect(response.ok).toBe(true);
    expect(calls[0]?.input).toBe("http://localhost:9505/api/portal/v1/auth/me");
    expect(calls[0]?.init?.credentials).toBe("include");
    expect(new Headers(calls[0]?.init?.headers).get("X-CSRF-Token")).toBe("csrf_test");
  });

  it("normalizes console error envelopes", async () => {
    const client = createConsoleClient({
      fetch: async () =>
        new Response(
          JSON.stringify({
            error: {
              code: "not_implemented",
              message: "reserved",
              request_id: "req_test"
            }
          }),
          { status: 501, headers: { "Content-Type": "application/json" } }
        )
    });

    await expect(client.request("/api/admin/v1/auth/me")).rejects.toMatchObject({
      status: 501,
      code: "not_implemented",
      message: "reserved",
      requestID: "req_test"
    });
  });
});
