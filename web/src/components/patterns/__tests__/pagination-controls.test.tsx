import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PaginationControls } from "@/components/patterns/pagination-controls";

describe("PaginationControls", () => {
  it("renders every page when the result has seven or fewer pages", () => {
    render(
      <PaginationControls
        page={{ page: 4, size: 10, total: 70, pages: 7, has_next: true, has_prev: true }}
        onPageChange={vi.fn()}
      />,
    );

    for (let page = 1; page <= 7; page += 1) {
      expect(screen.getByRole("button", { name: `第 ${page} 页` })).toBeTruthy();
    }
    expect(screen.queryByText("更多页面")).toBeNull();
    expect(screen.getByRole("button", { name: "第 4 页" }).getAttribute("aria-current")).toBe("page");
  });

  it("uses first, last, adjacent pages, and ellipses around a large result", () => {
    render(
      <PaginationControls
        page={{ page: 5, size: 10, total: 100, pages: 10, has_next: true, has_prev: true }}
        onPageChange={vi.fn()}
      />,
    );

    [1, 4, 5, 6, 10].forEach((page) => expect(screen.getByRole("button", { name: `第 ${page} 页` })).toBeTruthy());
    [2, 3, 7, 8, 9].forEach((page) => expect(screen.queryByRole("button", { name: `第 ${page} 页` })).toBeNull());
    expect(screen.getAllByText("更多页面")).toHaveLength(2);
  });

  it("computes every navigation target from the bounded current page", async () => {
    const user = userEvent.setup();
    const onPageChange = vi.fn();

    render(
      <PaginationControls
        page={{ page: 99, size: 10, total: 40, pages: 4, has_next: false, has_prev: true }}
        onPageChange={onPageChange}
      />,
    );

    expect(screen.getByRole("button", { name: "第 4 页" }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("button", { name: "下一页" }).hasAttribute("disabled")).toBe(true);

    await user.click(screen.getByRole("button", { name: "首页" }));
    await user.click(screen.getByRole("button", { name: "上一页" }));
    await user.click(screen.getByRole("button", { name: "第 2 页" }));

    expect(onPageChange.mock.calls).toEqual([[1], [3], [2]]);
  });

  it("disables every navigation control for an empty result", () => {
    render(
      <PaginationControls
        page={{ page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false }}
        onPageChange={vi.fn()}
      />,
    );

    ["首页", "上一页", "下一页", "末页"].forEach((label) => {
      expect(screen.getByRole("button", { name: label }).hasAttribute("disabled")).toBe(true);
    });
    expect(screen.queryByRole("button", { name: /第 \d+ 页/ })).toBeNull();
  });
});
