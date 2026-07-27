"use client";

import { useState, type MouseEvent } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import type { ButtonProps } from "@/components/ui/button";

interface ConfirmationDialogProps {
  title?: string;
  description?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void | Promise<void>;
  cancelLabel?: string;
  confirmLabel?: string;
  confirmVariant?: Extract<NonNullable<ButtonProps["variant"]>, "default" | "destructive">;
  pending?: boolean;
}

export function ConfirmationDialog({
  title = "确认操作",
  description = "请确认是否继续当前操作。",
  open,
  onOpenChange,
  onConfirm,
  cancelLabel = "取消",
  confirmLabel = "确认",
  confirmVariant = "destructive",
  pending = false,
}: ConfirmationDialogProps) {
  const [isConfirming, setIsConfirming] = useState(false);
  const isBusy = pending || isConfirming;

  async function handleConfirm(event: MouseEvent<HTMLButtonElement>) {
    event.preventDefault();
    if (isBusy) {
      return;
    }

    setIsConfirming(true);
    try {
      await onConfirm();
      onOpenChange(false);
    } catch {
      // The caller owns domain-specific feedback and the dialog remains open.
    } finally {
      setIsConfirming(false);
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={(nextOpen) => { if (!isBusy) onOpenChange(nextOpen); }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={isBusy}>{cancelLabel}</AlertDialogCancel>
          <AlertDialogAction variant={confirmVariant} disabled={isBusy} onClick={handleConfirm}>
            {isBusy ? "处理中..." : confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
