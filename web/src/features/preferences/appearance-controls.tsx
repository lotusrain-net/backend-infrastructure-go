"use client";

import * as React from "react";
import { Monitor, Moon, Palette, RotateCcw, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import {
  DEFAULT_ACCOUNT_PREFERENCES,
  normalizeAccentColor,
} from "@/features/preferences/appearance";
import { useAppearancePreferences } from "@/features/preferences/provider";
import type { ColorMode, FontScale, RadiusScale, ThemeName } from "@/types/api";

const themes: Array<{ value: ThemeName; label: string }> = [
  { value: "enterprise", label: "企业" },
  { value: "cyberpunk", label: "赛博朋克" },
];

const colorModes: Array<{ value: ColorMode; label: string; icon: typeof Sun }> = [
  { value: "light", label: "浅色", icon: Sun },
  { value: "dark", label: "深色", icon: Moon },
  { value: "system", label: "跟随系统", icon: Monitor },
];

const fontScales: Array<{ value: FontScale; label: string }> = [
  { value: "small", label: "小" },
  { value: "standard", label: "标准" },
  { value: "large", label: "大" },
];

const radiusScales: Array<{ value: RadiusScale; label: string }> = [
  { value: "square", label: "直角" },
  { value: "compact", label: "紧凑" },
  { value: "rounded", label: "圆润" },
];

export function AppearanceControls({ compact = false }: { compact?: boolean }) {
  const appearance = useAppearancePreferences();

  return <AppearanceControlsForm key={appearance.preferences.accent_color ?? "theme-default"} compact={compact} {...appearance} />;
}

function AppearanceControlsForm({
  compact,
  preferences,
  syncStatus,
  syncError,
  updatePreferences,
  retry,
}: {
  compact: boolean;
  preferences: ReturnType<typeof useAppearancePreferences>["preferences"];
  syncStatus: ReturnType<typeof useAppearancePreferences>["syncStatus"];
  syncError: string | null;
  updatePreferences: ReturnType<typeof useAppearancePreferences>["updatePreferences"];
  retry: ReturnType<typeof useAppearancePreferences>["retry"];
}) {
  const [accentDraft, setAccentDraft] = React.useState(preferences.accent_color ?? "");
  const [accentError, setAccentError] = React.useState<string | null>(null);

  function commitAccent(value: string) {
    const trimmed = value.trim();
    if (!trimmed) {
      setAccentError(null);
      updatePreferences({ accent_color: null });
      return;
    }

    const normalized = normalizeAccentColor(trimmed);
    if (!normalized) {
      setAccentError("请输入 #RRGGBB 格式的颜色。");
      return;
    }

    setAccentError(null);
    setAccentDraft(normalized);
    updatePreferences({ accent_color: normalized });
  }

  return (
    <div className="space-y-5">
      <label className="grid gap-1.5 text-[length:var(--text-label)] font-medium text-[color:var(--fg-muted)]">
        <span className="flex items-center gap-2"><Palette aria-hidden="true" className="h-4 w-4" />界面主题</span>
        <Select value={preferences.theme} onChange={(event) => updatePreferences({ theme: event.target.value as ThemeName })}>
          {themes.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </Select>
      </label>

      <SegmentedControl
        label="色彩模式"
        value={preferences.color_mode}
        options={colorModes.map(({ icon: Icon, ...option }) => ({ ...option, icon: <Icon aria-hidden="true" className="h-4 w-4" /> }))}
        onChange={(color_mode) => updatePreferences({ color_mode: color_mode as ColorMode })}
      />

      {compact ? null : (
        <>
          <fieldset className="grid gap-2">
            <legend className="text-[length:var(--text-label)] font-medium text-[color:var(--fg-muted)]">主强调色</legend>
            <div className="flex flex-wrap items-center gap-3">
              <input
                aria-label="主强调色"
                type="color"
                value={preferences.accent_color ?? "#0F63C9"}
                onChange={(event) => {
                  setAccentDraft(event.target.value.toUpperCase());
                  commitAccent(event.target.value);
                }}
                className="h-10 w-12 cursor-pointer border border-[color:var(--border-strong)] bg-[color:var(--surface)] p-1 [border-radius:var(--radius-md)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)]"
              />
              <label className="grid min-w-[13rem] flex-1 gap-1 text-[length:var(--text-label)] text-[color:var(--fg-muted)]">
                <span>主强调色 Hex</span>
                <Input
                  value={accentDraft}
                  onChange={(event) => setAccentDraft(event.target.value)}
                  onBlur={(event) => commitAccent(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      commitAccent(event.currentTarget.value);
                    }
                  }}
                  aria-invalid={accentError ? "true" : undefined}
                  aria-describedby={accentError ? "accent-color-error" : undefined}
                  placeholder="#0F63C9"
                />
              </label>
            </div>
            {accentError ? <p id="accent-color-error" role="alert" className="text-[length:var(--text-label)] text-[color:var(--danger)]">{accentError}</p> : null}
          </fieldset>

          <SegmentedControl
            label="圆角"
            value={preferences.radius_scale}
            options={radiusScales}
            onChange={(radius_scale) => updatePreferences({ radius_scale: radius_scale as RadiusScale })}
          />
          <SegmentedControl
            label="字号"
            value={preferences.font_scale}
            options={fontScales}
            onChange={(font_scale) => updatePreferences({ font_scale: font_scale as FontScale })}
          />

          <Button type="button" variant="secondary" onClick={() => updatePreferences(DEFAULT_ACCOUNT_PREFERENCES)}>
            <RotateCcw aria-hidden="true" className="h-4 w-4" />
            重置外观
          </Button>
        </>
      )}

      {compact ? null : <SyncStatus status={syncStatus} error={syncError} onRetry={retry} />}
    </div>
  );
}

function SegmentedControl({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: Array<{ value: string; label: string; icon?: React.ReactNode }>;
  onChange: (value: string) => void;
}) {
  return (
    <fieldset>
      <legend className="text-[length:var(--text-label)] font-medium text-[color:var(--fg-muted)]">{label}</legend>
      <div className="mt-1.5 grid grid-flow-col auto-cols-fr gap-2">
        {options.map((option) => {
          const selected = option.value === value;
          return (
            <button
              key={option.value}
              type="button"
              aria-pressed={selected}
              onClick={() => onChange(option.value)}
              className={`flex min-h-10 items-center justify-center gap-1 border px-2 text-[length:var(--text-label)] font-medium transition-colors [border-radius:var(--radius-sm)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)] ${selected ? "border-[color:var(--accent-primary)] bg-[color:var(--accent-primary-subtle)] text-[color:var(--accent-primary)]" : "border-[color:var(--border-subtle)] text-[color:var(--fg-muted)] hover:bg-[color:var(--surface-hover)]"}`}
            >
              {option.icon}
              {option.label}
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}

function SyncStatus({
  status,
  error,
  onRetry,
}: {
  status: "idle" | "loading" | "syncing" | "synced" | "error";
  error: string | null;
  onRetry: () => void;
}) {
  const label =
    status === "loading"
      ? "正在获取账号外观偏好..."
      : status === "syncing"
        ? "正在同步外观偏好..."
        : status === "synced"
          ? "外观偏好已同步"
          : status === "error"
            ? error ?? "外观偏好同步失败"
            : "外观偏好尚未同步";

  return (
    <div className="flex flex-wrap items-center gap-3" role="status" aria-live="polite">
      <span className={status === "error" ? "text-[length:var(--text-label)] text-[color:var(--danger)]" : "text-[length:var(--text-label)] text-[color:var(--fg-muted)]"}>{label}</span>
      {status === "error" ? <Button type="button" variant="secondary" size="sm" onClick={onRetry}>重试同步</Button> : null}
    </div>
  );
}
