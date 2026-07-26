import { describe, expect, it, vi } from "vitest";
import {
  getThemeState,
  hydrateThemeStore,
  initializeThemeStore,
  setColorMode,
  setTheme,
} from "@/stores/theme-store";

describe("theme persistence", () => {
  it("persists theme selection and applies DOM attributes", () => {
    setTheme("cyberpunk");
    setColorMode("dark");

    const saved = JSON.parse(window.localStorage.getItem("backend-infra-theme") ?? "{}");
    expect(saved).toEqual({
      state: { theme: "cyberpunk", colorMode: "dark" },
      version: 0,
    });
    expect(document.documentElement.dataset.theme).toBe("cyberpunk");
    expect(document.documentElement.dataset.colorMode).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("hydrates from storage and honors system mode", () => {
    window.localStorage.setItem(
      "backend-infra-theme",
      JSON.stringify({ state: { theme: "enterprise", colorMode: "system" }, version: 0 }),
    );
    vi.stubGlobal(
      "matchMedia",
      vi.fn().mockImplementation(() => ({
        matches: true,
        media: "(prefers-color-scheme: dark)",
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    );

    hydrateThemeStore();
    initializeThemeStore();

    expect(getThemeState().hydrated).toBe(true);
    expect(document.documentElement.dataset.theme).toBe("enterprise");
    expect(document.documentElement.dataset.colorMode).toBe("dark");
  });
});
