"use client";

import * as SelectPrimitive from "@radix-ui/react-select";
import { Check, ChevronDown, ChevronUp } from "lucide-react";
import * as React from "react";
import { cn } from "@/lib/utils";

export const Select = SelectPrimitive.Root;
export const SelectGroup = SelectPrimitive.Group;
export const SelectValue = SelectPrimitive.Value;

export function SelectTrigger({ className, children, ...props }: React.ComponentProps<typeof SelectPrimitive.Trigger>) {
  return (
    <SelectPrimitive.Trigger
      className={cn("flex h-10 w-full items-center justify-between gap-2 border border-[color:var(--input)] bg-[color:var(--input-background)] px-3 text-left text-[length:var(--text-body-sm)] text-[color:var(--foreground)] outline-none [border-radius:var(--radius-md)] data-[placeholder]:text-[color:var(--muted-foreground)] focus-visible:border-[color:var(--ring)] focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] disabled:cursor-not-allowed disabled:opacity-50", className)}
      {...props}
    >
      {children}
      <SelectPrimitive.Icon asChild><ChevronDown aria-hidden="true" className="size-4 shrink-0 text-[color:var(--muted-foreground)]" /></SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
  );
}

export function SelectContent({ className, children, position = "popper", ...props }: React.ComponentProps<typeof SelectPrimitive.Content>) {
  return (
    <SelectPrimitive.Portal>
      <SelectPrimitive.Content
        position={position}
        className={cn("z-[80] max-h-72 min-w-36 overflow-hidden border border-[color:var(--border)] bg-[color:var(--popover)] text-[color:var(--popover-foreground)] shadow-[var(--shadow-popover)] [border-radius:var(--radius-lg)]", className)}
        {...props}
      >
        <SelectScrollUpButton />
        <SelectPrimitive.Viewport className={cn("p-1", position === "popper" && "w-[var(--radix-select-trigger-width)] min-w-[var(--radix-select-trigger-width)]")}>{children}</SelectPrimitive.Viewport>
        <SelectScrollDownButton />
      </SelectPrimitive.Content>
    </SelectPrimitive.Portal>
  );
}

export function SelectLabel({ className, ...props }: React.ComponentProps<typeof SelectPrimitive.Label>) {
  return <SelectPrimitive.Label className={cn("px-2 py-1.5 text-[length:var(--text-caption)] font-medium text-[color:var(--muted-foreground)]", className)} {...props} />;
}

export function SelectItem({ className, children, ...props }: React.ComponentProps<typeof SelectPrimitive.Item>) {
  return (
    <SelectPrimitive.Item
      className={cn("relative flex min-h-9 w-full cursor-default select-none items-center gap-2 py-1.5 pl-2 pr-8 text-[length:var(--text-body-sm)] outline-none [border-radius:var(--radius-sm)] data-[highlighted]:bg-[color:var(--accent)] data-[highlighted]:text-[color:var(--accent-foreground)] data-[state=checked]:bg-[color:var(--primary-subtle)] data-[state=checked]:text-[color:var(--primary-subtle-foreground)] data-[disabled]:pointer-events-none data-[disabled]:opacity-50", className)}
      {...props}
    >
      <span className="absolute right-2 flex size-4 items-center justify-center"><SelectPrimitive.ItemIndicator><Check aria-hidden="true" className="size-3.5" /></SelectPrimitive.ItemIndicator></span>
      <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
    </SelectPrimitive.Item>
  );
}

export function SelectSeparator({ className, ...props }: React.ComponentProps<typeof SelectPrimitive.Separator>) {
  return <SelectPrimitive.Separator className={cn("-mx-1 my-1 h-px bg-[color:var(--border)]", className)} {...props} />;
}

export function SelectScrollUpButton({ className, ...props }: React.ComponentProps<typeof SelectPrimitive.ScrollUpButton>) {
  return <SelectPrimitive.ScrollUpButton className={cn("flex h-7 cursor-default items-center justify-center", className)} {...props}><ChevronUp aria-hidden="true" className="size-4" /></SelectPrimitive.ScrollUpButton>;
}

export function SelectScrollDownButton({ className, ...props }: React.ComponentProps<typeof SelectPrimitive.ScrollDownButton>) {
  return <SelectPrimitive.ScrollDownButton className={cn("flex h-7 cursor-default items-center justify-center", className)} {...props}><ChevronDown aria-hidden="true" className="size-4" /></SelectPrimitive.ScrollDownButton>;
}
