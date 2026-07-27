"use client";

import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import * as React from "react";
import { cn } from "@/lib/utils";

export const TooltipProvider = TooltipPrimitive.Provider;
export const Tooltip = TooltipPrimitive.Root;
export const TooltipTrigger = TooltipPrimitive.Trigger;
export function TooltipContent({ className, sideOffset = 6, ...props }: React.ComponentProps<typeof TooltipPrimitive.Content>) {
  return <TooltipPrimitive.Portal><TooltipPrimitive.Content sideOffset={sideOffset} className={cn("z-[100] max-w-64 bg-[color:var(--foreground)] px-2 py-1 text-[length:var(--text-caption)] text-[color:var(--background)] shadow-[var(--shadow-popover)] [border-radius:var(--radius-sm)]", className)} {...props} /></TooltipPrimitive.Portal>;
}
