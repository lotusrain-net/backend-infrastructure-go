import type { AccountPreferences, ColorMode, ThemeName } from "@/types/api";

export const DEFAULT_ACCOUNT_PREFERENCES: AccountPreferences = {
  theme: "enterprise",
  color_mode: "system",
  accent_color: null,
  font_scale: "standard",
  radius_scale: "compact",
};

export type ResolvedColorMode = Exclude<ColorMode, "system">;

export interface AccentTokens {
  primary: string;
  hover: string;
  subtle: string;
  contrast: string;
  focus: string;
}

interface RGB {
  red: number;
  green: number;
  blue: number;
}

const hexPattern = /^#[0-9a-f]{6}$/i;

export function normalizeAccentColor(value: unknown): string | null {
  if (typeof value !== "string" || !hexPattern.test(value)) {
    return null;
  }

  return value.toUpperCase();
}

export function createAppearancePreferences(
  partial: Partial<AccountPreferences> = {},
): AccountPreferences {
  return {
    ...DEFAULT_ACCOUNT_PREFERENCES,
    ...partial,
    accent_color:
      partial.accent_color === undefined ? DEFAULT_ACCOUNT_PREFERENCES.accent_color : normalizeAccentColor(partial.accent_color),
  };
}

export function parseAppearancePreferences(value: unknown): AccountPreferences | null {
  if (!isRecord(value)) {
    return null;
  }

  const accent = value.accent_color === null ? null : normalizeAccentColor(value.accent_color);
  if (
    !isTheme(value.theme) ||
    !isColorMode(value.color_mode) ||
    (value.accent_color !== null && accent === null) ||
    !isFontScale(value.font_scale) ||
    !isRadiusScale(value.radius_scale)
  ) {
    return null;
  }

  return {
    theme: value.theme,
    color_mode: value.color_mode,
    accent_color: accent,
    font_scale: value.font_scale,
    radius_scale: value.radius_scale,
  };
}

export function contrastRatio(first: string, second: string): number {
  const firstRGB = hexToRGB(first);
  const secondRGB = hexToRGB(second);
  const firstLuminance = relativeLuminance(firstRGB);
  const secondLuminance = relativeLuminance(secondRGB);
  const lighter = Math.max(firstLuminance, secondLuminance);
  const darker = Math.min(firstLuminance, secondLuminance);

  return (lighter + 0.05) / (darker + 0.05);
}

export function deriveAccentTokens(
  accent: string,
  surface: string,
  colorMode: ResolvedColorMode,
): AccentTokens {
  const primary = normalizeAccentColor(accent);
  const normalizedSurface = normalizeAccentColor(surface);
  if (!primary || !normalizedSurface) {
    throw new Error("Accent and surface colors must use #RRGGBB.");
  }

  const contrast = highestContrastForeground(primary);
  return {
    primary,
    // Hover must retain the same 4.5:1 foreground choice as the base button.
    hover: mixHex(primary, contrast === "#000000" ? "#FFFFFF" : "#000000", 0.14),
    subtle: mixHex(primary, normalizedSurface, colorMode === "dark" ? 0.28 : 0.14),
    contrast,
    focus: ensureContrast(primary, normalizedSurface, 3),
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isTheme(value: unknown): value is ThemeName {
  return value === "enterprise" || value === "cyberpunk";
}

function isColorMode(value: unknown): value is ColorMode {
  return value === "light" || value === "dark" || value === "system";
}

function isFontScale(value: unknown): value is AccountPreferences["font_scale"] {
  return value === "small" || value === "standard" || value === "large";
}

function isRadiusScale(value: unknown): value is AccountPreferences["radius_scale"] {
  return value === "square" || value === "compact" || value === "rounded";
}

function hexToRGB(value: string): RGB {
  const normalized = normalizeAccentColor(value);
  if (!normalized) {
    throw new Error("Color must use #RRGGBB.");
  }

  return {
    red: Number.parseInt(normalized.slice(1, 3), 16),
    green: Number.parseInt(normalized.slice(3, 5), 16),
    blue: Number.parseInt(normalized.slice(5, 7), 16),
  };
}

function rgbToHex({ red, green, blue }: RGB): string {
  return `#${[red, green, blue]
    .map((channel) => Math.max(0, Math.min(255, Math.round(channel))).toString(16).padStart(2, "0"))
    .join("")
    .toUpperCase()}`;
}

function relativeLuminance({ red, green, blue }: RGB): number {
  const [r, g, b] = [red, green, blue].map((channel) => {
    const value = channel / 255;
    return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });

  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function mixHex(source: string, target: string, targetWeight: number): string {
  const first = hexToRGB(source);
  const second = hexToRGB(target);
  const sourceWeight = 1 - targetWeight;

  return rgbToHex({
    red: first.red * sourceWeight + second.red * targetWeight,
    green: first.green * sourceWeight + second.green * targetWeight,
    blue: first.blue * sourceWeight + second.blue * targetWeight,
  });
}

function highestContrastForeground(background: string): string {
  return contrastRatio(background, "#000000") >= contrastRatio(background, "#FFFFFF") ? "#000000" : "#FFFFFF";
}

function ensureContrast(color: string, surface: string, minimum: number): string {
  if (contrastRatio(color, surface) >= minimum) {
    return color;
  }

  const candidates = ["#000000", "#FFFFFF"]
    .map((target) => ({ target, value: closestAccessibleMix(color, target, surface, minimum) }))
    .filter((candidate): candidate is { target: string; value: string } => candidate.value !== null);

  if (candidates.length === 0) {
    return highestContrastForeground(surface);
  }

  return candidates.sort((first, second) => {
    const firstDistance = colorDistance(color, first.value);
    const secondDistance = colorDistance(color, second.value);
    return firstDistance - secondDistance;
  })[0]?.value ?? highestContrastForeground(surface);
}

function closestAccessibleMix(color: string, target: string, surface: string, minimum: number): string | null {
  if (contrastRatio(target, surface) < minimum) {
    return null;
  }

  let lower = 0;
  let upper = 1;
  for (let iteration = 0; iteration < 16; iteration += 1) {
    const midpoint = (lower + upper) / 2;
    const mixed = mixHex(color, target, midpoint);
    if (contrastRatio(mixed, surface) >= minimum) {
      upper = midpoint;
    } else {
      lower = midpoint;
    }
  }

  return mixHex(color, target, upper);
}

function colorDistance(first: string, second: string): number {
  const left = hexToRGB(first);
  const right = hexToRGB(second);
  return (left.red - right.red) ** 2 + (left.green - right.green) ** 2 + (left.blue - right.blue) ** 2;
}
