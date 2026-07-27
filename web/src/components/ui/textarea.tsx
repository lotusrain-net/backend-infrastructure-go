import * as React from "react";
import { cn } from "@/lib/utils";

export function Textarea({ className, ...props }: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={cn(
        "min-h-24 w-full resize-y border border-[color:var(--border-strong)] bg-[color:var(--surface)] px-3 py-2 font-mono text-[length:var(--text-body-sm)] text-[color:var(--fg-default)] outline-none [border-radius:var(--radius-md)] placeholder:text-[color:var(--fg-muted)] focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)] disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}
