import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EmptyState } from "@/components/patterns/empty-state";
import { ErrorState } from "@/components/patterns/error-state";

describe("shared empty and error states", () => {
  it("renders empty state copy", () => {
    render(<EmptyState />);

    expect(screen.getByText("暂无数据")).toBeTruthy();
    expect(screen.getByText("当前筛选条件没有匹配的记录。")).toBeTruthy();
  });

  it("renders an accessible error state", () => {
    render(<ErrorState message="审计服务暂时不可用。" />);

    expect(screen.getByRole("alert").textContent).toContain("审计服务暂时不可用。");
  });
});
