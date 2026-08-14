"use client";

import {
  DialogContent as PublicDialogContent,
  type DialogContentProps,
} from "@purplevoid/backend-infrastructure-web/ui/client";

export {
  Dialog,
  DialogClose,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogTitle,
  DialogTrigger,
} from "@purplevoid/backend-infrastructure-web/ui/client";

export function DialogContent(props: DialogContentProps) {
  return <PublicDialogContent closeLabel="关闭" {...props} />;
}
