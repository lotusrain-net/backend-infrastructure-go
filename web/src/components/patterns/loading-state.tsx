import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export function LoadingState({ label = "正在加载..." }: { label?: string }) {
  return (
    <Card className="flex items-center gap-3 px-4 py-3 text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)] shadow-none sm:px-4 sm:py-3">
      <Skeleton className="size-3 [border-radius:var(--radius-full)]" />
      <span>{label}</span>
    </Card>
  );
}
