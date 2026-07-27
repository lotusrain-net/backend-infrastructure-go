"use client";

import * as AlertDialogPrimitive from "@radix-ui/react-alert-dialog";
import * as React from "react";
import { buttonVariants, type ButtonProps } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export const AlertDialog = AlertDialogPrimitive.Root;
export const AlertDialogTrigger = AlertDialogPrimitive.Trigger;
type AlertDialogActionProps = React.ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Action> & {
  variant?: NonNullable<ButtonProps["variant"]>;
};

export const AlertDialogAction = React.forwardRef<
  React.ElementRef<typeof AlertDialogPrimitive.Action>,
  AlertDialogActionProps
>(({ className, variant = "destructive", ...props }, ref) => (
  <AlertDialogPrimitive.Action
    ref={ref}
    className={cn(buttonVariants({ variant }), className)}
    {...props}
  />
));
AlertDialogAction.displayName = AlertDialogPrimitive.Action.displayName;
export const AlertDialogCancel = React.forwardRef<React.ElementRef<typeof AlertDialogPrimitive.Cancel>, React.ComponentPropsWithoutRef<typeof AlertDialogPrimitive.Cancel>>(({ className, ...props }, ref) => <AlertDialogPrimitive.Cancel ref={ref} className={cn(buttonVariants({ variant: "outline" }), className)} {...props} />);
AlertDialogCancel.displayName = AlertDialogPrimitive.Cancel.displayName;

export function AlertDialogContent({ className, children, ...props }: React.ComponentProps<typeof AlertDialogPrimitive.Content>) {
  return <AlertDialogPrimitive.Portal><AlertDialogPrimitive.Overlay className="fixed inset-0 z-50 bg-[color:var(--overlay)] backdrop-blur-[1px]" /><AlertDialogPrimitive.Content className={cn("fixed left-1/2 top-1/2 z-[60] w-[min(30rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 border border-[color:var(--border)] bg-[color:var(--popover)] p-5 text-[color:var(--popover-foreground)] shadow-[var(--shadow-dialog)] outline-none [border-radius:var(--radius-xl)]", className)} {...props}>{children}</AlertDialogPrimitive.Content></AlertDialogPrimitive.Portal>;
}

export function AlertDialogHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) { return <div className={cn("grid gap-1.5", className)} {...props} />; }
export function AlertDialogFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) { return <div className={cn("mt-6 flex flex-wrap justify-end gap-2", className)} {...props} />; }
export function AlertDialogTitle({ className, ...props }: React.ComponentProps<typeof AlertDialogPrimitive.Title>) { return <AlertDialogPrimitive.Title className={cn("text-[length:var(--text-heading)] font-semibold", className)} {...props} />; }
export function AlertDialogDescription({ className, ...props }: React.ComponentProps<typeof AlertDialogPrimitive.Description>) { return <AlertDialogPrimitive.Description className={cn("text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]", className)} {...props} />; }
