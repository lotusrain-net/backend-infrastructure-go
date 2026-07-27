"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

export interface PageMeta {
  page: number;
  size: number;
  total: number;
  pages: number;
  has_next: boolean;
  has_prev: boolean;
}

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
    <section className="overflow-hidden border border-[color:var(--border-subtle)] bg-[color:var(--surface)] [border-radius:var(--radius-lg)]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[color:var(--border-subtle)] px-4 py-3">
        <p aria-live="polite" className="text-[length:var(--text-body-sm)] text-[color:var(--fg-muted)]">
          共 {page.total} 条记录
        </p>
        <p className="text-[length:var(--text-body-sm)] text-[color:var(--fg-muted)]">
          第 {page.page} / {Math.max(page.pages, 1)} 页
        </p>
      </div>

      <div className="overflow-x-auto">
        <table className="min-w-full text-left text-[length:var(--text-body-sm)]">
          <thead className="bg-[color:var(--surface-subtle)] text-[length:var(--text-caption)] text-[color:var(--fg-muted)]">
            <tr>
              {columns.map((column) => (
                <th key={column.key} scope="col" className="whitespace-nowrap px-4 py-3 font-semibold">
                  {column.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <LoadingRows columnCount={columnCount} />
            ) : error ? (
              <tr>
                <td colSpan={columnCount} className="px-4 py-10">
                  <div role="alert" className="border-l-2 border-[color:var(--danger)] bg-[color:var(--danger-subtle)] px-4 py-3 text-[color:var(--danger)]">
                    {error}
                  </div>
                </td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={columnCount} className="px-4 py-12 text-center">
                  <div role="status" className="mx-auto max-w-md space-y-1">
                    <p className="font-medium text-[color:var(--fg-default)]">{emptyTitle}</p>
                    <p className="text-[length:var(--text-body-sm)] text-[color:var(--fg-muted)]">{emptyDescription}</p>
                  </div>
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <tr key={rowKey(row)} className="border-t border-[color:var(--border-subtle)] hover:bg-[color:var(--surface-hover)]">
                  {columns.map((column) => (
                    <td key={column.key} className={`px-4 py-3 text-[color:var(--fg-default)] ${column.className ?? ""}`}>
                      {column.render(row)}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {onPageChange ? (
        <nav aria-label="分页" className="flex items-center justify-end gap-2 border-t border-[color:var(--border-subtle)] px-4 py-3">
          <PaginationButton
            label="上一页"
            disabled={!page.has_prev}
            onClick={() => onPageChange(page.page - 1)}
          >
            <ChevronLeft aria-hidden="true" className="h-4 w-4" />
          </PaginationButton>
          <PaginationButton
            label="下一页"
            disabled={!page.has_next}
            onClick={() => onPageChange(page.page + 1)}
          >
            <ChevronRight aria-hidden="true" className="h-4 w-4" />
          </PaginationButton>
        </nav>
      ) : null}
    </section>
  );
}

function LoadingRows({ columnCount }: { columnCount: number }) {
  return (
    <>
      {Array.from({ length: 5 }, (_, index) => (
        <tr key={index} className="border-t border-[color:var(--border-subtle)]" aria-busy="true">
          {Array.from({ length: columnCount }, (_, cellIndex) => (
            <td key={cellIndex} className="px-4 py-3">
              <span className="block h-4 w-4/5 animate-pulse bg-[color:var(--surface-subtle)]" />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

function PaginationButton({
  children,
  disabled,
  label,
  onClick,
}: {
  children: ReactNode;
  disabled: boolean;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className="inline-flex h-9 w-9 items-center justify-center border border-[color:var(--border-strong)] text-[color:var(--fg-default)] transition-colors [border-radius:var(--radius-md)] hover:bg-[color:var(--surface-hover)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)] disabled:cursor-not-allowed disabled:opacity-45"
    >
      {children}
    </button>
  );
}
