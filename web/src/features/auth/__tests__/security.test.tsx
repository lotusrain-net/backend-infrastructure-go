import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Profile } from "@/features/auth/profile";
const save = vi.fn(),
  enroll = vi.fn(),
  confirm = vi.fn(),
  disable = vi.fn();
vi.mock("@/features/auth/security-api", () => ({
  useSecurityQuery: () => ({
    data: { mode: "default", totp_enabled: false, recovery_codes_remaining: 0 },
    isPending: false,
  }),
  useSecurityMutations: () => ({
    save: { mutateAsync: save },
    enroll: { mutateAsync: enroll, reset: vi.fn() },
    confirm: { mutateAsync: confirm, reset: vi.fn() },
    disable: { mutateAsync: disable, reset: vi.fn() },
  }),
}));
vi.mock("@/features/preferences/appearance-controls", () => ({
  AppearanceControls: () => null,
}));
describe("Profile two-step verification", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    enroll.mockResolvedValue({
      secret: "MANUALSECRET",
      otpauth_uri: "otpauth://totp/test?secret=MANUALSECRET",
      expires_in: 600,
    });
    confirm.mockResolvedValue({ recovery_codes: ["once-only-recovery"] });
  });
  it("embeds default selector in the profile card and confirms enrollment before showing recovery codes", async () => {
    const user = userEvent.setup();
    render(<Profile />);
    const selector = screen.getByRole("combobox", { name: "两步验证方式" });
    expect(selector.textContent).toContain("default");
    const card = screen.getByText("用户 ID").closest('[data-slot="card"]');
    expect(card).toBeTruthy();
    expect(within(card as HTMLElement).getByRole("combobox")).toBe(selector);
    await user.click(selector);
    await user.click(
      screen.getByRole("option", { name: "启用一次性代码二次验证" }),
    );
    await user.type(screen.getByLabelText("当前密码"), "correct-password");
    await user.click(screen.getByRole("button", { name: "开始绑定" }));
    expect(await screen.findByText("MANUALSECRET")).toBeTruthy();
    expect(
      screen.getByRole("img", { name: "身份验证器绑定二维码" }),
    ).toBeTruthy();
    await user.type(screen.getByLabelText("一次性代码"), "123456");
    await user.click(screen.getByRole("button", { name: "确认绑定" }));
    expect(await screen.findByText("once-only-recovery")).toBeTruthy();
    expect(screen.queryByText("MANUALSECRET")).toBeNull();
    await user.click(
      screen.getByRole("button", { name: "我已安全保存恢复码" }),
    );
    await waitFor(() =>
      expect(screen.queryByText("once-only-recovery")).toBeNull(),
    );
  });
});
