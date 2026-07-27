"use client";

import { RotateCcw, Search } from "lucide-react";
import type { ReactNode } from "react";

interface FilterBarProps {
  keyword: string;
  pageSize: number;
  onKeywordChange: (keyword: string) => void;
  onPageSizeChange: (size: number) => void;
  onReset: () => void;
  keywordLabel?: string;
  keywordPlaceholder?: string;
  children?: ReactNode;
}

export function FilterBar({
  keyword,
  pageSize,
  onKeywordChange,
  onPageSizeChange,
  onReset,
  keywordLabel = "关键词",
  keywordPlaceholder = "输入关键词筛选",
  children,
}: FilterBarProps) {
  return (
    <div className="flex flex-wrap items-end gap-3 border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-4 [border-radius:var(--radius-lg)]">
      <label className="grid min-w-[14rem] flex-1 gap-1.5">
        <span className="text-[length:var(--text-caption)] font-medium text-[color:var(--fg-muted)]">{keywordLabel}</span>
        <span className="relative">
          <Search aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[color:var(--fg-muted)]" />
          <input
            value={keyword}
            onChange={(event) => onKeywordChange(event.target.value)}
            placeholder={keywordPlaceholder}
            className="h-10 w-full border border-[color:var(--border-strong)] bg-[color:var(--surface)] py-2 pl-9 pr-3 text-[length:var(--text-body-sm)] text-[color:var(--fg-default)] outline-none [border-radius:var(--radius-md)] placeholder:text-[color:var(--fg-muted)] focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)]"
          />
        </span>
      </label>

      {children}

      <label className="grid gap-1.5">
        <span className="text-[length:var(--text-caption)] font-medium text-[color:var(--fg-muted)]">每页数量</span>
        <select
          value={pageSize}
          onChange={(event) => onPageSizeChange(Number(event.target.value))}
          className="h-10 border border-[color:var(--border-strong)] bg-[color:var(--surface)] px-3 text-[length:var(--text-body-sm)] text-[color:var(--fg-default)] outline-none [border-radius:var(--radius-md)] focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)]"
        >
          {[10, 20, 50, 100].map((size) => (
            <option key={size} value={size}>
              {size} 条
            </option>
          ))}
        </select>
      </label>

      <button
        type="button"
        onClick={onReset}
        className="inline-flex h-10 items-center gap-2 border border-[color:var(--border-strong)] px-3 text-[length:var(--text-label)] font-medium text-[color:var(--fg-default)] transition-colors [border-radius:var(--radius-md)] hover:bg-[color:var(--surface-hover)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)]"
      >
        <RotateCcw aria-hidden="true" className="h-4 w-4" />
        重置
      </button>
    </div>
  );
}
