import { describe, expect, it } from "vitest";
import { contrastRatio, createAppearancePreferences, deriveThemeTokens, parseAppearancePreferences } from "./index";

describe("public theme helpers", () => {
  it("normalizes and validates appearance preferences", () => {
    const preferences = createAppearancePreferences({ accent_color: "#1a2b3c" });
    expect(preferences.accent_color).toBe("#1A2B3C");
    expect(parseAppearancePreferences(preferences)).toEqual(preferences);
  });

  it("derives accessible primary actions", () => {
    const tokens = deriveThemeTokens("#FFF200", "enterprise", "light");
    expect(contrastRatio(tokens.primary, tokens.primaryForeground)).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(tokens.primaryHover, tokens.primaryForeground)).toBeGreaterThanOrEqual(4.5);
  });
});
