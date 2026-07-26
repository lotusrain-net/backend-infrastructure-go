"use client";

import * as React from "react";
import {
  hydrateThemeStore,
  initializeThemeStore,
  useThemeStore,
} from "@/stores/theme-store";

export function ThemeInit() {
  const colorMode = useThemeStore((state) => state.colorMode);
  const theme = useThemeStore((state) => state.theme);

  React.useEffect(() => {
    hydrateThemeStore();
    initializeThemeStore();
  }, []);

  React.useEffect(() => {
    if (typeof window === "undefined" || colorMode !== "system") {
      return;
    }

    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => initializeThemeStore();
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [colorMode, theme]);

  return null;
}
