import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { RegisterForm } from "../register-form";
import { ApiError } from "@/lib/api/client";
const replace = vi.fn(),
  register = vi.fn().mockResolvedValue({ id: "new" });
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("../security-api", () => ({
  useRegisterMutation: () => ({
    mutateAsync: register,
    isPending: false,
    reset: vi.fn(),
  }),
  useEmailCodeMutation: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
    reset: vi.fn(),
  }),
}));
beforeEach(() => {
  replace.mockReset();
  register.mockReset().mockResolvedValue({ id: "new" });
});
it("returns to login after registration without requiring an optional email code", async () => {
  const user = userEvent.setup();
  render(<RegisterForm />);
  expect(screen.queryByLabelText("邮箱验证码")).toBeNull();
  await user.type(screen.getByLabelText("用户名"), "newuser");
  await user.type(screen.getByLabelText("邮箱"), "new@example.com");
  await user.type(screen.getByLabelText("密码"), "correct-password");
  await user.type(screen.getByLabelText("确认密码"), "correct-password");
  await user.click(screen.getByRole("button", { name: "注册" }));
  await waitFor(() =>
    expect(register).toHaveBeenCalledWith({
      username: "newuser",
      email: "new@example.com",
      password: "correct-password",
    }),
  );
  expect(replace).toHaveBeenCalledWith("/login?registered=1");
});

it("requests email verification in a dialog only when the server requires it", async () => {
  register.mockRejectedValueOnce(
    new ApiError(
      "validation failed",
      422,
      422,
      { email_code: "required" },
      { email_code: "required" },
    ),
  );
  const user = userEvent.setup();
  render(<RegisterForm />);
  await user.type(screen.getByLabelText("用户名"), "newuser");
  await user.type(screen.getByLabelText("邮箱"), "new@example.com");
  await user.type(screen.getByLabelText("密码"), "correct-password");
  await user.type(screen.getByLabelText("确认密码"), "correct-password");
  await user.click(screen.getByRole("button", { name: "注册" }));
  expect(await screen.findByRole("dialog")).toBeTruthy();
  expect(replace).not.toHaveBeenCalled();
  await user.type(screen.getByLabelText("邮箱验证码"), "123456");
  await user.click(screen.getByRole("button", { name: "验证并注册" }));
  expect(register).toHaveBeenLastCalledWith({
    username: "newuser",
    email: "new@example.com",
    password: "correct-password",
    email_code: "123456",
  });
  await waitFor(() =>
    expect(replace).toHaveBeenCalledWith("/login?registered=1"),
  );
});

it("discards a canceled verification code before registering with a changed email", async () => {
  register.mockRejectedValueOnce(
    new ApiError(
      "validation failed",
      422,
      422,
      { email_code: "required" },
      { email_code: "required" },
    ),
  );
  const user = userEvent.setup();
  render(<RegisterForm />);
  await user.type(screen.getByLabelText("用户名"), "newuser");
  await user.type(screen.getByLabelText("邮箱"), "new@example.com");
  await user.type(screen.getByLabelText("密码"), "correct-password");
  await user.type(screen.getByLabelText("确认密码"), "correct-password");
  await user.click(screen.getByRole("button", { name: "注册" }));
  await user.type(await screen.findByLabelText("邮箱验证码"), "123456");
  await user.click(screen.getByRole("button", { name: "返回注册" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.clear(screen.getByLabelText("邮箱"));
  await user.type(screen.getByLabelText("邮箱"), "changed@example.com");
  await user.click(screen.getByRole("button", { name: "注册" }));
  expect(register).toHaveBeenLastCalledWith({
    username: "newuser",
    email: "changed@example.com",
    password: "correct-password",
  });
});


it("validates password confirmation and preserves a safe next without sending confirmation", async () => {
  const user = userEvent.setup();
  render(<RegisterForm nextPath="/tasks?tab=active" />);
  await user.type(screen.getByLabelText("用户名"), "newuser");
  await user.type(screen.getByLabelText("邮箱"), "new@example.com");
  await user.type(screen.getByLabelText("密码"), "correct-password");
  await user.type(screen.getByLabelText("确认密码"), "wrong-password");
  await user.click(screen.getByRole("button", { name: "注册" }));
  expect(await screen.findByText("两次输入的密码不一致。")).toBeTruthy();
  expect(register).not.toHaveBeenCalled();
  await user.clear(screen.getByLabelText("确认密码"));
  await user.type(screen.getByLabelText("确认密码"), "correct-password");
  await user.click(screen.getByRole("button", { name: "注册" }));
  await waitFor(() => expect(replace).toHaveBeenCalledWith("/login?registered=1&next=%2Ftasks%3Ftab%3Dactive"));
  expect(register).toHaveBeenCalledWith({username:"newuser",email:"new@example.com",password:"correct-password"});
});

it("maps server fields to their inputs and unknown fields to the root alert", async () => {
  register.mockRejectedValueOnce(new ApiError("validation failed",422,422,{}, {username:"用户名不可用", unknown:"其他错误"}));
  const user=userEvent.setup();
  render(<RegisterForm nextPath="https://evil.example" />);
  expect(screen.getByRole("link",{name:"返回登录"}).getAttribute("href")).toBe("/login?next=%2Fdashboard");
  await user.type(screen.getByLabelText("用户名"), "newuser");
  await user.type(screen.getByLabelText("邮箱"), "new@example.com");
  await user.type(screen.getByLabelText("密码"), "correct-password");
  await user.type(screen.getByLabelText("确认密码"), "correct-password");
  await user.click(screen.getByRole("button",{name:"注册"}));
  expect(await screen.findByText("用户名不可用")).toBeTruthy();
  expect(screen.getByLabelText("用户名").getAttribute("aria-invalid")).toBe("true");
  expect(screen.getAllByRole("alert").map((item) => item.textContent).join(" ")).toContain("其他错误");
});
