import { create } from "zustand";
import {
  createAppearancePreferences,
  DEFAULT_ACCOUNT_PREFERENCES,
  deriveThemeTokens,
  parseAppearancePreferences,
  resolveColorMode,
  themeTokenProperties,
  type AccountPreferences,
} from "@lotusrain-net/backend-infrastructure-web/theme";

export { createAppearancePreferences } from "@lotusrain-net/backend-infrastructure-web/theme";

const CACHE_PREFIX = "backend-infra-appearance";
export type PreferenceSyncStatus = "idle" | "loading" | "syncing" | "synced" | "error";
export type PreferenceFailureKind = "load" | "save" | null;

export interface ThemeState {
  userID: string | null;
  preferences: AccountPreferences;
  syncStatus: PreferenceSyncStatus;
  syncError: string | null;
  failureKind: PreferenceFailureKind;
  pendingPreferences: AccountPreferences | null;
  isPreviewing: boolean;
  switchUser: (userID: string) => void;
  previewLocal: (userID: string, preferences: AccountPreferences) => void;
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
  isPreviewing: false,
};

function applyPreferences(preferences: AccountPreferences) {
  if (typeof document === "undefined") {
    return;
  }

  const root = document.documentElement;
  const prefersDark = typeof window !== "undefined"
    && typeof window.matchMedia === "function"
    && window.matchMedia("(prefers-color-scheme: dark)").matches;
  const resolvedColorMode = resolveColorMode(preferences.color_mode, prefersDark);
  root.dataset.theme = preferences.theme;
  root.dataset.colorMode = resolvedColorMode;
  root.dataset.fontScale = preferences.font_scale;
  root.dataset.radiusScale = preferences.radius_scale;
  root.classList.toggle("dark", resolvedColorMode === "dark");

  if (preferences.accent_color) {
    const tokens = deriveThemeTokens(
      preferences.accent_color,
      preferences.theme,
      resolvedColorMode,
    );
    for (const [property, value] of Object.entries(themeTokenProperties(tokens))) {
      root.style.setProperty(property, value);
    }
    return;
  }

  for (const property of seededThemeProperties) {
    root.style.removeProperty(property);
  }
}

const seededThemeProperties = Object.keys(themeTokenProperties(deriveThemeTokens("#1A2B3C", "enterprise", "light")));

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
      isPreviewing: false,
    });
    applyPreferences(preferences);
  },
  previewLocal: (userID, preferences) => {
    if (get().userID !== userID) {
      return;
    }

    const normalized = createAppearancePreferences(preferences);
    set({
      preferences: normalized,
      syncStatus: "idle",
      syncError: null,
      failureKind: null,
      pendingPreferences: null,
      isPreviewing: true,
    });
    applyPreferences(normalized);
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
      isPreviewing: false,
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
      isPreviewing: false,
    });
    applyPreferences(normalized);
  },
  markSyncFailed: (userID, error, kind = "save") => {
    if (get().userID !== userID || get().isPreviewing) {
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
      set({ syncStatus: "syncing", syncError: null, failureKind: null, isPreviewing: false });
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
