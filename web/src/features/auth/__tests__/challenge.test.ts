import { describe, it, expect, vi, afterEach } from "vitest";
import { login } from "@/features/auth/api";
import { queryClient } from "@/lib/query/client";
import { useAuthStore } from "@/stores/auth-store";
afterEach(() => {
  vi.unstubAllGlobals();
  queryClient.clear();
});
describe("TOTP challenge boundary", () => {
  it("does not load current user or populate auth cache before verification", async () => {
    useAuthStore.setState({ user: null });
    const fetcher = vi
      .fn()
      .mockResolvedValue(
        new Response(
          JSON.stringify({
            code: 202,
            msg: "success",
            data: {
              status: "totp_required",
              challenge_id: "challenge",
              expires_in: 300,
            },
          }),
          { status: 202, headers: { "Content-Type": "application/json" } },
        ),
      );
    vi.stubGlobal("fetch", fetcher);
    const result = await login({
      email: "a@example.com",
      password: "password",
    });
    expect(result).toEqual({
      status: "totp_required",
      challenge_id: "challenge",
      expires_in: 300,
    });
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(useAuthStore.getState().user).toBeNull();
    expect(queryClient.getQueryData(["auth", "current-user"])).toBeUndefined();
  });
});
