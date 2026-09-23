import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EmailCodeField as EmailCode } from "../email-code";
import { ApiError } from "@/lib/api/client";
const send = vi.fn();
vi.mock("../security-api", () => ({
  useEmailCodeMutation: () => ({
    mutateAsync: send,
    isPending: false,
    reset: vi.fn(),
  }),
}));
afterEach(() => vi.useRealTimers());
describe("controlled email code", () => {
  it("validates email, filters digits, prevents pending duplicates and counts down", async () => {
    vi.useFakeTimers();
    const change = vi.fn();
    let resolve!: (v: { resend_after_seconds: number }) => void;
    send.mockImplementation(
      () =>
        new Promise((r) => {
          resolve = r;
        }),
    );
    const { rerender } = render(
      <EmailCode email="bad" purpose="register" value="" onChange={change} />,
    );
    expect((screen.getByRole("button") as HTMLButtonElement).disabled).toBe(
      true,
    );
    rerender(
      <EmailCode
        email="a@example.com"
        purpose="register"
        value=""
        onChange={change}
      />,
    );
    fireEvent.change(screen.getByLabelText("邮箱验证码"), {
      target: { value: "a1234567" },
    });
    expect(change).toHaveBeenCalledWith("123456");
    fireEvent.click(screen.getByRole("button"));
    fireEvent.click(screen.getByRole("button"));
    expect(send).toHaveBeenCalledTimes(1);
    await act(async () => resolve({ resend_after_seconds: 60 }));
    expect(screen.getByRole("button").textContent).toBe("60 秒后重发");
    await act(async () => vi.advanceTimersByTime(1000));
    expect(screen.getByRole("button").textContent).toBe("59 秒后重发");
  });
});
