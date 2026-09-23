"use client";

import {
  ConfirmationDialog as PublicConfirmationDialog,
  zhCNPatternMessages,
  type ConfirmationDialogProps as PublicConfirmationDialogProps,
} from "@lotusrain-net/backend-infrastructure-web/patterns";

export type ConfirmationDialogProps = PublicConfirmationDialogProps;

export function ConfirmationDialog(props: ConfirmationDialogProps) {
  return (
    <PublicConfirmationDialog
      messages={zhCNPatternMessages}
      title="确认操作"
      description="请确认是否继续当前操作。"
      {...props}
    />
  );
}
