"use client";

import { RotateCcw, Search } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

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

const pageSizes = [10, 20, 50, 100] as const;

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
    <Card className="grid grid-cols-1 items-end gap-3 p-4 shadow-none sm:flex sm:flex-wrap sm:p-4">
      <div className="grid min-w-0 w-full gap-1.5 sm:min-w-[14rem] sm:flex-1">
        <Label htmlFor="filter-keyword">{keywordLabel}</Label>
        <div className="relative">
          <Search aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[color:var(--muted-foreground)]" />
          <Input
            id="filter-keyword"
            value={keyword}
            onChange={(event) => onKeywordChange(event.target.value)}
            placeholder={keywordPlaceholder}
            className="pl-9"
          />
        </div>
      </div>

      {children ? <div className="min-w-0 w-full sm:w-auto">{children}</div> : null}

      <div className="grid w-full justify-items-start gap-1.5 sm:w-auto">
        <Label htmlFor="filter-page-size">每页数量</Label>
        <Select value={String(pageSize)} onValueChange={(value) => onPageSizeChange(Number(value))}>
          <SelectTrigger id="filter-page-size" className="w-[6.5rem]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {pageSizes.map((size) => (
              <SelectItem key={size} value={String(size)}>
                {size} 条
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <Button type="button" variant="outline" className="w-full sm:w-auto" onClick={onReset}>
        <RotateCcw aria-hidden="true" className="size-4" />
        重置
      </Button>
    </Card>
  );
}
