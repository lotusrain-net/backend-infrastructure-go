"use client";

import * as React from "react";
import { Monitor, Moon, Palette, RotateCcw, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  DEFAULT_ACCOUNT_PREFERENCES,
  normalizeAccentColor,
} from "@lotusrain-net/backend-infrastructure-web/theme";
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

const ACCENT_COMMIT_DELAY = 300;

export function AppearanceControls({ compact = false }: { compact?: boolean }) {
  const appearance = useAppearancePreferences();

  return <AppearanceControlsForm compact={compact} {...appearance} />;
}

function AppearanceControlsForm({
  compact,
  preferences,
  syncStatus,
  syncError,
  previewPreferences,
  updatePreferences,
  retry,
}: {
  compact: boolean;
  preferences: ReturnType<typeof useAppearancePreferences>["preferences"];
  syncStatus: ReturnType<typeof useAppearancePreferences>["syncStatus"];
  syncError: string | null;
  previewPreferences: ReturnType<typeof useAppearancePreferences>["previewPreferences"];
  updatePreferences: ReturnType<typeof useAppearancePreferences>["updatePreferences"];
  retry: ReturnType<typeof useAppearancePreferences>["retry"];
}) {
  const [accentDraft, setAccentDraft] = React.useState<string | null>(null);
  const [accentError, setAccentError] = React.useState<string | null>(null);
  const accentCommitTimeout = React.useRef<number | null>(null);

  const clearAccentCommit = React.useCallback(() => {
    if (accentCommitTimeout.current !== null) {
      window.clearTimeout(accentCommitTimeout.current);
      accentCommitTimeout.current = null;
    }
  }, []);

  React.useEffect(() => {
    return clearAccentCommit;
  }, [clearAccentCommit]);

  const previewAccent = React.useCallback(
    (value: string) => {
      const accent = parseAccentDraft(value);
      if (accent === undefined) {
        return;
      }

      setAccentError(null);
      previewPreferences({ accent_color: accent });
    },
    [previewPreferences],
  );

  const scheduleAccentCommit = React.useCallback(
    (value: string) => {
      const accent = parseAccentDraft(value);
      if (accent === undefined) {
        setAccentError("请输入 #RRGGBB 格式的颜色。");
        return;
      }

      setAccentError(null);
      clearAccentCommit();
      accentCommitTimeout.current = window.setTimeout(() => {
        accentCommitTimeout.current = null;
        updatePreferences({ accent_color: accent });
      }, ACCENT_COMMIT_DELAY);
    },
    [clearAccentCommit, updatePreferences],
  );

  const displayedAccent = accentDraft ?? preferences.accent_color ?? "";
  const pickerAccent = normalizeAccentColor(displayedAccent) ?? preferences.accent_color ?? "#147D6B";

  return (
    <div className="space-y-5">
      <div className="grid gap-1.5 text-[length:var(--text-label)] font-medium text-[color:var(--muted-foreground)]">
        <span className="flex items-center gap-2"><Palette aria-hidden="true" className="h-4 w-4" />界面主题</span>
        <Select value={preferences.theme} onValueChange={(theme) => updatePreferences({ theme: theme as ThemeName })}>
          <SelectTrigger aria-label="界面主题"><SelectValue /></SelectTrigger>
          <SelectContent>
            {themes.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>

      <SegmentedControl
        label="色彩模式"
        value={preferences.color_mode}
        options={colorModes.map(({ icon: Icon, ...option }) => ({ ...option, icon: <Icon aria-hidden="true" className="h-4 w-4" /> }))}
        onChange={(color_mode) => updatePreferences({ color_mode: color_mode as ColorMode })}
      />

      {compact ? null : (
        <>
          <fieldset className="grid gap-2">
            <legend className="text-[length:var(--text-label)] font-medium text-[color:var(--muted-foreground)]">主强调色</legend>
            <div className="flex flex-wrap items-center gap-3">
              <input
                aria-label="主强调色"
                type="color"
                value={pickerAccent}
                onInput={(event) => {
                  const value = event.currentTarget.value.toUpperCase();
                  setAccentDraft(value);
                  previewAccent(value);
                }}
                onChange={(event) => {
                  const value = event.currentTarget.value.toUpperCase();
                  setAccentDraft(value);
                  previewAccent(value);
                  scheduleAccentCommit(value);
                }}
                onBlur={(event) => scheduleAccentCommit(event.currentTarget.value)}
                className="h-10 w-12 cursor-pointer border border-[color:var(--input)] bg-[color:var(--input-background)] p-1 [border-radius:var(--radius-md)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]"
              />
              <label className="grid min-w-[13rem] flex-1 gap-1 text-[length:var(--text-label)] text-[color:var(--muted-foreground)]">
                <span>主强调色 Hex</span>
                <Input
                  value={displayedAccent}
                  onChange={(event) => {
                    const value = event.target.value.toUpperCase();
                    setAccentDraft(value);
                    previewAccent(value);
                  }}
                  onBlur={(event) => scheduleAccentCommit(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      scheduleAccentCommit(event.currentTarget.value);
                    }
                  }}
                  aria-invalid={accentError ? "true" : undefined}
                  aria-describedby={accentError ? "accent-color-error" : undefined}
                  placeholder="#0F63C9"
                />
              </label>
            </div>
            {accentError ? <p id="accent-color-error" role="alert" className="text-[length:var(--text-label)] text-[color:var(--destructive)]">{accentError}</p> : null}
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

          <Button type="button" variant="secondary" onClick={() => {
            clearAccentCommit();
            setAccentDraft(null);
            setAccentError(null);
            updatePreferences(DEFAULT_ACCOUNT_PREFERENCES);
          }}>
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
      <legend className="text-[length:var(--text-label)] font-medium text-[color:var(--muted-foreground)]">{label}</legend>
      <div className="mt-1.5 grid grid-flow-col auto-cols-fr gap-2">
        {options.map((option) => {
          const selected = option.value === value;
          return (
            <button
              key={option.value}
              type="button"
              aria-pressed={selected}
              onClick={() => onChange(option.value)}
              className={`flex min-h-10 items-center justify-center gap-1 border px-2 text-[length:var(--text-label)] font-medium transition-colors [border-radius:var(--radius-sm)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] ${selected ? "border-[color:var(--primary)] bg-[color:var(--primary)] text-[color:var(--primary-foreground)]" : "border-[color:var(--border)] text-[color:var(--muted-foreground)] hover:bg-[color:var(--accent)] hover:text-[color:var(--accent-foreground)]"}`}
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
      <span className={status === "error" ? "text-[length:var(--text-label)] text-[color:var(--destructive)]" : "text-[length:var(--text-label)] text-[color:var(--muted-foreground)]"}>{label}</span>
      {status === "error" ? <Button type="button" variant="secondary" size="sm" onClick={onRetry}>重试同步</Button> : null}
    </div>
  );
}

function parseAccentDraft(value: string): string | null | undefined {
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }

  return normalizeAccentColor(trimmed) ?? undefined;
}
