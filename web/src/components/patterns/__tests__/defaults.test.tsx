import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ConfirmationDialog } from "@/components/patterns/confirmation-dialog";
import { ErrorState } from "@/components/patterns/error-state";
import { LoadingState } from "@/components/patterns/loading-state";
import { QueryState } from "@/components/patterns/query-state";

describe("canonical pattern defaults", () => {
  it("uses Chinese defaults for confirmation dialog", () => {
    render(<ConfirmationDialog open onOpenChange={vi.fn()} onConfirm={vi.fn()} />);

    expect(screen.getByRole("alertdialog")).toBeTruthy();
    expect(screen.getByText("确认操作")).toBeTruthy();
    expect(screen.getByText("请确认是否继续当前操作。")).toBeTruthy();
    expect(screen.getByRole("button", { name: "取消" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "确认" })).toBeTruthy();
  });

  it("supports a non-destructive confirmation action and keeps the dialog open after a failed async action", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const error = new Error("network unavailable");
    const onConfirm = vi.fn().mockRejectedValue(error);
    const onConfirmError = vi.fn();
    render(
      <ConfirmationDialog
        open
        onOpenChange={onOpenChange}
        onConfirm={onConfirm}
        onConfirmError={onConfirmError}
        confirmLabel="启用"
        confirmVariant="default"
      />,
    );

    const confirm = screen.getByRole("button", { name: "启用" });
    expect(confirm.className).toContain("--primary");
    expect(confirm.className).not.toContain("--destructive");

    await user.click(confirm);

    await waitFor(() => expect(onConfirm).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(onConfirmError).toHaveBeenCalledWith(error));
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(screen.getByRole("alertdialog")).toBeTruthy();
  });

  it("uses Chinese defaults for error and loading states", () => {
    const { rerender } = render(<ErrorState message="网络异常" />);
    expect(screen.getByRole("alert").textContent).toContain("请求失败");

    rerender(<LoadingState />);
    expect(screen.getByText("正在加载...")).toBeTruthy();
  });

  it("uses Chinese defaults for query state", () => {
    render(<QueryState state={{ page: 2, size: 20, search: "" }} total={45} />);

    expect(screen.getByText("第 2 页 · 每页 20 条")).toBeTruthy();
    expect(screen.getByText("共 45 条记录")).toBeTruthy();
  });
});
