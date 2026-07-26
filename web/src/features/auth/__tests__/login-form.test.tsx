import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LoginForm, resolveLoginDestination } from "@/features/auth/login-form";

const replace = vi.fn();
const mutateAsync = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

vi.mock("@/features/auth/api", () => ({
  useLoginMutation: () => ({ mutateAsync, isPending: false }),
}));

describe("LoginForm", () => {
  beforeEach(() => {
    replace.mockReset();
    mutateAsync.mockReset();
  });

  it("submits JSON-compatible credentials and returns to the requested internal page", async () => {
    mutateAsync.mockResolvedValue({ id: "u1" });
    const user = userEvent.setup();

    render(<LoginForm nextPath="/tasks" />);

    await user.type(screen.getByLabelText("邮箱"), "operator@example.com");
    await user.type(screen.getByLabelText("密码"), "correct horse battery staple");
    await user.click(screen.getByRole("button", { name: "登录" }));

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith({
        email: "operator@example.com",
        password: "correct horse battery staple",
      });
      expect(replace).toHaveBeenCalledWith("/tasks");
    });
  });

  it("rejects an external return URL", async () => {
    mutateAsync.mockResolvedValue({ id: "u1" });
    const user = userEvent.setup();

    render(<LoginForm nextPath="https://example.com" />);

    await user.type(screen.getByLabelText("邮箱"), "operator@example.com");
    await user.type(screen.getByLabelText("密码"), "correct horse battery staple");
    await user.click(screen.getByRole("button", { name: "登录" }));

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/dashboard"));
  });

  it("rejects decoded backslash and control-character return URLs", () => {
    expect(resolveLoginDestination("/\\evil.example")).toBe("/dashboard");
    expect(resolveLoginDestination("/%5Cevil.example")).toBe("/dashboard");
    expect(resolveLoginDestination("/tasks\nunsafe")).toBe("/dashboard");
  });
});
