"use client";

import {
  PaginationControls as PublicPaginationControls,
  zhCNPatternMessages,
  type PaginationControlsProps,
} from "@purplevoid/backend-infrastructure-web/patterns";

export type {
  PageMeta,
  PaginationControlsProps,
} from "@purplevoid/backend-infrastructure-web/patterns";

export function PaginationControls(props: PaginationControlsProps) {
  return <PublicPaginationControls messages={zhCNPatternMessages} {...props} />;
}
