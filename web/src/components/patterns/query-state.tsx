import type { TableQueryState } from "@/components/patterns/use-table-query-state";
import { Card } from "@/components/ui/card";

export function QueryState({ state, total }: { state: TableQueryState; total: number }) {
  return (
    <Card className="flex items-center justify-between px-4 py-3 text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)] shadow-none sm:px-4 sm:py-3">
      <span>
        第 {state.page} 页 · 每页 {state.size} 条
      </span>
      <span>共 {total} 条记录</span>
    </Card>
  );
}
