"use client";

import * as PopoverPrimitive from "@radix-ui/react-popover";
import * as React from "react";
import { cn } from "@/lib/utils";

export const Popover = PopoverPrimitive.Root;
export const PopoverTrigger = PopoverPrimitive.Trigger;
export const PopoverAnchor = PopoverPrimitive.Anchor;
export function PopoverContent({ className, align = "center", sideOffset = 8, ...props }: React.ComponentProps<typeof PopoverPrimitive.Content>) {
  return <PopoverPrimitive.Portal><PopoverPrimitive.Content align={align} sideOffset={sideOffset} className={cn("z-[80] w-72 border border-[color:var(--border)] bg-[color:var(--popover)] p-4 text-[color:var(--popover-foreground)] shadow-[var(--shadow-popover)] outline-none [border-radius:var(--radius-lg)]", className)} {...props} /></PopoverPrimitive.Portal>;
}
