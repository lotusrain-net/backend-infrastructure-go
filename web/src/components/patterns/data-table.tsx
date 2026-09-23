"use client";

import {
  DataTable as PublicDataTable,
  zhCNPatternMessages,
  type DataTableProps,
} from "@lotusrain-net/backend-infrastructure-web/patterns";

export type {
  DataTableColumn,
  DataTableProps,
  PageMeta,
} from "@lotusrain-net/backend-infrastructure-web/patterns";

export function DataTable<T>(props: DataTableProps<T>) {
  return <PublicDataTable messages={zhCNPatternMessages} {...props} />;
}
