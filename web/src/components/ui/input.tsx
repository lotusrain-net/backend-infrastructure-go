import * as React from "react";
import { cn } from "@/lib/utils";

export function Input({ className, ...props }: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        "h-10 w-full border border-[color:var(--border-strong)] bg-[color:var(--surface)] px-3 text-[length:var(--text-body-sm)] text-[color:var(--fg-default)] outline-none [border-radius:var(--radius-md)] placeholder:text-[color:var(--fg-muted)] focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)] disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}
