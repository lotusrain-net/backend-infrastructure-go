"use client";

import * as CheckboxPrimitive from "@radix-ui/react-checkbox";
import { Check } from "lucide-react";
import * as React from "react";
import { cn } from "@/lib/utils";

export function Checkbox({ className, ...props }: React.ComponentProps<typeof CheckboxPrimitive.Root>) {
  return (
    <CheckboxPrimitive.Root
      className={cn("flex size-4 shrink-0 items-center justify-center border border-[color:var(--input)] bg-[color:var(--input-background)] text-[color:var(--primary-foreground)] shadow-sm [border-radius:var(--radius-sm)] outline-none transition-colors data-[state=checked]:border-[color:var(--primary)] data-[state=checked]:bg-[color:var(--primary)] focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] disabled:cursor-not-allowed disabled:opacity-50", className)}
      {...props}
    >
      <CheckboxPrimitive.Indicator className="flex items-center justify-center">
        <Check aria-hidden="true" className="size-3.5" />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}
