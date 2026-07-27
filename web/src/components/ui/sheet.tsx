"use client";

import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import * as React from "react";
import { cn } from "@/lib/utils";

export const Sheet = DialogPrimitive.Root;
export const SheetTrigger = DialogPrimitive.Trigger;
export const SheetClose = DialogPrimitive.Close;

export function SheetContent({ className, children, side = "right", ...props }: React.ComponentProps<typeof DialogPrimitive.Content> & { side?: "top" | "right" | "bottom" | "left" }) {
  const sideClasses = {
    right: "inset-y-0 right-0 h-full w-[min(22rem,86vw)] border-l",
    left: "inset-y-0 left-0 h-full w-[min(22rem,86vw)] border-r",
    top: "inset-x-0 top-0 w-full border-b",
    bottom: "inset-x-0 bottom-0 w-full border-t",
  }[side];
  return <DialogPrimitive.Portal><DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-[color:var(--overlay)]" /><DialogPrimitive.Content className={cn("fixed z-[60] flex flex-col border-[color:var(--border)] bg-[color:var(--popover)] text-[color:var(--popover-foreground)] shadow-[var(--shadow-dialog)] outline-none", sideClasses, className)} {...props}>{children}<DialogPrimitive.Close className="absolute right-4 top-4 inline-flex size-8 items-center justify-center [border-radius:var(--radius-md)] hover:bg-[color:var(--accent)] focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]"><X aria-hidden="true" className="size-4" /><span className="sr-only">关闭</span></DialogPrimitive.Close></DialogPrimitive.Content></DialogPrimitive.Portal>;
}

export function SheetHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) { return <div className={cn("flex flex-col gap-1.5 p-5 pr-12", className)} {...props} />; }
export function SheetFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) { return <div className={cn("mt-auto flex flex-col gap-2 p-5", className)} {...props} />; }
export function SheetTitle({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Title>) { return <DialogPrimitive.Title className={cn("font-semibold", className)} {...props} />; }
export function SheetDescription({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Description>) { return <DialogPrimitive.Description className={cn("text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]", className)} {...props} />; }
