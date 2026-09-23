import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LoginForm, resolveLoginDestination } from "@/features/auth/login-form";
import { ApiError } from "@/lib/api/client";

const replace = vi.fn();
const mutateAsync = vi.fn();
const verify = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

vi.mock("@/features/auth/api", () => ({
  useLoginMutation: () => ({ mutateAsync, isPending: false, reset: vi.fn() }),
}));

vi.mock("@/features/auth/security-api", () => ({
  useVerifyLoginMutation: () => ({
    mutateAsync: verify,
    isPending: false,
    reset: vi.fn(),
  }),
  useEmailCodeMutation: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
    reset: vi.fn(),
  }),
}));

describe("LoginForm", () => {
  beforeEach(() => {
    replace.mockReset();
    mutateAsync.mockReset();
    verify.mockReset();
  });

  it("submits JSON-compatible credentials and returns to the requested internal page", async () => {
    mutateAsync.mockResolvedValue({ id: "u1" });
    const user = userEvent.setup();

    render(<LoginForm nextPath="/tasks" />);
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.queryByLabelText("邮箱验证码")).toBeNull();

    await user.type(screen.getByLabelText("邮箱"), "operator@example.com");
    await user.type(
      screen.getByLabelText("密码"),
      "correct horse battery staple",
    );
    await user.click(screen.getByRole("button", { name: "登录" }));

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith({
        email: "operator@example.com",
        password: "correct horse battery staple",
      });
      expect(replace).toHaveBeenCalledWith("/tasks");
    });
  });

  it("keeps TOTP challenge on the login form without navigating", async () => {
    mutateAsync.mockResolvedValue({
      status: "totp_required",
      challenge_id: "challenge",
      expires_in: 300,
    });
    const user = userEvent.setup();
    render(<LoginForm />);
    await user.type(screen.getByLabelText("邮箱"), "operator@example.com");
    await user.type(screen.getByLabelText("密码"), "correct-password");
    await user.click(screen.getByRole("button", { name: "登录" }));
    expect(await screen.findByLabelText("一次性代码")).toBeTruthy();
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(replace).not.toHaveBeenCalled();
    await user.type(screen.getByLabelText("一次性代码"), "123456");
    await user.click(screen.getByRole("button", { name: "验证并登录" }));
    expect(verify).toHaveBeenCalledWith({
      challenge_id: "challenge",
      code: "123456",
    });
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/dashboard"));
  });

  it("opens email verification only when required and retries with the submitted credentials", async () => {
    mutateAsync
      .mockRejectedValueOnce(
        new ApiError(
          "validation failed",
          422,
          422,
          { email_code: "required" },
          { email_code: "required" },
        ),
      )
      .mockResolvedValue({ id: "u1" });
    const user = userEvent.setup();
    render(<LoginForm nextPath="/tasks" />);
    await user.type(screen.getByLabelText("邮箱"), "operator@example.com");
    await user.type(screen.getByLabelText("密码"), "correct-password");
    await user.click(screen.getByRole("button", { name: "登录" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(replace).not.toHaveBeenCalled();
    await user.type(screen.getByLabelText("邮箱验证码"), "123456");
    await user.click(screen.getByRole("button", { name: "验证并登录" }));
    expect(mutateAsync).toHaveBeenLastCalledWith({
      email: "operator@example.com",
      password: "correct-password",
      email_code: "123456",
    });
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/tasks"));
  });

  it("does not mistake an invalid password for a request for email verification", async () => {
    mutateAsync.mockRejectedValue(
      new ApiError("invalid credentials", 401, 401),
    );
    const user = userEvent.setup();
    render(<LoginForm />);
    await user.type(screen.getByLabelText("邮箱"), "operator@example.com");
    await user.type(screen.getByLabelText("密码"), "wrong-password");
    await user.click(screen.getByRole("button", { name: "登录" }));
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("rejects an external return URL", async () => {
    mutateAsync.mockResolvedValue({ id: "u1" });
    const user = userEvent.setup();

    render(<LoginForm nextPath="https://example.com" />);

    await user.type(screen.getByLabelText("邮箱"), "operator@example.com");
    await user.type(
      screen.getByLabelText("密码"),
      "correct horse battery staple",
    );
    await user.click(screen.getByRole("button", { name: "登录" }));

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/dashboard"));
  });

  it("rejects decoded backslash and control-character return URLs", () => {
    expect(resolveLoginDestination("/\\evil.example")).toBe("/dashboard");
    expect(resolveLoginDestination("/%5Cevil.example")).toBe("/dashboard");
    expect(resolveLoginDestination("/tasks\nunsafe")).toBe("/dashboard");
  });
});
