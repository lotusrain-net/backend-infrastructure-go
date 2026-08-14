export type ThemeName = "enterprise" | "cyberpunk";
export type ColorMode = "light" | "dark" | "system";
export type ResolvedColorMode = Exclude<ColorMode, "system">;
export type FontScale = "small" | "standard" | "large";
export type RadiusScale = "square" | "compact" | "rounded";

export interface AccountPreferences {
  theme: ThemeName;
  color_mode: ColorMode;
  accent_color: string | null;
  font_scale: FontScale;
  radius_scale: RadiusScale;
}

export const DEFAULT_ACCOUNT_PREFERENCES: AccountPreferences = {
  theme: "enterprise",
  color_mode: "system",
  accent_color: null,
  font_scale: "standard",
  radius_scale: "compact",
};

/**
 * Compatibility tokens still consumed by the first generation of UI
 * primitives. New primitives should use the semantic ThemeTokens fields.
 */
export interface AccentTokens {
  primary: string;
  hover: string;
  subtle: string;
  contrast: string;
  focus: string;
}

/**
 * The complete runtime palette derived from an account's optional seed color.
 * These names intentionally mirror the semantic CSS custom properties.
 */
export interface ThemeTokens extends AccentTokens {
  canvas: string;
  surface: string;
  surfaceRaised: string;
  surfaceSubtle: string;
  surfaceHover: string;
  foreground: string;
  mutedForeground: string;
  inverse: string;
  card: string;
  cardForeground: string;
  popover: string;
  popoverForeground: string;
  primaryForeground: string;
  primaryHover: string;
  primarySubtle: string;
  secondary: string;
  secondaryForeground: string;
  muted: string;
  accent: string;
  accentForeground: string;
  border: string;
  borderStrong: string;
  input: string;
  inputBackground: string;
  sidebar: string;
  sidebarForeground: string;
  sidebarPrimary: string;
  sidebarPrimaryForeground: string;
  sidebarAccent: string;
  sidebarAccentForeground: string;
  sidebarBorder: string;
  sidebarRing: string;
  chartPrimary: string;
  chartSecondary: string;
  chartTertiary: string;
  chartQuaternary: string;
  chartQuinary: string;
  shadowColor: string;
}

interface BaseThemePalette {
  canvas: string;
  surface: string;
  surfaceRaised: string;
  surfaceSubtle: string;
  surfaceHover: string;
  foreground: string;
  mutedForeground: string;
  inverse: string;
  popover: string;
  inputBackground: string;
  secondary: string;
  secondaryForeground: string;
  border: string;
  borderStrong: string;
  sidebar: string;
  sidebarForeground: string;
  chartPrimary: string;
  chartSecondary: string;
  chartTertiary: string;
  chartQuaternary: string;
  chartQuinary: string;
  focus: string;
}

interface RGB {
  red: number;
  green: number;
  blue: number;
}

const hexPattern = /^#[0-9a-f]{6}$/i;

/**
 * These are the brand palettes used when no custom seed is selected. They are
 * intentionally kept separate from semantic status colors: a seed may tint
 * the application chrome without changing success, warning, or danger meaning.
 */
const baseThemePalettes: Record<ThemeName, Record<ResolvedColorMode, BaseThemePalette>> = {
  enterprise: {
    light: {
      canvas: "#F3F6F8",
      surface: "#FFFFFF",
      surfaceRaised: "#FFFDFC",
      surfaceSubtle: "#E8F1EF",
      surfaceHover: "#DCEBE7",
      foreground: "#1D2939",
      mutedForeground: "#526071",
      inverse: "#FFFFFF",
      popover: "#FFFEFC",
      inputBackground: "#FEFFFE",
      secondary: "#FCE7DD",
      secondaryForeground: "#7B321C",
      border: "#CCDAD7",
      borderStrong: "#8EA8A2",
      sidebar: "#F6FBF9",
      sidebarForeground: "#20343A",
      chartPrimary: "#147D6B",
      chartSecondary: "#F58A65",
      chartTertiary: "#5A7EA8",
      chartQuaternary: "#B26E9A",
      chartQuinary: "#7A9F52",
      focus: "#0C7A67",
    },
    dark: {
      canvas: "#10191B",
      surface: "#152528",
      surfaceRaised: "#1A2D30",
      surfaceSubtle: "#20373A",
      surfaceHover: "#294347",
      foreground: "#F1FCF8",
      mutedForeground: "#B5CBC5",
      inverse: "#0D1719",
      popover: "#182A2D",
      inputBackground: "#17282A",
      secondary: "#3F2D2C",
      secondaryForeground: "#FFD6C6",
      border: "#345653",
      borderStrong: "#5B817B",
      sidebar: "#122124",
      sidebarForeground: "#E8F8F3",
      chartPrimary: "#78E0C3",
      chartSecondary: "#FFAD8E",
      chartTertiary: "#8CB7F1",
      chartQuaternary: "#E6A3D1",
      chartQuinary: "#B7DD77",
      focus: "#A1F5DF",
    },
  },
  cyberpunk: {
    light: {
      canvas: "#F3F7FC",
      surface: "#FBFDFF",
      surfaceRaised: "#FFFFFF",
      surfaceSubtle: "#E8F4F8",
      surfaceHover: "#DDF0F6",
      foreground: "#112338",
      mutedForeground: "#456477",
      inverse: "#F7FDFF",
      popover: "#FFFFFF",
      inputBackground: "#FCFEFF",
      secondary: "#F8E8F0",
      secondaryForeground: "#7A1D54",
      border: "#B7D8E2",
      borderStrong: "#6AB8CA",
      sidebar: "#F6FBFE",
      sidebarForeground: "#163042",
      chartPrimary: "#007C91",
      chartSecondary: "#B7226A",
      chartTertiary: "#3D75B4",
      chartQuaternary: "#7B6FD0",
      chartQuinary: "#5B9A73",
      focus: "#B81C72",
    },
    dark: {
      canvas: "#070B13",
      surface: "#0D1521",
      surfaceRaised: "#122033",
      surfaceSubtle: "#16283A",
      surfaceHover: "#1B334A",
      foreground: "#E9FCFF",
      mutedForeground: "#A5D0DC",
      inverse: "#061017",
      popover: "#101A28",
      inputBackground: "#0F1926",
      secondary: "#3C1A34",
      secondaryForeground: "#FFD6E9",
      border: "#255870",
      borderStrong: "#3E9DB2",
      sidebar: "#0A111C",
      sidebarForeground: "#E0F9FF",
      chartPrimary: "#00C9E7",
      chartSecondary: "#FF6DA9",
      chartTertiary: "#7EA8FF",
      chartQuaternary: "#BA8CFF",
      chartQuinary: "#71D7A5",
      focus: "#FF5CA2",
    },
  },
};

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

/**
 * Retained for callers that only need action colors. The primary is adjusted
 * only when necessary to remain usable as text on the supplied surface.
 */
export function deriveAccentTokens(
  accent: string,
  surface: string,
  _colorMode: ResolvedColorMode,
): AccentTokens {
  const normalizedAccent = normalizeAccentColor(accent);
  const normalizedSurface = normalizeAccentColor(surface);
  if (!normalizedAccent || !normalizedSurface) {
    throw new Error("Accent and surface colors must use #RRGGBB.");
  }

  const primary = ensureContrast(normalizedAccent, normalizedSurface, 5);
  const contrast = highestContrastForeground(primary);
  const hover = ensurePairContrast(
    mixHex(primary, contrast === "#000000" ? "#FFFFFF" : "#000000", 0.16),
    contrast,
    4.5,
  );

  return {
    primary,
    hover,
    subtle: deriveSubtleSurface(primary, normalizedSurface, 4.5),
    contrast,
    focus: ensureContrast(primary, normalizedSurface, 3),
  };
}

/**
 * Turn a valid custom seed into an entire theme palette. Surface colors remain
 * surface-led (rather than an 86% primary mix), so user-picked colors affect
 * the whole interface without sacrificing readable selected states.
 */
export function deriveThemeTokens(
  accent: string,
  theme: ThemeName,
  colorMode: ResolvedColorMode,
): ThemeTokens {
  const normalizedAccent = normalizeAccentColor(accent);
  if (!normalizedAccent) {
    throw new Error("Accent colors must use #RRGGBB.");
  }

  const base = baseThemePalettes[theme][colorMode];
  const action = deriveAccentTokens(normalizedAccent, base.surface, colorMode);
  const tintWeight = colorMode === "dark" ? 0.11 : 0.075;
  const tint = (color: string, weight: number) => mixHex(color, action.primary, weight);
  const canvas = tint(base.canvas, tintWeight);
  const surface = tint(base.surface, colorMode === "dark" ? 0.075 : 0.055);
  const surfaceRaised = tint(base.surfaceRaised, colorMode === "dark" ? 0.095 : 0.07);
  const surfaceSubtle = tint(base.surfaceSubtle, colorMode === "dark" ? 0.15 : 0.12);
  const surfaceHover = tint(base.surfaceHover, colorMode === "dark" ? 0.19 : 0.16);
  const popover = tint(base.popover, colorMode === "dark" ? 0.085 : 0.025);
  const inputBackground = tint(base.inputBackground, colorMode === "dark" ? 0.065 : 0.035);
  const border = tint(base.border, colorMode === "dark" ? 0.22 : 0.18);
  const borderStrong = tint(base.borderStrong, colorMode === "dark" ? 0.32 : 0.28);
  const sidebar = tint(base.sidebar, colorMode === "dark" ? 0.13 : 0.07);
  const secondary = tint(base.secondary, colorMode === "dark" ? 0.12 : 0.1);
  const focus = ensureContrast(tint(base.focus, 0.4), canvas, 3);

  return {
    ...action,
    canvas,
    surface,
    surfaceRaised,
    surfaceSubtle,
    surfaceHover,
    foreground: base.foreground,
    mutedForeground: base.mutedForeground,
    inverse: base.inverse,
    card: surface,
    cardForeground: base.foreground,
    popover,
    popoverForeground: base.foreground,
    primaryForeground: action.contrast,
    primaryHover: action.hover,
    primarySubtle: action.subtle,
    secondary,
    secondaryForeground: base.secondaryForeground,
    muted: surfaceSubtle,
    accent: surfaceHover,
    accentForeground: base.foreground,
    border,
    borderStrong,
    input: borderStrong,
    inputBackground,
    sidebar,
    sidebarForeground: base.sidebarForeground,
    sidebarPrimary: action.primary,
    sidebarPrimaryForeground: action.contrast,
    sidebarAccent: surfaceHover,
    sidebarAccentForeground: base.foreground,
    sidebarBorder: border,
    sidebarRing: focus,
    chartPrimary: action.primary,
    chartSecondary: tint(base.chartSecondary, 0.42),
    chartTertiary: tint(base.chartTertiary, 0.24),
    chartQuaternary: tint(base.chartQuaternary, 0.34),
    chartQuinary: tint(base.chartQuinary, 0.3),
    shadowColor: colorWithAlpha(action.primary, colorMode === "dark" ? 0.28 : 0.16),
    focus,
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

/** targetWeight is deliberately the target's weight, which keeps call sites readable. */
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

function ensurePairContrast(background: string, foreground: string, minimum: number): string {
  if (contrastRatio(background, foreground) >= minimum) {
    return background;
  }

  const target = foreground === "#FFFFFF" ? "#000000" : "#FFFFFF";
  return closestAccessibleMix(background, target, foreground, minimum) ?? target;
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

  return candidates.sort((first, second) => colorDistance(color, first.value) - colorDistance(color, second.value))[0]?.value
    ?? highestContrastForeground(surface);
}

function deriveSubtleSurface(foreground: string, surface: string, minimum: number): string {
  const preferredSurfaceWeight = 0.88;
  const preferred = mixHex(foreground, surface, preferredSurfaceWeight);
  if (contrastRatio(foreground, preferred) >= minimum) {
    return preferred;
  }

  if (contrastRatio(foreground, surface) < minimum) {
    return surface;
  }

  let lower = preferredSurfaceWeight;
  let upper = 1;
  for (let iteration = 0; iteration < 18; iteration += 1) {
    const midpoint = (lower + upper) / 2;
    const candidate = mixHex(foreground, surface, midpoint);
    if (contrastRatio(foreground, candidate) >= minimum) {
      upper = midpoint;
    } else {
      lower = midpoint;
    }
  }

  return mixHex(foreground, surface, upper);
}

function closestAccessibleMix(color: string, target: string, surface: string, minimum: number): string | null {
  if (contrastRatio(target, surface) < minimum) {
    return null;
  }

  let lower = 0;
  let upper = 1;
  for (let iteration = 0; iteration < 18; iteration += 1) {
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

function colorWithAlpha(value: string, alpha: number): string {
  const { red, green, blue } = hexToRGB(value);
  return `rgb(${red} ${green} ${blue} / ${alpha})`;
}

export function resolveColorMode(mode: ColorMode, prefersDark = false): ResolvedColorMode {
  return mode === "system" ? (prefersDark ? "dark" : "light") : mode;
}

export function themeTokenProperties(tokens: ThemeTokens): Record<string, string> {
  return {
    "--background": tokens.canvas,
    "--foreground": tokens.foreground,
    "--card": tokens.card,
    "--card-foreground": tokens.cardForeground,
    "--popover": tokens.popover,
    "--popover-foreground": tokens.popoverForeground,
    "--primary": tokens.primary,
    "--primary-foreground": tokens.primaryForeground,
    "--primary-hover": tokens.primaryHover,
    "--primary-subtle": tokens.primarySubtle,
    "--primary-subtle-foreground": tokens.primary,
    "--secondary": tokens.secondary,
    "--secondary-foreground": tokens.secondaryForeground,
    "--muted": tokens.muted,
    "--muted-foreground": tokens.mutedForeground,
    "--accent": tokens.accent,
    "--accent-foreground": tokens.accentForeground,
    "--border": tokens.border,
    "--input": tokens.input,
    "--input-background": tokens.inputBackground,
    "--ring": tokens.focus,
    "--sidebar": tokens.sidebar,
    "--sidebar-foreground": tokens.sidebarForeground,
    "--sidebar-muted": tokens.mutedForeground,
    "--sidebar-primary": tokens.sidebarPrimary,
    "--sidebar-primary-foreground": tokens.sidebarPrimaryForeground,
    "--sidebar-accent": tokens.sidebarAccent,
    "--sidebar-accent-foreground": tokens.sidebarAccentForeground,
    "--sidebar-border": tokens.sidebarBorder,
    "--sidebar-ring": tokens.sidebarRing,
    "--chart-1": tokens.chartPrimary,
    "--chart-2": tokens.chartSecondary,
    "--chart-3": tokens.chartTertiary,
    "--chart-4": tokens.chartQuaternary,
    "--chart-5": tokens.chartQuinary,
    "--surface-raised": tokens.surfaceRaised,
    "--shadow-color": tokens.shadowColor,
    "--shadow-card": `0 1px 2px ${tokens.shadowColor}, 0 10px 28px ${tokens.shadowColor}`,
    "--shadow-popover": `0 14px 32px ${tokens.shadowColor}`,
    "--shadow-dialog": `0 22px 64px ${tokens.shadowColor}`,
    "--canvas": tokens.canvas,
    "--surface": tokens.surface,
    "--surface-subtle": tokens.surfaceSubtle,
    "--surface-hover": tokens.surfaceHover,
    "--fg-default": tokens.foreground,
    "--fg-muted": tokens.mutedForeground,
    "--fg-inverse": tokens.inverse,
    "--border-subtle": tokens.border,
    "--border-strong": tokens.borderStrong,
    "--accent-primary": tokens.primary,
    "--accent-primary-hover": tokens.primaryHover,
    "--accent-primary-subtle": tokens.primarySubtle,
    "--accent-contrast": tokens.primaryForeground,
    "--focus-ring": tokens.focus,
    "--chart-primary": tokens.chartPrimary,
  };
}
