import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError, withQuery } from "./index";

afterEach(() => vi.restoreAllMocks());

describe("public ApiClient", () => {
  it("uses an injected base URL and unwraps the stable envelope", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ code: 200, msg: "ok", data: { id: "1" } }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    const client = new ApiClient({ baseURL: "https://api.example.test/", fetch: fetcher });

    await expect(client.request<{ id: string }>("/items/1")).resolves.toEqual({ id: "1" });
    expect(fetcher).toHaveBeenCalledWith("https://api.example.test/items/1", expect.any(Object));
  });

  it("shares refresh work and expires the session after one retry", async () => {
    const refresh = vi.fn().mockResolvedValue(undefined);
    const onSessionExpired = vi.fn();
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ code: 401, msg: "unauthorized", data: {} }), {
        status: 401,
        headers: { "content-type": "application/json" },
      }),
    );
    const client = new ApiClient({ fetch: fetcher, refresh, onSessionExpired });

    await expect(client.request("/private")).rejects.toBeInstanceOf(ApiError);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(onSessionExpired).toHaveBeenCalledWith("session_expired");
  });

  it("passes typed-array request bodies through without JSON encoding", async () => {
    const fetch = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.body).toBeInstanceOf(Uint8Array);
      expect(new Headers(init?.headers).has("content-type")).toBe(false);
      return new Response(JSON.stringify({ code: 200, msg: "success", data: null }), {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    });
    const client = new ApiClient({ fetch });

    await client.request("/upload", { method: "POST", body: new Uint8Array([1, 2, 3]) });
  });
});

describe("withQuery", () => {
  it("appends arrays and preserves an existing query", () => {
    expect(withQuery("/items?active=true", { tag: ["a", "b"], page: 2, empty: "" })).toBe(
      "/items?active=true&tag=a&tag=b&page=2",
    );
  });
});
