import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Profile } from "@/features/auth/profile";
import { useAuthStore } from "@/stores/auth-store";
const verifyEmail = vi.fn(), send = vi.fn();
const save = vi.fn(),
  enroll = vi.fn(),
  confirm = vi.fn(),
  disable = vi.fn();
vi.mock("@/features/auth/security-api", () => ({
  useEmailCodeMutation: () => ({mutateAsync:send, reset:vi.fn(), isPending:false}),
  useSecurityQuery: () => ({
    data: { mode: "default", totp_enabled: false, recovery_codes_remaining: 0 },
    isPending: false,
  }),
  useSecurityMutations: () => ({
    save: { mutateAsync: save },
    verifyEmail: {mutateAsync:verifyEmail, reset:vi.fn()},
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

it("verifies the current mailbox before enabling email two-step verification", async () => {
  const user = userEvent.setup();
  useAuthStore.setState({user:{id:"user", email:"real@example.com", username:"real", display_name:"Real", is_active:true, permissions:[], email_verified_at:null}});
  verifyEmail.mockResolvedValue({email_verified_at:"2026-09-23T00:00:00Z"});
  send.mockResolvedValue({resend_after_seconds:60});
  render(<Profile />);
  await user.click(screen.getByRole("combobox", {name:"两步验证方式"}));
  await user.click(screen.getByRole("option", {name:"启用邮箱二次验证"}));
  expect(save).not.toHaveBeenCalled();
  expect(screen.getByText(/请先验证/)).toBeTruthy();
  await user.click(screen.getByRole("button", {name:"发送验证码"}));
  expect(send).toHaveBeenCalledWith({email:"real@example.com", purpose:"verify_email"});
  await user.type(screen.getByLabelText("邮箱验证码"), "123456");
  await user.click(screen.getByRole("button", {name:"验证并启用邮箱二次验证"}));
  await waitFor(() => expect(verifyEmail).toHaveBeenCalledWith("123456"));
  await waitFor(() => expect(save).toHaveBeenCalledWith("email"));
});

it("keeps email verification retryable after an invalid code and never saves the policy", async () => {
  vi.clearAllMocks();
  const user = userEvent.setup();
  useAuthStore.setState({user:{id:"user", email:"real@example.com", username:"real", display_name:"Real", is_active:true, permissions:[], email_verified_at:null}});
  verifyEmail.mockRejectedValueOnce(new Error("验证码不正确或已过期，请重新获取后再试。"));
  render(<Profile />);
  await user.click(screen.getByRole("combobox", {name:"两步验证方式"}));
  await user.click(screen.getByRole("option", {name:"启用邮箱二次验证"}));
  await user.type(screen.getByLabelText("邮箱验证码"), "123456");
  await user.click(screen.getByRole("button", {name:"验证并启用邮箱二次验证"}));
  expect(await screen.findByText("验证码不正确或已过期，请重新获取后再试。")).toBeTruthy();
  expect(save).not.toHaveBeenCalled();
  expect((screen.getByLabelText("邮箱验证码") as HTMLInputElement).value).toBe("");
  expect(screen.getByRole("button", {name:"发送验证码"})).toBeTruthy();
});
