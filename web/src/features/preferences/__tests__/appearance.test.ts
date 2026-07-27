import { describe, expect, it } from "vitest";
import {
  DEFAULT_ACCOUNT_PREFERENCES,
  contrastRatio,
  deriveAccentTokens,
  normalizeAccentColor,
} from "@/features/preferences/appearance";

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
