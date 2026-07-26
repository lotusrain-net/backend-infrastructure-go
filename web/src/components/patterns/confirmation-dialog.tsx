"use client";

import * as Dialog from "@radix-ui/react-dialog";
import { Button } from "@/components/ui/button";

interface ConfirmationDialogProps {
  title?: string;
  description?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  cancelLabel?: string;
  confirmLabel?: string;
}

export function ConfirmationDialog({
  title = "确认操作",
  description = "请确认是否继续当前操作。",
  open,
  onOpenChange,
  onConfirm,
  cancelLabel = "取消",
  confirmLabel = "确认",
}: ConfirmationDialogProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/40 backdrop-blur-sm" />
        <Dialog.Content className="fixed left-1/2 top-1/2 w-[min(28rem,90vw)] -translate-x-1/2 -translate-y-1/2 border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-6 shadow-xl">
          <Dialog.Title className="text-xl font-semibold text-[color:var(--fg-default)]">
            {title}
          </Dialog.Title>
          <Dialog.Description className="mt-2 text-sm text-[color:var(--fg-muted)]">
            {description}
          </Dialog.Description>
          <div className="mt-6 flex justify-end gap-3">
            <Button variant="ghost" onClick={() => onOpenChange(false)}>
              {cancelLabel}
            </Button>
            <Button variant="danger" onClick={onConfirm}>
              {confirmLabel}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
