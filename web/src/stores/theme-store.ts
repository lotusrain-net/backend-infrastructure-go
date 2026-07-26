import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { ColorMode, ThemeName } from "@/types/api";

const STORAGE_KEY = "backend-infra-theme";

export interface ThemeState {
  theme: ThemeName;
  colorMode: ColorMode;
  hydrated: boolean;
  setTheme: (theme: ThemeName) => void;
  setColorMode: (colorMode: ColorMode) => void;
  setHydrated: (hydrated: boolean) => void;
}

const defaultState = {
  theme: "enterprise" as ThemeName,
  colorMode: "system" as ColorMode,
  hydrated: false,
};

function resolveColorMode(colorMode: ColorMode) {
  if (colorMode !== "system") {
    return colorMode;
  }

  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return "light";
  }

  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyTheme(theme: ThemeName, colorMode: ColorMode) {
  if (typeof document === "undefined") {
    return;
  }

  const resolved = resolveColorMode(colorMode);
  const root = document.documentElement;
  root.dataset.theme = theme;
  root.dataset.colorMode = resolved;
  root.classList.toggle("dark", resolved === "dark");
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      ...defaultState,
      setTheme: (theme) => {
        set({ theme });
        applyTheme(theme, get().colorMode);
      },
      setColorMode: (colorMode) => {
        set({ colorMode });
        applyTheme(get().theme, colorMode);
      },
      setHydrated: (hydrated) => set({ hydrated }),
    }),
    {
      name: STORAGE_KEY,
      partialize: ({ theme, colorMode }) => ({ theme, colorMode }),
      onRehydrateStorage: () => (state) => {
        if (!state) {
          return;
        }

        state.setHydrated(true);
        applyTheme(state.theme, state.colorMode);
      },
    },
  ),
);

export function getThemeState() {
  return useThemeStore.getState();
}

export function subscribeThemeStore(listener: () => void) {
  return useThemeStore.subscribe(listener);
}

export function setTheme(theme: ThemeName) {
  useThemeStore.getState().setTheme(theme);
}

export function setColorMode(colorMode: ColorMode) {
  useThemeStore.getState().setColorMode(colorMode);
}

export function hydrateThemeStore() {
  void useThemeStore.persist.rehydrate();
}

export function initializeThemeStore() {
  const state = useThemeStore.getState();
  applyTheme(state.theme, state.colorMode);
}

export function resetThemeStore() {
  useThemeStore.persist.clearStorage();
  useThemeStore.setState(defaultState);
}
