import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { VerificationDialog } from "../verification-dialog";
import { ApiError } from "@/lib/api/client";

vi.mock("../security-api", () => ({
  useEmailCodeMutation: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
    reset: vi.fn(),
  }),
}));

it.each(["email", "totp"] as const)(
  "uses the same six-digit validation and retry behavior for %s",
  async (method) => {
    const user = userEvent.setup();
    const verify = vi
      .fn()
      .mockRejectedValueOnce(new ApiError("invalid credentials", 401, 401))
      .mockResolvedValue(undefined);
    render(
      <VerificationDialog
        {...(method === "email"
          ? { method, email: "a@example.com" }
          : { method })}
        purpose="login"
        onVerify={verify}
        onCancel={vi.fn()}
      />,
    );
    const code = screen.getByLabelText(
      method === "email" ? "邮箱验证码" : "一次性代码",
    );
    const submit = screen.getByRole("button", { name: "验证并登录" });
    expect((submit as HTMLButtonElement).disabled).toBe(true);
    await user.type(code, "a12345");
    expect((code as HTMLInputElement).value).toBe("12345");
    expect((submit as HTMLButtonElement).disabled).toBe(true);
    await user.type(code, "67");
    expect((code as HTMLInputElement).value).toBe("123456");
    await user.click(submit);
    expect((await screen.findByRole("alert")).textContent).toContain(
      "验证码无效或已过期",
    );
    expect(screen.getByRole("dialog")).toBeTruthy();
    await user.clear(code);
    await user.type(code, "654321");
    await user.click(submit);
    expect(verify).toHaveBeenLastCalledWith({ code: "654321" });
  },
);

it("clears the input when switching to recovery and blocks dismissal and duplicate submissions while pending", async () => {
  const user = userEvent.setup();
  let resolve!: () => void;
  const verify = vi.fn(
    () =>
      new Promise<void>((done) => {
        resolve = done;
      }),
  );
  const cancel = vi.fn();
  render(
    <VerificationDialog
      method="totp"
      purpose="login"
      onVerify={verify}
      onCancel={cancel}
    />,
  );
  await user.type(screen.getByLabelText("一次性代码"), "123456");
  await user.click(screen.getByRole("button", { name: "使用恢复码" }));
  expect((screen.getByLabelText("恢复码") as HTMLInputElement).value).toBe("");
  await user.type(screen.getByLabelText("恢复码"), "abc-def");
  await user.click(screen.getByRole("button", { name: "验证并登录" }));
  fireEvent.submit(screen.getByLabelText("恢复码").closest("form")!);
  await user.keyboard("{Escape}");
  expect(cancel).not.toHaveBeenCalled();
  expect(verify).toHaveBeenCalledTimes(1);
  expect(verify).toHaveBeenCalledWith({ recovery_code: "abc-def" });
  expect(
    (screen.getByRole("button", { name: "返回登录" }) as HTMLButtonElement)
      .disabled,
  ).toBe(true);
  await act(async () => resolve());
});
