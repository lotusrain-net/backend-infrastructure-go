"use client";

import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationFirst,
  PaginationItem,
  PaginationLast,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";

export interface PageMeta {
  page: number;
  size: number;
  total: number;
  pages: number;
  has_next: boolean;
  has_prev: boolean;
}

interface PaginationControlsProps {
  page: PageMeta;
  onPageChange: (page: number) => void;
  className?: string;
}

type PageToken = number | "ellipsis";

export function PaginationControls({ page, onPageChange, className }: PaginationControlsProps) {
  const totalPages = Number.isSafeInteger(page.pages) ? Math.max(page.pages, 0) : 0;
  const currentPage = totalPages === 0 ? 1 : clamp(safePage(page.page), 1, totalPages);
  const isFirstPage = totalPages === 0 || currentPage === 1;
  const isLastPage = totalPages === 0 || currentPage === totalPages;

  return (
    <Pagination className={className}>
      <PaginationContent className="justify-end">
        <PaginationItem>
          <PaginationFirst disabled={isFirstPage} onClick={() => onPageChange(1)} />
        </PaginationItem>
        <PaginationItem>
          <PaginationPrevious disabled={isFirstPage} onClick={() => onPageChange(currentPage - 1)} />
        </PaginationItem>
        {paginationTokens(totalPages, currentPage).map((token, index) => (
          <PaginationItem key={`${token}-${index}`}>
            {token === "ellipsis" ? (
              <PaginationEllipsis />
            ) : (
              <PaginationLink
                aria-label={`第 ${token} 页`}
                isActive={token === currentPage}
                disabled={token === currentPage}
                onClick={() => onPageChange(token)}
              >
                {token}
              </PaginationLink>
            )}
          </PaginationItem>
        ))}
        <PaginationItem>
          <PaginationNext disabled={isLastPage} onClick={() => onPageChange(currentPage + 1)} />
        </PaginationItem>
        <PaginationItem>
          <PaginationLast disabled={isLastPage} onClick={() => onPageChange(totalPages)} />
        </PaginationItem>
      </PaginationContent>
    </Pagination>
  );
}

function paginationTokens(totalPages: number, currentPage: number): PageToken[] {
  if (totalPages === 0) {
    return [];
  }
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }
  if (currentPage <= 3) {
    return [1, 2, 3, 4, "ellipsis", totalPages];
  }
  if (currentPage >= totalPages - 2) {
    return [1, "ellipsis", totalPages - 3, totalPages - 2, totalPages - 1, totalPages];
  }
  return [1, "ellipsis", currentPage - 1, currentPage, currentPage + 1, "ellipsis", totalPages];
}

function safePage(page: number) {
  return Number.isSafeInteger(page) && page > 0 ? page : 1;
}

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(Math.max(value, minimum), maximum);
}
