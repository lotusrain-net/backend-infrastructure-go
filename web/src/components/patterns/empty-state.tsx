import { Inbox } from "lucide-react";

interface EmptyStateProps {
  title?: string;
  description?: string;
}

export function EmptyState({
  title = "暂无数据",
  description = "当前筛选条件没有匹配的记录。",
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center gap-3 border border-[color:var(--border-subtle)] p-8 text-center [border-radius:var(--radius-lg)]">
      <span className="bg-[color:var(--surface-subtle)] p-3 text-[color:var(--fg-muted)] [border-radius:var(--radius-full)]">
        <Inbox className="h-5 w-5" />
      </span>
      <div className="space-y-1">
        <h3 className="text-[length:var(--text-heading-sm)] font-semibold text-[color:var(--fg-default)]">{title}</h3>
        <p className="text-[length:var(--text-body-sm)] text-[color:var(--fg-muted)]">{description}</p>
      </div>
    </div>
  );
}
