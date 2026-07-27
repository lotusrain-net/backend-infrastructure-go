import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const alertVariants = cva("relative grid w-full grid-cols-[auto_1fr] gap-x-3 gap-y-1 border px-4 py-3 text-[length:var(--text-body-sm)] [border-radius:var(--radius-lg)]", {
  variants: {
    variant: {
      default: "border-[color:var(--border)] bg-[color:var(--card)] text-[color:var(--card-foreground)]",
      destructive: "border-[color:var(--destructive)] bg-[color:var(--destructive-subtle)] text-[color:var(--destructive-subtle-foreground)]",
      success: "border-[color:var(--success)] bg-[color:var(--success-subtle)] text-[color:var(--success-subtle-foreground)]",
    },
  },
  defaultVariants: { variant: "default" },
});

export function Alert({ className, variant, ...props }: React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof alertVariants>) {
  return <div role="alert" className={cn(alertVariants({ variant }), className)} {...props} />;
}

export function AlertTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  return <h3 className={cn("col-start-2 font-semibold", className)} {...props} />;
}

export function AlertDescription({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("col-start-2 leading-relaxed opacity-90", className)} {...props} />;
}
