import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { BasicAuthSettingsPage } from "../basic-auth";
import { useAuthStore } from "@/stores/auth-store";
const save = vi.fn();
const defaults = {
  password_login_enabled: true,
  registration_enabled: false,
  registration_email_verification_required: true,
  allowed_email_domains: [],
};
vi.mock("../security-api", () => ({
  useBasicAuthQuery: () => ({ data: defaults, isPending: false }),
  useSaveBasicAuthMutation: () => ({ mutateAsync: save, isPending: false }),
}));
beforeEach(() => {
  save.mockReset();
  save.mockResolvedValue(defaults);
  useAuthStore.setState({
    user: {
      id: "u",
      username: "admin",
      display_name: "Admin",
      email: "a@b.com",
      is_active: true,
      permissions: ["*"],
    },
  });
});
it("saves all four fields and keeps verification independent from registration", async () => {
  const user = userEvent.setup();
  render(<BasicAuthSettingsPage />);
  expect(
    (
      screen.getByRole("checkbox", {
        name: "注册时要求邮箱验证",
      }) as HTMLInputElement
    ).checked,
  ).toBe(true);
  expect(
    (screen.getByRole("checkbox", { name: "允许公开注册" }) as HTMLInputElement)
      .checked,
  ).toBe(false);
  await user.click(
    screen.getByRole("checkbox", { name: "注册时要求邮箱验证" }),
  );
  await user.type(
    screen.getByLabelText("允许的邮箱域名"),
    "Example.COM, work.test",
  );
  await user.click(screen.getByRole("button", { name: "保存设置" }));
  await waitFor(() =>
    expect(save).toHaveBeenCalledWith({
      ...defaults,
      registration_email_verification_required: false,
      allowed_email_domains: ["example.com", "work.test"],
    }),
  );
});
it("does not allow unprivileged users to edit settings", () => {
  useAuthStore.setState({ user: null });
  render(<BasicAuthSettingsPage />);
  expect(screen.getByRole("alert").textContent).toContain("没有查看");
  expect(screen.queryByRole("button", { name: "保存设置" })).toBeNull();
});
