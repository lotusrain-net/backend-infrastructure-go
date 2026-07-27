import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

const badgeVariants = cva("inline-flex w-fit items-center gap-1 border px-2 py-0.5 text-[length:var(--text-caption)] font-medium [border-radius:var(--radius-sm)]", {
  variants: {
    variant: {
      neutral: "border-[color:var(--input)] bg-[color:var(--muted)] text-[color:var(--muted-foreground)]",
      info: "border-[color:var(--primary)] bg-[color:var(--primary-subtle)] text-[color:var(--primary-subtle-foreground)]",
      success: "border-[color:var(--success)] bg-[color:var(--success-subtle)] text-[color:var(--success-subtle-foreground)]",
      warning: "border-[color:var(--warning)] bg-[color:var(--warning-subtle)] text-[color:var(--warning-subtle-foreground)]",
      danger: "border-[color:var(--destructive)] bg-[color:var(--destructive-subtle)] text-[color:var(--destructive-subtle-foreground)]",
    },
  },
  defaultVariants: { variant: "neutral" },
});

export function Badge({ className, variant, ...props }: HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />;
}
