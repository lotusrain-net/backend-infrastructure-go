import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AppearancePreferencesProvider,
  useAppearancePreferences,
} from "@/features/preferences/provider";
import { AppearanceControls } from "@/features/preferences/appearance-controls";
import { getThemeState } from "@/stores/theme-store";
import type { AccountPreferences } from "@/types/api";

const useCurrentUserPreferencesQuery = vi.fn();
const usePutCurrentUserPreferencesMutation = vi.fn();
const mutateAsync = vi.fn();

vi.mock("@/features/preferences/api", () => ({
  useCurrentUserPreferencesQuery: (...args: unknown[]) => useCurrentUserPreferencesQuery(...args),
  usePutCurrentUserPreferencesMutation: () => usePutCurrentUserPreferencesMutation(),
}));

const serverPreferences: AccountPreferences = {
  theme: "cyberpunk",
  color_mode: "dark",
  accent_color: "#1A2B3C",
  font_scale: "large",
  radius_scale: "rounded",
};

function Harness() {
  const { preferences, syncStatus, previewPreferences, updatePreferences, retry } = useAppearancePreferences();
  return (
    <>
      <output data-testid="preferences">{JSON.stringify(preferences)}</output>
      <output data-testid="sync-status">{syncStatus}</output>
      <button onClick={() => previewPreferences({ accent_color: "#FFF200" })}>预览主题颜色</button>
      <button onClick={() => updatePreferences({ theme: "enterprise" })}>更新主题</button>
      <button onClick={() => updatePreferences({ font_scale: "large" })}>放大字号</button>
      <button onClick={retry}>重试</button>
    </>
  );
}

describe("AppearancePreferencesProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useCurrentUserPreferencesQuery.mockReturnValue({
      data: serverPreferences,
      isError: false,
      error: null,
      refetch: vi.fn(),
    });
    usePutCurrentUserPreferencesMutation.mockReturnValue({ mutateAsync });
  });

  it("lets a successful server preference response override the local account cache", async () => {
    window.localStorage.setItem(
      "backend-infra-appearance:user-1",
      JSON.stringify({ ...serverPreferences, theme: "enterprise" }),
    );

    render(<AppearancePreferencesProvider userID="user-1"><Harness /></AppearancePreferencesProvider>);

    await waitFor(() => {
      expect(screen.getByTestId("preferences").textContent).toContain("cyberpunk");
    });
    expect(getThemeState().syncStatus).toBe("synced");
  });

  it("keeps a failed update as the visible preview and retries the same complete payload", async () => {
    const user = userEvent.setup();
    mutateAsync.mockRejectedValue(new Error("同步失败"));
    render(<AppearancePreferencesProvider userID="user-1"><Harness /></AppearancePreferencesProvider>);

    await user.click(screen.getByRole("button", { name: "更新主题" }));

    await waitFor(() => expect(screen.getByTestId("sync-status").textContent).toBe("error"));
    expect(getThemeState().preferences.theme).toBe("enterprise");
    expect(mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ theme: "enterprise" }));

    await user.click(screen.getByRole("button", { name: "重试" }));
    expect(mutateAsync).toHaveBeenCalledTimes(2);
  });

  it("applies a local color preview without enqueueing an incomplete preference request", async () => {
    const user = userEvent.setup();
    render(<AppearancePreferencesProvider userID="user-1"><Harness /></AppearancePreferencesProvider>);

    await user.click(screen.getByRole("button", { name: "预览主题颜色" }));

    expect(getThemeState().preferences.accent_color).toBe("#FFF200");
    expect(getThemeState().isPreviewing).toBe(true);
    expect(getThemeState().pendingPreferences).toBeNull();
    expect(document.documentElement.style.getPropertyValue("--background")).not.toBe("");
    expect(mutateAsync).not.toHaveBeenCalled();
  });

  it("keeps the native color picker mounted while a local preview updates the provider", () => {
    render(<AppearancePreferencesProvider userID="user-1"><AppearanceControls /></AppearancePreferencesProvider>);

    const picker = screen.getByLabelText("主强调色");
    fireEvent.input(picker, { target: { value: "#1a2b3c" } });

    expect(screen.getByLabelText("主强调色")).toBe(picker);
    expect(getThemeState().preferences.accent_color).toBe("#1A2B3C");
    expect(mutateAsync).not.toHaveBeenCalled();
  });

  it("serializes rapid changes and never applies an older server response over the latest preview", async () => {
    const user = userEvent.setup();
    let resolveFirst: (value: AccountPreferences) => void = () => undefined;
    let resolveSecond: (value: AccountPreferences) => void = () => undefined;
    const first = new Promise<AccountPreferences>((resolve) => {
      resolveFirst = resolve;
    });
    const second = new Promise<AccountPreferences>((resolve) => {
      resolveSecond = resolve;
    });
    mutateAsync.mockReturnValueOnce(first).mockReturnValueOnce(second);
    render(<AppearancePreferencesProvider userID="user-1"><Harness /></AppearancePreferencesProvider>);

    await user.click(screen.getByRole("button", { name: "更新主题" }));
    await user.click(screen.getByRole("button", { name: "放大字号" }));

    expect(mutateAsync).toHaveBeenCalledTimes(1);
    expect(getThemeState().preferences).toEqual(expect.objectContaining({ theme: "enterprise", font_scale: "large" }));

    resolveFirst(serverPreferences);
    await waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(2));
    expect(mutateAsync).toHaveBeenLastCalledWith(expect.objectContaining({ theme: "enterprise", font_scale: "large" }));
    expect(getThemeState().preferences).toEqual(expect.objectContaining({ theme: "enterprise", font_scale: "large" }));

    resolveSecond({ ...serverPreferences, theme: "enterprise", font_scale: "large" });
    await waitFor(() => expect(getThemeState().syncStatus).toBe("synced"));
    expect(getThemeState().preferences).toEqual(expect.objectContaining({ theme: "enterprise", font_scale: "large" }));
  });
});
