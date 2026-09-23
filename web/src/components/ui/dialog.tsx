"use client";

import {
  DialogContent as PublicDialogContent,
  type DialogContentProps,
} from "@lotusrain-net/backend-infrastructure-web/ui/client";

export {
  Dialog,
  DialogClose,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogTitle,
  DialogTrigger,
} from "@lotusrain-net/backend-infrastructure-web/ui/client";

export function DialogContent(props: DialogContentProps) {
  return <PublicDialogContent closeLabel="关闭" {...props} />;
}
