"use client";

import * as React from "react";
import {
  initializeThemeStore,
  useThemeStore,
} from "@/stores/theme-store";

export function ThemeInit() {
  const colorMode = useThemeStore((state) => state.preferences.color_mode);
  const theme = useThemeStore((state) => state.preferences.theme);

  React.useEffect(() => {
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
