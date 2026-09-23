import { describe, expect, it } from "vitest";
import {
  DEFAULT_ACCOUNT_PREFERENCES,
  contrastRatio,
  deriveAccentTokens,
  deriveThemeTokens,
  normalizeAccentColor,
} from "@lotusrain-net/backend-infrastructure-web/theme";

describe("appearance color utilities", () => {
  it("normalizes valid #RRGGBB input and rejects other values", () => {
    expect(normalizeAccentColor("#1a2B3c")).toBe("#1A2B3C");
    expect(normalizeAccentColor("1A2B3C")).toBeNull();
    expect(normalizeAccentColor("#123")).toBeNull();
  });

  it("derives accessible button text and focus tokens for arbitrary accents", () => {
    const tokens = deriveAccentTokens("#777777", "#FFFFFF", "light");

    expect(contrastRatio(tokens.primary, tokens.contrast)).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(tokens.hover, tokens.contrast)).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(tokens.focus, "#FFFFFF")).toBeGreaterThanOrEqual(3);
    expect(tokens.subtle).not.toBe(tokens.primary);
  });

  it("derives a complete, accessible palette from a seed instead of only changing the primary action", () => {
    const tokens = deriveThemeTokens("#FFF200", "enterprise", "light");

    expect(tokens.canvas).not.toBe(tokens.surface);
    expect(tokens.surface).not.toBe(tokens.popover);
    expect(tokens.border).not.toBe(tokens.input);
    expect(tokens.sidebar).not.toBe(tokens.canvas);
    expect(tokens.chartSecondary).not.toBe(tokens.primary);
    expect(contrastRatio(tokens.primary, tokens.primaryForeground)).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(tokens.primary, tokens.primarySubtle)).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(tokens.focus, tokens.canvas)).toBeGreaterThanOrEqual(3);
  });

  it("keeps high-contrast foregrounds for white, yellow, and saturated seeds in both themes and color modes", () => {
    for (const theme of ["enterprise", "cyberpunk"] as const) {
      for (const mode of ["light", "dark"] as const) {
        for (const seed of ["#FFFFFF", "#FFF200", "#FF00AA", "#12002E"]) {
          const tokens = deriveThemeTokens(seed, theme, mode);
          expect(contrastRatio(tokens.primary, tokens.primaryForeground)).toBeGreaterThanOrEqual(4.5);
          expect(contrastRatio(tokens.primary, tokens.primarySubtle)).toBeGreaterThanOrEqual(4.5);
          expect(contrastRatio(tokens.focus, tokens.canvas)).toBeGreaterThanOrEqual(3);
        }
      }
    }
  });

  it("uses the agreed account defaults", () => {
    expect(DEFAULT_ACCOUNT_PREFERENCES).toEqual({
      theme: "enterprise",
      color_mode: "system",
      accent_color: null,
      font_scale: "standard",
      radius_scale: "compact",
    });
  });
});
