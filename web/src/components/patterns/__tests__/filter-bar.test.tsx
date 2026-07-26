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
});
