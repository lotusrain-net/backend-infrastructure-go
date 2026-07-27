import type { ReactNode } from "react";

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description: string;
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="max-w-3xl space-y-1">
        <h1 className="text-[length:var(--text-heading)] font-semibold text-[color:var(--foreground)]">{title}</h1>
        <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">{description}</p>
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  );
}
