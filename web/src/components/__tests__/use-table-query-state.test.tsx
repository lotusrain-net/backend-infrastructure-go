import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { type TableQueryCodec, useTableQueryState } from "@/components/patterns/use-table-query-state";

const replace = vi.fn();
let query = "page=3&size=20&action=task.submit&unrelated=keep";

vi.mock("next/navigation", () => ({
  usePathname: () => "/audit",
  useRouter: () => ({ replace }),
  useSearchParams: () => new URLSearchParams(query),
}));

interface AuditTableState {
  page: number;
  size: number;
  action: string;
}

const codec: TableQueryCodec<AuditTableState> = {
  defaultState: { page: 1, size: 20, action: "" },
  keys: ["page", "size", "action"],
  resetPageOnChangeKeys: ["size", "action"],
  parse(params) {
    return {
      page: Number(params.get("page")) || 1,
      size: Number(params.get("size")) || 20,
      action: params.get("action") ?? "",
    };
  },
  serialize(state) {
    return {
      page: state.page === 1 ? undefined : String(state.page),
      size: state.size === 20 ? undefined : String(state.size),
      action: state.action || undefined,
    };
  },
};

describe("paginated table query state", () => {
  beforeEach(() => {
    query = "page=3&size=20&action=task.submit&unrelated=keep";
    replace.mockReset();
  });

  it("uses URL state, preserves unrelated parameters, and resets the page after filters change", () => {
    const { result } = renderHook(() => useTableQueryState(codec));

    expect(result.current.state).toEqual({ page: 3, size: 20, action: "task.submit" });

    act(() => result.current.setState({ action: "auth.login" }));
    expect(replace).toHaveBeenLastCalledWith("/audit?unrelated=keep&action=auth.login", { scroll: false });

    act(() => result.current.setState({ page: 4 }));
    expect(replace).toHaveBeenLastCalledWith("/audit?unrelated=keep&page=4&action=task.submit", { scroll: false });

    act(() => result.current.reset());
    expect(replace).toHaveBeenLastCalledWith("/audit?unrelated=keep", { scroll: false });
  });
});
