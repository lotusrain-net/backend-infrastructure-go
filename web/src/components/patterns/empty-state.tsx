import { Inbox } from "lucide-react";
import { Card } from "@/components/ui/card";

interface EmptyStateProps {
  title?: string;
  description?: string;
}

export function EmptyState({
  title = "暂无数据",
  description = "当前筛选条件没有匹配的记录。",
}: EmptyStateProps) {
  return (
    <Card className="flex flex-col items-center gap-3 p-8 text-center shadow-none sm:p-8">
      <span className="bg-[color:var(--muted)] p-3 text-[color:var(--muted-foreground)] [border-radius:var(--radius-full)]">
        <Inbox className="h-5 w-5" />
      </span>
      <div className="space-y-1">
        <h3 className="text-[length:var(--text-heading-sm)] font-semibold">{title}</h3>
        <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">{description}</p>
      </div>
    </Card>
  );
}
