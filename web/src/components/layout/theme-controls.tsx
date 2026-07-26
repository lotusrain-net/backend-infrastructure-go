"use client";

import { Monitor, Moon, Palette, Sun } from "lucide-react";
import { Select } from "@/components/ui/select";
import { useThemeStore } from "@/stores/theme-store";
import type { ColorMode, ThemeName } from "@/types/api";

const themes: Array<{ value: ThemeName; label: string }> = [
  { value: "enterprise", label: "企业" },
  { value: "cyberpunk", label: "赛博朋克" },
];

const colorModes: Array<{ value: ColorMode; label: string; icon: typeof Sun }> = [
  { value: "light", label: "浅色", icon: Sun },
  { value: "dark", label: "深色", icon: Moon },
  { value: "system", label: "跟随系统", icon: Monitor },
];

export function ThemeControls() {
  const theme = useThemeStore((state) => state.theme);
  const colorMode = useThemeStore((state) => state.colorMode);
  const setTheme = useThemeStore((state) => state.setTheme);
  const setColorMode = useThemeStore((state) => state.setColorMode);

  return (
    <div className="space-y-3">
      <label className="grid gap-1.5 text-xs font-medium text-[color:var(--fg-muted)]">
        <span className="flex items-center gap-2"><Palette aria-hidden="true" className="h-3.5 w-3.5" />界面主题</span>
        <Select value={theme} onChange={(event) => setTheme(event.target.value as ThemeName)}>
          {themes.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </Select>
      </label>
      <fieldset>
        <legend className="text-xs font-medium text-[color:var(--fg-muted)]">色彩模式</legend>
        <div className="mt-1.5 grid grid-cols-3 gap-2">
          {colorModes.map((option) => {
            const Icon = option.icon;
            const selected = option.value === colorMode;
            return (
              <button key={option.value} type="button" onClick={() => setColorMode(option.value)} aria-pressed={selected} className={`flex min-h-10 flex-col items-center justify-center gap-1 border px-1 text-[11px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)] ${selected ? "border-[color:var(--accent-primary)] bg-[color:var(--accent-primary-subtle)] text-[color:var(--accent-primary)]" : "border-[color:var(--border-subtle)] text-[color:var(--fg-muted)] hover:bg-[color:var(--surface-hover)]"}`}>
                <Icon aria-hidden="true" className="h-3.5 w-3.5" />
                {option.label}
              </button>
            );
          })}
        </div>
      </fieldset>
    </div>
  );
}
