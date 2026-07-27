import * as React from "react";
import { cn } from "@/lib/utils";

export const Textarea = React.forwardRef<HTMLTextAreaElement, React.TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className, ...props }, ref) => (
    <textarea
      ref={ref}
      className={cn(
        "min-h-24 w-full resize-y border border-[color:var(--input)] bg-[color:var(--input-background)] px-3 py-2 font-mono text-[length:var(--text-body-sm)] text-[color:var(--foreground)] outline-none [border-radius:var(--radius-md)] placeholder:text-[color:var(--muted-foreground)] focus-visible:border-[color:var(--ring)] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--ring),transparent_60%)] aria-[invalid=true]:border-[color:var(--destructive)] disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  ),
);

Textarea.displayName = "Textarea";
