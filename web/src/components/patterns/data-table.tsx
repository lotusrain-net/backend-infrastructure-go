"use client";

import type { ReactNode } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Card } from "@/components/ui/card";
import { PaginationControls, type PageMeta } from "@/components/patterns/pagination-controls";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export type { PageMeta } from "@/components/patterns/pagination-controls";

export interface DataTableColumn<T> {
  key: string;
  label: string;
  render: (row: T) => ReactNode;
  className?: string;
}

interface DataTableProps<T> {
  columns: Array<DataTableColumn<T>>;
  rows: T[];
  rowKey: (row: T) => string;
  page: PageMeta;
  isLoading?: boolean;
  error?: string | null;
  emptyTitle?: string;
  emptyDescription?: string;
  onPageChange?: (page: number) => void;
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  page,
  isLoading = false,
  error,
  emptyTitle = "暂无记录",
  emptyDescription = "调整筛选条件后重试。",
  onPageChange,
}: DataTableProps<T>) {
  const columnCount = Math.max(columns.length, 1);

  return (
    <Card className="overflow-hidden p-0 shadow-none sm:p-0">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[color:var(--border)] px-4 py-3">
        <p aria-live="polite" className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">
          共 {page.total} 条记录
        </p>
        <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">
          第 {page.page} / {Math.max(page.pages, 1)} 页
        </p>
      </div>

      <Table className="min-w-full">
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            {columns.map((column) => (
              <TableHead key={column.key} scope="col" className={column.className}>
                {column.label}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <LoadingRows columnCount={columnCount} />
          ) : error ? (
            <TableRow className="hover:bg-transparent">
              <TableCell colSpan={columnCount} className="py-10">
                <Alert variant="destructive">
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              </TableCell>
            </TableRow>
          ) : rows.length === 0 ? (
            <TableRow className="hover:bg-transparent">
              <TableCell colSpan={columnCount} className="py-12 text-center">
                <div role="status" className="mx-auto max-w-md space-y-1">
                  <p className="font-medium">{emptyTitle}</p>
                  <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">{emptyDescription}</p>
                </div>
              </TableCell>
            </TableRow>
          ) : (
            rows.map((row) => (
              <TableRow key={rowKey(row)}>
                {columns.map((column) => (
                  <TableCell key={column.key} className={column.className}>
                    {column.render(row)}
                  </TableCell>
                ))}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>

      {onPageChange ? (
        <PaginationControls className="border-t border-[color:var(--border)] px-4 py-3" page={page} onPageChange={onPageChange} />
      ) : null}
    </Card>
  );
}

function LoadingRows({ columnCount }: { columnCount: number }) {
  return (
    <>
      {Array.from({ length: 5 }, (_, index) => (
        <TableRow key={index} aria-busy="true" className="hover:bg-transparent">
          {Array.from({ length: columnCount }, (_, cellIndex) => (
            <TableCell key={cellIndex}>
              <Skeleton className="h-4 w-4/5" />
            </TableCell>
          ))}
        </TableRow>
      ))}
    </>
  );
}
