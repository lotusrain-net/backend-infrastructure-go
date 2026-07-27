import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AppearanceControls } from "@/features/preferences/appearance-controls";
import { DEFAULT_ACCOUNT_PREFERENCES } from "@/features/preferences/appearance";

const updatePreferences = vi.fn();
const retry = vi.fn();

vi.mock("@/features/preferences/provider", () => ({
  useAppearancePreferences: () => ({
    preferences: DEFAULT_ACCOUNT_PREFERENCES,
    syncStatus: "error",
    syncError: "网络连接失败",
    updatePreferences,
    retry,
  }),
}));

describe("AppearanceControls", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

  it("normalizes a valid Hex input before requesting an immediate preview sync", async () => {
    const user = userEvent.setup();
    render(<AppearanceControls />);

    const input = screen.getByLabelText("主强调色 Hex");
    await user.clear(input);
    await user.type(input, "#1a2b3c");
    await user.tab();

    expect(updatePreferences).toHaveBeenCalledWith({ accent_color: "#1A2B3C" });
  });
});
