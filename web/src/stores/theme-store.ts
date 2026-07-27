import { create } from "zustand";
import {
  createAppearancePreferences,
  DEFAULT_ACCOUNT_PREFERENCES,
  deriveAccentTokens,
  parseAppearancePreferences,
  type ResolvedColorMode,
} from "@/features/preferences/appearance";
import type { AccountPreferences, ColorMode } from "@/types/api";

export { createAppearancePreferences } from "@/features/preferences/appearance";

const CACHE_PREFIX = "backend-infra-appearance";
const surfaceColors = {
  enterprise: { light: "#FFFFFF", dark: "#182330" },
  cyberpunk: { light: "#FFFFFF", dark: "#0D1521" },
} as const;

export type PreferenceSyncStatus = "idle" | "loading" | "syncing" | "synced" | "error";
export type PreferenceFailureKind = "load" | "save" | null;

export interface ThemeState {
  userID: string | null;
  preferences: AccountPreferences;
  syncStatus: PreferenceSyncStatus;
  syncError: string | null;
  failureKind: PreferenceFailureKind;
  pendingPreferences: AccountPreferences | null;
  switchUser: (userID: string) => void;
  preview: (userID: string, preferences: AccountPreferences) => void;
  applyRemote: (userID: string, preferences: AccountPreferences) => void;
  markSyncFailed: (userID: string, error: unknown, kind?: Exclude<PreferenceFailureKind, null>) => void;
  markSyncing: (userID: string) => void;
  clearActiveUser: () => void;
}

const defaultState = {
  userID: null,
  preferences: DEFAULT_ACCOUNT_PREFERENCES,
  syncStatus: "idle" as PreferenceSyncStatus,
  syncError: null as string | null,
  failureKind: null as PreferenceFailureKind,
  pendingPreferences: null as AccountPreferences | null,
};

function resolveColorMode(colorMode: ColorMode): ResolvedColorMode {
  if (colorMode !== "system") {
    return colorMode;
  }

  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return "light";
  }

  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyPreferences(preferences: AccountPreferences) {
  if (typeof document === "undefined") {
    return;
  }

  const root = document.documentElement;
  const resolvedColorMode = resolveColorMode(preferences.color_mode);
  root.dataset.theme = preferences.theme;
  root.dataset.colorMode = resolvedColorMode;
  root.dataset.fontScale = preferences.font_scale;
  root.dataset.radiusScale = preferences.radius_scale;
  root.classList.toggle("dark", resolvedColorMode === "dark");

  if (preferences.accent_color) {
    const tokens = deriveAccentTokens(
      preferences.accent_color,
      surfaceColors[preferences.theme][resolvedColorMode],
      resolvedColorMode,
    );
    root.style.setProperty("--accent-primary", tokens.primary);
    root.style.setProperty("--accent-primary-hover", tokens.hover);
    root.style.setProperty("--accent-primary-subtle", tokens.subtle);
    root.style.setProperty("--accent-contrast", tokens.contrast);
    root.style.setProperty("--focus-ring", tokens.focus);
    root.style.setProperty("--chart-primary", tokens.primary);
    return;
  }

  for (const property of [
    "--accent-primary",
    "--accent-primary-hover",
    "--accent-primary-subtle",
    "--accent-contrast",
    "--focus-ring",
    "--chart-primary",
  ]) {
    root.style.removeProperty(property);
  }
}

function getCachedPreferences(userID: string): AccountPreferences | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    const serialized = window.localStorage.getItem(cacheKeyForUser(userID));
    return serialized ? parseAppearancePreferences(JSON.parse(serialized)) : null;
  } catch {
    return null;
  }
}

function cachePreferences(userID: string, preferences: AccountPreferences) {
  if (typeof window === "undefined") {
    return;
  }

  try {
    window.localStorage.setItem(cacheKeyForUser(userID), JSON.stringify(preferences));
  } catch {
    // Storage availability must not block an in-memory preview or remote synchronization.
  }
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  ...defaultState,
  switchUser: (userID) => {
    const preferences = getCachedPreferences(userID) ?? DEFAULT_ACCOUNT_PREFERENCES;
    set({
      userID,
      preferences,
      syncStatus: "loading",
      syncError: null,
      failureKind: null,
      pendingPreferences: null,
    });
    applyPreferences(preferences);
  },
  preview: (userID, preferences) => {
    if (get().userID !== userID) {
      return;
    }

    const normalized = createAppearancePreferences(preferences);
    cachePreferences(userID, normalized);
    set({
      preferences: normalized,
      syncStatus: "syncing",
      syncError: null,
      failureKind: null,
      pendingPreferences: normalized,
    });
    applyPreferences(normalized);
  },
  applyRemote: (userID, preferences) => {
    if (get().userID !== userID) {
      return;
    }

    const normalized = createAppearancePreferences(preferences);
    cachePreferences(userID, normalized);
    set({
      preferences: normalized,
      syncStatus: "synced",
      syncError: null,
      failureKind: null,
      pendingPreferences: null,
    });
    applyPreferences(normalized);
  },
  markSyncFailed: (userID, error, kind = "save") => {
    if (get().userID !== userID) {
      return;
    }

    set({
      syncStatus: "error",
      syncError: error instanceof Error ? error.message : "外观偏好同步失败，请重试。",
      failureKind: kind,
    });
  },
  markSyncing: (userID) => {
    if (get().userID === userID) {
      set({ syncStatus: "syncing", syncError: null, failureKind: null });
    }
  },
  clearActiveUser: () => {
    set(defaultState);
    applyPreferences(DEFAULT_ACCOUNT_PREFERENCES);
  },
}));

export function cacheKeyForUser(userID: string) {
  return `${CACHE_PREFIX}:${userID}`;
}

export function getThemeState() {
  return useThemeStore.getState();
}

export function switchThemeUser(userID: string) {
  useThemeStore.getState().switchUser(userID);
}

export function previewPreferences(userID: string, preferences: AccountPreferences) {
  useThemeStore.getState().preview(userID, preferences);
}

export function applyRemotePreferences(userID: string, preferences: AccountPreferences) {
  useThemeStore.getState().applyRemote(userID, preferences);
}

export function markPreferencesSyncFailed(
  userID: string,
  error: unknown,
  kind: Exclude<PreferenceFailureKind, null> = "save",
) {
  useThemeStore.getState().markSyncFailed(userID, error, kind);
}

export function initializeThemeStore() {
  applyPreferences(useThemeStore.getState().preferences);
}

export function clearActiveThemeState() {
  useThemeStore.getState().clearActiveUser();
}

export function resetThemeStore() {
  useThemeStore.setState(defaultState);
  applyPreferences(DEFAULT_ACCOUNT_PREFERENCES);
}
