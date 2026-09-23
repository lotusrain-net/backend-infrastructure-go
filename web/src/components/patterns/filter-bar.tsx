"use client";

import {
  FilterBar as PublicFilterBar,
  zhCNPatternMessages,
  type FilterBarProps,
} from "@lotusrain-net/backend-infrastructure-web/patterns";

export type { FilterBarProps } from "@lotusrain-net/backend-infrastructure-web/patterns";

export function FilterBar(props: FilterBarProps) {
  return <PublicFilterBar messages={zhCNPatternMessages} {...props} />;
}
