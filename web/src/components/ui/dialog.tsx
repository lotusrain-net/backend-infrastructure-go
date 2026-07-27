"use client";

import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import * as React from "react";
import { cn } from "@/lib/utils";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

export function DialogOverlay({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return <DialogPrimitive.Overlay className={cn("fixed inset-0 z-50 bg-[color:var(--overlay)] backdrop-blur-[1px] data-[state=closed]:animate-out data-[state=open]:animate-in", className)} {...props} />;
}

export function DialogContent({ className, children, showClose = true, ...props }: React.ComponentProps<typeof DialogPrimitive.Content> & { showClose?: boolean }) {
  return (
    <DialogPrimitive.Portal>
      <DialogOverlay />
      <DialogPrimitive.Content className={cn("fixed left-1/2 top-1/2 z-[60] flex max-h-[min(44rem,calc(100vh-2rem))] w-[min(36rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 flex-col border border-[color:var(--border)] bg-[color:var(--popover)] text-[color:var(--popover-foreground)] shadow-[var(--shadow-dialog)] outline-none [border-radius:var(--radius-xl)] data-[state=closed]:animate-out data-[state=open]:animate-in", className)} {...props}>
        {children}
        {showClose ? <DialogPrimitive.Close className="absolute right-4 top-4 inline-flex size-8 items-center justify-center text-[color:var(--muted-foreground)] [border-radius:var(--radius-md)] outline-none hover:bg-[color:var(--accent)] hover:text-[color:var(--accent-foreground)] focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]"><X aria-hidden="true" className="size-4" /><span className="sr-only">关闭</span></DialogPrimitive.Close> : null}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

export function DialogHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("flex flex-col gap-1 border-b border-[color:var(--border)] px-5 py-4 pr-12", className)} {...props} />;
}

export function DialogFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("flex flex-wrap items-center justify-end gap-2 border-t border-[color:var(--border)] px-5 py-4", className)} {...props} />;
}

export function DialogTitle({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return <DialogPrimitive.Title className={cn("text-[length:var(--text-heading)] font-semibold", className)} {...props} />;
}

export function DialogDescription({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return <DialogPrimitive.Description className={cn("text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]", className)} {...props} />;
}
