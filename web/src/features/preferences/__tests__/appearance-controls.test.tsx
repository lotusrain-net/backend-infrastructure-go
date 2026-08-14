import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AppearanceControls } from "@/features/preferences/appearance-controls";
import { DEFAULT_ACCOUNT_PREFERENCES } from "@purplevoid/backend-infrastructure-web/theme";

const updatePreferences = vi.fn();
const previewPreferences = vi.fn();
const retry = vi.fn();

vi.mock("@/features/preferences/provider", () => ({
  useAppearancePreferences: () => ({
    preferences: DEFAULT_ACCOUNT_PREFERENCES,
    syncStatus: "error",
    syncError: "网络连接失败",
    previewPreferences,
    updatePreferences,
    retry,
  }),
}));

describe("AppearanceControls", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
  });

  it("labels appearance controls and offers a visible retry after sync failure", async () => {
    const user = userEvent.setup();
    render(<AppearanceControls />);

    expect(screen.getByRole("combobox", { name: "界面主题" })).toBeTruthy();
    expect(screen.getByLabelText("主强调色 Hex")).toBeTruthy();
    expect(screen.getByRole("status").textContent).toContain("网络连接失败");

    await user.click(screen.getByRole("button", { name: "重试同步" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("previews picker input continuously and persists only the last complete color after a debounce", () => {
    vi.useFakeTimers();
    render(<AppearanceControls />);

    const picker = screen.getByLabelText("主强调色");
    fireEvent.input(picker, { target: { value: "#1a2b3c" } });
    fireEvent.change(picker, { target: { value: "#4d5e6f" } });

    expect(previewPreferences).toHaveBeenCalledWith({ accent_color: "#1A2B3C" });
    expect(previewPreferences).toHaveBeenLastCalledWith({ accent_color: "#4D5E6F" });
    expect(updatePreferences).not.toHaveBeenCalled();

    act(() => vi.advanceTimersByTime(300));
    expect(updatePreferences).toHaveBeenCalledTimes(1);
    expect(updatePreferences).toHaveBeenCalledWith({ accent_color: "#4D5E6F" });
  });

  it("normalizes valid Hex input before scheduling its final sync", async () => {
    const user = userEvent.setup();
    render(<AppearanceControls />);

    const input = screen.getByLabelText("主强调色 Hex");
    await user.clear(input);
    await user.type(input, "#1a2b3c");
    await user.tab();

    await new Promise((resolve) => window.setTimeout(resolve, 320));
    expect(previewPreferences).toHaveBeenCalledWith({ accent_color: "#1A2B3C" });
    expect(updatePreferences).toHaveBeenCalledWith({ accent_color: "#1A2B3C" });
  });
});
