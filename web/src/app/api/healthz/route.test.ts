import { beforeEach, describe, expect, it, vi } from "vitest";

describe("healthz route", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.unstubAllEnvs();
  });

  it("falls back to localhost when API_PROXY_TARGET is an empty string", async () => {
    vi.stubEnv("API_PROXY_TARGET", "");
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ status: "ok" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { GET } = await import("@/app/api/healthz/route");
    const response = await GET();

    expect(fetchMock).toHaveBeenCalledWith("http://127.0.0.1:8080/health/ready", { cache: "no-store" });
    expect(response.status).toBe(200);
  });
});
