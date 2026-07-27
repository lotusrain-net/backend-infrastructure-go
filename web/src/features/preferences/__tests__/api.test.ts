import { describe, expect, it, vi } from "vitest";
import { getCurrentUserPreferences, putCurrentUserPreferences } from "@/features/preferences/api";
import type { AccountPreferences } from "@/types/api";

const preferences: AccountPreferences = {
  theme: "enterprise",
  color_mode: "system",
  accent_color: null,
  font_scale: "standard",
  radius_scale: "compact",
};

describe("account preferences API", () => {
  it("gets and fully replaces the current user's preferences", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 200, msg: "success", data: preferences }), { status: 200, headers: { "content-type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: 200, msg: "success", data: preferences }), { status: 200, headers: { "content-type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(getCurrentUserPreferences()).resolves.toEqual(preferences);
    await expect(putCurrentUserPreferences(preferences)).resolves.toEqual(preferences);

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/v1/users/me/preferences", expect.any(Object));
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/v1/users/me/preferences",
      expect.objectContaining({ method: "PUT", body: JSON.stringify(preferences) }),
    );
  });
});
