import * as React from "react";
import { cn } from "@/lib/utils";

export function Card({ className, ...props }: React.HTMLAttributes<HTMLElement>) {
  return (
    <section
      data-slot="card"
      className={cn("border border-[color:var(--border)] bg-[color:var(--card)] p-5 text-[color:var(--card-foreground)] shadow-[var(--shadow-card)] [border-radius:var(--radius-lg)] sm:p-6", className)}
      {...props}
    />
  );
}

export function CardHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div data-slot="card-header" className={cn("flex flex-col gap-1.5 px-5 pt-5 sm:px-6 sm:pt-6", className)} {...props} />;
}

export function CardTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  return <h2 data-slot="card-title" className={cn("text-[length:var(--text-heading-sm)] font-semibold text-[color:var(--card-foreground)]", className)} {...props} />;
}

export function CardDescription({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <p data-slot="card-description" className={cn("text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]", className)} {...props} />;
}

export function CardContent({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div data-slot="card-content" className={cn("p-5 sm:p-6", className)} {...props} />;
}

export function CardFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div data-slot="card-footer" className={cn("flex items-center gap-3 border-t border-[color:var(--border)] p-5 sm:p-6", className)} {...props} />;
}
