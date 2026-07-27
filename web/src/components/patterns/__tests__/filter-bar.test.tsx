import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { FilterBar } from "@/components/patterns/filter-bar";

function Harness({ onChange }: { onChange: (value: string) => void }) {
  const [keyword, setKeyword] = useState("");

  return (
    <>
      <output data-testid="keyword">{keyword}</output>
      <FilterBar
        keyword={keyword}
        pageSize={20}
        onKeywordChange={(value) => {
          setKeyword(value);
          onChange(value);
        }}
        onPageSizeChange={vi.fn()}
        onReset={vi.fn()}
      />
    </>
  );
}

describe("FilterBar", () => {
  it("emits keyword changes for the owning query state", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(<Harness onChange={onChange} />);

    await user.type(screen.getByLabelText("关键词"), "operator");

    expect(onChange).toHaveBeenLastCalledWith("operator");
    expect(screen.getByTestId("keyword").textContent).toBe("operator");
  });

  it("uses the shared accessible select for page size", async () => {
    const user = userEvent.setup();
    const onPageSizeChange = vi.fn();

    render(
      <FilterBar
        keyword=""
        pageSize={20}
        onKeywordChange={vi.fn()}
        onPageSizeChange={onPageSizeChange}
        onReset={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("combobox", { name: "每页数量" }));
    await user.click(screen.getByRole("option", { name: "50 条" }));

    expect(onPageSizeChange).toHaveBeenCalledWith(50);
  });

  it("keeps the page-size select compact instead of stretching across the filter bar", () => {
    render(
      <FilterBar
        keyword=""
        pageSize={20}
        onKeywordChange={vi.fn()}
        onPageSizeChange={vi.fn()}
        onReset={vi.fn()}
      />,
    );

    expect(screen.getByRole("combobox", { name: "每页数量" }).className).toContain("w-[6.5rem]");
  });
});
