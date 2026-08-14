"use client";

import {
  ManagementDialog as PublicManagementDialog,
  type ManagementDialogProps as PublicManagementDialogProps,
} from "@purplevoid/backend-infrastructure-web/patterns";

export type ManagementDialogProps = PublicManagementDialogProps;

export function ManagementDialog(props: ManagementDialogProps) {
  return <PublicManagementDialog closeLabel="关闭" {...props} />;
}
