import type { TableQueryState } from "@/components/patterns/use-table-query-state";

export function QueryState({ state, total }: { state: TableQueryState; total: number }) {
  return (
    <div className="flex items-center justify-between border border-[color:var(--border-subtle)] bg-[color:var(--surface)] px-4 py-3 text-sm text-[color:var(--fg-muted)]">
      <span>
        第 {state.page} 页 · 每页 {state.size} 条
      </span>
      <span>共 {total} 条记录</span>
    </div>
  );
}
