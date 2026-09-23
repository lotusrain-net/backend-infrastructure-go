import type { PropsWithChildren } from "react";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useSecurityMutations } from "../security-api";

function response(status: number, data: unknown = {}) {
  return new Response(JSON.stringify({ code: status, msg: "test", data }), {
    status, headers: { "content-type": "application/json" },
  });
}
function setup() {
  const cache = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={cache}>{children}</QueryClientProvider>
  );
  return renderHook(() => useSecurityMutations(), { wrapper });
}
const operations = ["enroll", "confirm", "disable"] as const;
function invoke(mutations: ReturnType<typeof useSecurityMutations>, operation: typeof operations[number]) {
  return operation === "confirm"
    ? mutations.confirm.mutateAsync("123456")
    : mutations[operation].mutateAsync({ password: "correct-password", code: "123456" });
}

afterEach(() => vi.unstubAllGlobals());
describe("protected TOTP operations", () => {
  it.each(operations)("refreshes an expired session and retries %s", async (operation) => {
    const fetchMock = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(response(401))
      .mockResolvedValueOnce(response(200))
      .mockResolvedValueOnce(response(200, { ok: true }));
    vi.stubGlobal("fetch", fetchMock);
    const { result } = setup();
    await act(async () => { await invoke(result.current, operation); });
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      `/api/v1/users/me/security/totp/${operation}`,
      "/api/v1/auth/refresh",
      `/api/v1/users/me/security/totp/${operation}`,
    ]);
    expect(fetchMock.mock.calls[2][1]?.body).toEqual(fetchMock.mock.calls[0][1]?.body);
  });
  it.each(operations)("does not refresh or retry invalid proof for %s", async (operation) => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(response(422, { proof: "invalid" }));
    vi.stubGlobal("fetch", fetchMock);
    const { result } = setup();
    await act(async () => {
      await expect(invoke(result.current, operation)).rejects.toMatchObject({ status: 422 });
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
