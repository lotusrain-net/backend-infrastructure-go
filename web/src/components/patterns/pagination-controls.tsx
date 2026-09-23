"use client";

import {
  PaginationControls as PublicPaginationControls,
  zhCNPatternMessages,
  type PaginationControlsProps,
} from "@lotusrain-net/backend-infrastructure-web/patterns";

export type {
  PageMeta,
  PaginationControlsProps,
} from "@lotusrain-net/backend-infrastructure-web/patterns";

export function PaginationControls(props: PaginationControlsProps) {
  return <PublicPaginationControls messages={zhCNPatternMessages} {...props} />;
}
