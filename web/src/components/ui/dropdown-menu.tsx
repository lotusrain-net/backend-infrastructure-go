"use client";

import * as DropdownMenuPrimitive from "@radix-ui/react-dropdown-menu";
import { Check, ChevronRight } from "lucide-react";
import * as React from "react";
import { cn } from "@/lib/utils";

export const DropdownMenu = DropdownMenuPrimitive.Root;
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;
export const DropdownMenuGroup = DropdownMenuPrimitive.Group;
export const DropdownMenuPortal = DropdownMenuPrimitive.Portal;
export const DropdownMenuSub = DropdownMenuPrimitive.Sub;
export const DropdownMenuRadioGroup = DropdownMenuPrimitive.RadioGroup;

export function DropdownMenuContent({ className, sideOffset = 8, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.Content>) {
  return <DropdownMenuPrimitive.Portal><DropdownMenuPrimitive.Content sideOffset={sideOffset} className={cn("z-[80] min-w-48 border border-[color:var(--border)] bg-[color:var(--popover)] p-1 text-[color:var(--popover-foreground)] shadow-[var(--shadow-popover)] outline-none [border-radius:var(--radius-lg)]", className)} {...props} /></DropdownMenuPrimitive.Portal>;
}

export function DropdownMenuItem({ className, inset, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.Item> & { inset?: boolean }) {
  return <DropdownMenuPrimitive.Item className={cn("relative flex min-h-9 cursor-default select-none items-center gap-2 px-2 text-[length:var(--text-body-sm)] outline-none [border-radius:var(--radius-sm)] data-[highlighted]:bg-[color:var(--accent)] data-[highlighted]:text-[color:var(--accent-foreground)] data-[disabled]:pointer-events-none data-[disabled]:opacity-50", inset && "pl-8", className)} {...props} />;
}

export function DropdownMenuLabel({ className, inset, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.Label> & { inset?: boolean }) { return <DropdownMenuPrimitive.Label className={cn("px-2 py-1.5 text-[length:var(--text-caption)] font-medium", inset && "pl-8", className)} {...props} />; }
export function DropdownMenuSeparator({ className, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.Separator>) { return <DropdownMenuPrimitive.Separator className={cn("-mx-1 my-1 h-px bg-[color:var(--border)]", className)} {...props} />; }
export function DropdownMenuSubTrigger({ className, inset, children, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.SubTrigger> & { inset?: boolean }) { return <DropdownMenuPrimitive.SubTrigger className={cn("flex min-h-9 cursor-default items-center gap-2 px-2 text-[length:var(--text-body-sm)] outline-none [border-radius:var(--radius-sm)] data-[state=open]:bg-[color:var(--accent)] data-[state=open]:text-[color:var(--accent-foreground)] data-[highlighted]:bg-[color:var(--accent)] data-[highlighted]:text-[color:var(--accent-foreground)] focus:bg-[color:var(--accent)] focus:text-[color:var(--accent-foreground)]", inset && "pl-8", className)} {...props}>{children}<ChevronRight aria-hidden="true" className="ml-auto size-4" /></DropdownMenuPrimitive.SubTrigger>; }
export function DropdownMenuSubContent({ className, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.SubContent>) { return <DropdownMenuPrimitive.SubContent className={cn("z-[80] min-w-40 border border-[color:var(--border)] bg-[color:var(--popover)] p-1 shadow-[var(--shadow-popover)] outline-none [border-radius:var(--radius-lg)]", className)} {...props} />; }
export function DropdownMenuCheckboxItem({ className, children, checked, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.CheckboxItem>) { return <DropdownMenuPrimitive.CheckboxItem checked={checked} className={cn("relative flex min-h-9 cursor-default select-none items-center gap-2 py-1.5 pl-8 pr-2 text-[length:var(--text-body-sm)] outline-none [border-radius:var(--radius-sm)] data-[highlighted]:bg-[color:var(--accent)] data-[highlighted]:text-[color:var(--accent-foreground)]", className)} {...props}><span className="absolute left-2 flex size-4 items-center justify-center"><DropdownMenuPrimitive.ItemIndicator><Check aria-hidden="true" className="size-3.5" /></DropdownMenuPrimitive.ItemIndicator></span>{children}</DropdownMenuPrimitive.CheckboxItem>; }
export function DropdownMenuRadioItem({ className, children, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.RadioItem>) { return <DropdownMenuPrimitive.RadioItem className={cn("relative flex min-h-9 cursor-default select-none items-center gap-2 py-1.5 pl-8 pr-2 text-[length:var(--text-body-sm)] outline-none [border-radius:var(--radius-sm)] data-[highlighted]:bg-[color:var(--accent)] data-[highlighted]:text-[color:var(--accent-foreground)]", className)} {...props}><span className="absolute left-2 flex size-4 items-center justify-center"><DropdownMenuPrimitive.ItemIndicator><Check aria-hidden="true" className="size-3.5" /></DropdownMenuPrimitive.ItemIndicator></span>{children}</DropdownMenuPrimitive.RadioItem>; }
export const DropdownMenuShortcut = ({ className, ...props }: React.HTMLAttributes<HTMLSpanElement>) => <span className={cn("ml-auto text-[length:var(--text-caption)] tracking-wide text-[color:var(--muted-foreground)]", className)} {...props} />;
