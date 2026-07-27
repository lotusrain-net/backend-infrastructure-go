import { describe, expect, it } from "vitest";
import {
  applyRemotePreferences,
  cacheKeyForUser,
  createAppearancePreferences,
  getThemeState,
  initializeThemeStore,
  markPreferencesSyncFailed,
  previewPreferences,
  switchThemeUser,
} from "@/stores/theme-store";

describe("account appearance preferences", () => {
  it("keeps local preview caches isolated by user ID", () => {
    const first = createAppearancePreferences({ theme: "cyberpunk", color_mode: "dark", font_scale: "large" });
    const second = createAppearancePreferences({ theme: "enterprise", color_mode: "light", radius_scale: "square" });

    switchThemeUser("user-1");
    previewPreferences("user-1", first);
    switchThemeUser("user-2");
    previewPreferences("user-2", second);
    switchThemeUser("user-1");

    expect(JSON.parse(window.localStorage.getItem(cacheKeyForUser("user-1")) ?? "{}")).toEqual(first);
    expect(getThemeState().preferences).toEqual(first);
    expect(document.documentElement.dataset.theme).toBe("cyberpunk");
    expect(document.documentElement.dataset.fontScale).toBe("large");
  });

  it("applies a successful server response over a local cached preview", () => {
    const cached = createAppearancePreferences({ theme: "cyberpunk", color_mode: "dark" });
    const server = createAppearancePreferences({ theme: "enterprise", color_mode: "light", accent_color: "#1A2B3C" });

    switchThemeUser("user-1");
    previewPreferences("user-1", cached);
    applyRemotePreferences("user-1", server);

    expect(getThemeState().preferences).toEqual(server);
    expect(getThemeState().syncStatus).toBe("synced");
    expect(document.documentElement.dataset.colorMode).toBe("light");
    expect(document.documentElement.style.getPropertyValue("--accent-primary")).toBe("#1A2B3C");
  });

  it("retains the local preview and records a retryable failure", () => {
    const preview = createAppearancePreferences({ accent_color: "#1A2B3C" });

    switchThemeUser("user-1");
    previewPreferences("user-1", preview);
    markPreferencesSyncFailed("user-1", new Error("network unavailable"));
    initializeThemeStore();

    expect(getThemeState().preferences).toEqual(preview);
    expect(getThemeState().syncStatus).toBe("error");
    expect(getThemeState().syncError).toBe("network unavailable");
  });
});
