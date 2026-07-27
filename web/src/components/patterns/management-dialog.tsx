"use client";

import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";

interface ManagementDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  children: ReactNode;
}

export function ManagementDialog({ open, onOpenChange, title, description, children }: ManagementDialogProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/45" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 flex max-h-[min(44rem,90vh)] w-[min(36rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 flex-col border border-[color:var(--border-subtle)] bg-[color:var(--surface-raised)] shadow-[0_16px_40px_var(--shadow-color)] outline-none">
          <div className="flex items-start justify-between gap-4 border-b border-[color:var(--border-subtle)] px-5 py-4">
            <div className="space-y-1">
              <Dialog.Title className="text-base font-semibold text-[color:var(--fg-default)]">{title}</Dialog.Title>
              <Dialog.Description className="text-sm text-[color:var(--fg-muted)]">{description}</Dialog.Description>
            </div>
            <Dialog.Close asChild>
              <Button variant="ghost" size="icon" aria-label="关闭对话框" title="关闭对话框">
                <X aria-hidden="true" className="h-4 w-4" />
              </Button>
            </Dialog.Close>
          </div>
          <div className="min-h-0 overflow-y-auto p-5">{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
