import { describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "@/lib/api/client";
import { queryClient } from "@/lib/query/client";
import { clearAuthState, getAuthState, setCurrentUser } from "@/stores/auth-store";
import type { UserProfile } from "@/types/api";

const currentUser: UserProfile = {
  id: "u1",
  email: "dev@example.com",
  username: "dev",
  display_name: "Developer",
  is_active: true,
  permissions: ["users:read"],
};

describe("ApiClient", () => {
  it("unwraps the contract envelope and retries one protected request after a successful refresh", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 401, msg: "invalid credentials", data: {} }), {
          status: 401,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 200, msg: "success", data: { access_token: "access", token_type: "Bearer", expires_in: 900 } }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 200, msg: "success", data: currentUser }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ApiClient({
      fetch: fetchMock,
      refresh: async () => {
        const response = await fetchMock("/api/v1/auth/refresh", {
          method: "POST",
          credentials: "include",
        });
        if (!response.ok) throw new Error("refresh failed");
      },
    });
    const result = await client.request<UserProfile>("/api/v1/users/me");

    expect(result).toEqual(currentUser);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/v1/auth/refresh",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/v1/users/me",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("exposes validation details from contract error envelopes", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 422, msg: "validation failed", data: { email: "is required" } }), {
          status: 422,
          headers: { "content-type": "application/json" },
        }),
      ),
    );

    const client = new ApiClient({ fetch: globalThis.fetch });

    await expect(client.request("/api/v1/auth/login", { method: "POST", body: JSON.stringify({}) })).rejects.toMatchObject({
      message: "validation failed",
      status: 422,
      code: 422,
      fieldErrors: { email: "is required" },
    });
  });

  it("clears auth state, clears cached queries, and redirects when refresh fails", async () => {
    const redirect = vi.fn();
    const clearSpy = vi.spyOn(queryClient, "clear");
    setCurrentUser(currentUser);

    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 401, msg: "invalid credentials", data: {} }), {
          status: 401,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 401, msg: "invalid credentials", data: {} }), {
          status: 401,
          headers: { "content-type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ApiClient({
      fetch: fetchMock,
      messages: { sessionExpired: "登录状态已失效，请重新登录" },
      refresh: async () => {
        const response = await fetchMock("/api/v1/auth/refresh", { method: "POST", credentials: "include" });
        if (!response.ok) throw new Error("refresh failed");
      },
      onSessionExpired: () => {
        clearAuthState();
        queryClient.clear();
        redirect("session_expired");
      },
    });

    await expect(client.request("/api/v1/users/me")).rejects.toEqual(
      expect.objectContaining({
        message: "登录状态已失效，请重新登录",
        status: 401,
      }),
    );
    expect(getAuthState().user).toBeNull();
    expect(clearSpy).toHaveBeenCalledTimes(1);
    expect(redirect).toHaveBeenCalledWith("session_expired");
  });

  it("shares a single refresh request across concurrent protected 401 responses", async () => {
    let resolveRefresh: (value: Response) => void = () => {
      throw new Error("refresh resolver was not initialized");
    };
    const refreshPromise = new Promise<Response>((resolve) => {
      resolveRefresh = resolve;
    });

    const fetchMock = vi.fn<typeof fetch>((input) => {
      if (input === "/api/v1/auth/refresh") {
        return refreshPromise;
      }

      if (input === "/api/v1/users/me" || input === "/api/v1/roles") {
        const callCount = fetchMock.mock.calls.filter(([url]) => url === input).length;
        if (callCount === 1) {
          return Promise.resolve(
            new Response(JSON.stringify({ code: 401, msg: "invalid credentials", data: {} }), {
              status: 401,
              headers: { "content-type": "application/json" },
            }),
          );
        }

        return Promise.resolve(
          new Response(
            JSON.stringify({
              code: 200,
              msg: "success",
              data:
                input === "/api/v1/users/me"
                  ? currentUser
                  : [{ id: "r1", name: "admin", description: "Administrator" }],
            }),
            {
              status: 200,
              headers: { "content-type": "application/json" },
            },
          ),
        );
      }

      return Promise.reject(new Error(`unexpected request: ${String(input)}`));
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = new ApiClient({
      fetch: fetchMock,
      refresh: async () => {
        const response = await fetchMock("/api/v1/auth/refresh", { method: "POST", credentials: "include" });
        if (!response.ok) throw new Error("refresh failed");
      },
    });
    const pendingUser = client.request<UserProfile>("/api/v1/users/me");
    const pendingRoles = client.request<Array<{ id: string; name: string; description: string }>>("/api/v1/roles");

    resolveRefresh(
      new Response(JSON.stringify({ code: 200, msg: "success", data: { access_token: "access", token_type: "Bearer", expires_in: 900 } }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    const [user, roles] = await Promise.all([pendingUser, pendingRoles]);

    expect(user).toEqual(currentUser);
    expect(roles).toEqual([{ id: "r1", name: "admin", description: "Administrator" }]);
    expect(fetchMock.mock.calls.filter(([url]) => url === "/api/v1/auth/refresh")).toHaveLength(1);
    expect(fetchMock.mock.calls.filter(([url]) => url === "/api/v1/users/me")).toHaveLength(2);
    expect(fetchMock.mock.calls.filter(([url]) => url === "/api/v1/roles")).toHaveLength(2);
  });

  it("clears auth state and redirects when the retried request is still unauthorized", async () => {
    const redirect = vi.fn();
    const clearSpy = vi.spyOn(queryClient, "clear");
    setCurrentUser(currentUser);

    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 401, msg: "invalid credentials", data: {} }), {
          status: 401,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 200, msg: "success", data: { access_token: "access", token_type: "Bearer", expires_in: 900 } }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 401, msg: "invalid credentials", data: {} }), {
          status: 401,
          headers: { "content-type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const client = new ApiClient({
      fetch: fetchMock,
      messages: { sessionExpired: "登录状态已失效，请重新登录" },
      refresh: async () => {
        const response = await fetchMock("/api/v1/auth/refresh", { method: "POST", credentials: "include" });
        if (!response.ok) throw new Error("refresh failed");
      },
      onSessionExpired: () => {
        clearAuthState();
        queryClient.clear();
        redirect("session_expired");
      },
    });

    await expect(client.request("/api/v1/users/me")).rejects.toEqual(
      expect.objectContaining({
        message: "登录状态已失效，请重新登录",
        status: 401,
      }),
    );
    expect(getAuthState().user).toBeNull();
    expect(clearSpy).toHaveBeenCalledTimes(1);
    expect(redirect).toHaveBeenCalledWith("session_expired");
  });
});
