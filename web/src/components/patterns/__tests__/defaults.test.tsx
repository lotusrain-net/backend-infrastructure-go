import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ConfirmationDialog } from "@/components/patterns/confirmation-dialog";
import { ErrorState } from "@/components/patterns/error-state";
import { LoadingState } from "@/components/patterns/loading-state";
import { QueryState } from "@/components/patterns/query-state";

describe("canonical pattern defaults", () => {
  it("uses Chinese defaults for confirmation dialog", () => {
    render(<ConfirmationDialog open onOpenChange={vi.fn()} onConfirm={vi.fn()} />);

    expect(screen.getByText("确认操作")).toBeTruthy();
    expect(screen.getByText("请确认是否继续当前操作。")).toBeTruthy();
    expect(screen.getByRole("button", { name: "取消" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "确认" })).toBeTruthy();
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
