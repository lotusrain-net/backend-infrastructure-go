import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { useTableQueryState } from "@/components/patterns/use-table-query-state";

function Harness() {
  const { state, setPage, setSize, setSearch, reset } = useTableQueryState(
    new URLSearchParams("page=3&size=20&search=alice"),
  );

  return (
    <div>
      <output data-testid="state">{JSON.stringify(state)}</output>
      <button onClick={() => setSearch("bob")}>search</button>
      <button onClick={() => setPage(5)}>page</button>
      <button onClick={() => setSize(50)}>size</button>
      <button onClick={reset}>reset</button>
    </div>
  );
}

describe("paginated table query state", () => {
  it("preserves and updates page, size, and search state", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    expect(screen.getByTestId("state").textContent).toBe('{"page":3,"size":20,"search":"alice"}');

    await user.click(screen.getByRole("button", { name: "search" }));
    expect(screen.getByTestId("state").textContent).toBe('{"page":1,"size":20,"search":"bob"}');

    await user.click(screen.getByRole("button", { name: "size" }));
    expect(screen.getByTestId("state").textContent).toBe('{"page":1,"size":50,"search":"bob"}');

    await user.click(screen.getByRole("button", { name: "page" }));
    expect(screen.getByTestId("state").textContent).toBe('{"page":5,"size":50,"search":"bob"}');

    await user.click(screen.getByRole("button", { name: "reset" }));
    expect(screen.getByTestId("state").textContent).toBe('{"page":1,"size":10,"search":""}');
  });
});
