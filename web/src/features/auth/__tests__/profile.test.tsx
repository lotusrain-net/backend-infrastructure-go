import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Profile } from "@/features/auth/profile";
import { useAuthStore } from "@/stores/auth-store";

vi.mock("@/features/preferences/appearance-controls", () => ({
  AppearanceControls: () => <div>外观设置内容</div>,
}));

vi.mock("@/features/auth/security", () => ({
  SecurityControls: () => <div>两步验证</div>,
}));

describe("Profile", () => {
  it("splits profile and appearance into accessible tabs", async () => {
    useAuthStore.setState({
      user: {
        id: "user-1",
        email: "ops@example.com",
        username: "ops",
        display_name: "Ops",
        is_active: true,
        permissions: ["tasks:read"],
      },
    });
    const user = userEvent.setup();
    render(<Profile />);

    expect(
      screen.getByRole("tab", { name: "资料" }).getAttribute("aria-selected"),
    ).toBe("true");
    await user.click(screen.getByRole("tab", { name: "外观" }));

    expect(
      screen.getByRole("tab", { name: "外观" }).getAttribute("aria-selected"),
    ).toBe("true");
    expect(screen.getByText("外观设置内容")).toBeTruthy();
  });
});
