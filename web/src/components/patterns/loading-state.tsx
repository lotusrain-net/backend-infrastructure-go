export function LoadingState({ label = "正在加载..." }: { label?: string }) {
  return (
    <div className="flex items-center gap-3 border border-[color:var(--border-subtle)] bg-[color:var(--surface)] px-4 py-3 text-sm text-[color:var(--fg-muted)]">
      <span className="h-3 w-3 animate-spin rounded-full border-2 border-[color:var(--focus-ring)] border-t-transparent" />
      <span>{label}</span>
    </div>
  );
}
