import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

const badgeVariants = cva("inline-flex items-center border px-2 py-0.5 text-xs font-medium", {
  variants: {
    variant: {
      neutral: "border-[color:var(--border-strong)] bg-[color:var(--surface-subtle)] text-[color:var(--fg-muted)]",
      info: "border-[color:var(--accent-primary)]/40 bg-[color:var(--accent-primary-subtle)] text-[color:var(--accent-primary)]",
      success: "border-[color:var(--success)]/45 bg-[color:var(--success-subtle)] text-[color:var(--success)]",
      warning: "border-[color:var(--warning)]/45 bg-[color:var(--warning-subtle)] text-[color:var(--warning)]",
      danger: "border-[color:var(--danger)]/45 bg-[color:var(--danger-subtle)] text-[color:var(--danger)]",
    },
  },
  defaultVariants: { variant: "neutral" },
});

export function Badge({ className, variant, ...props }: HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />;
}
