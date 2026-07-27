import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, MoreHorizontal } from "lucide-react";
import * as React from "react";
import { Button, buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function Pagination({ className, ...props }: React.ComponentProps<"nav">) { return <nav role="navigation" aria-label="分页" className={cn("mx-auto flex w-full justify-center", className)} {...props} />; }
export function PaginationContent({ className, ...props }: React.ComponentProps<"ul">) { return <ul className={cn("flex flex-wrap items-center justify-center gap-1", className)} {...props} />; }
export function PaginationItem({ className, ...props }: React.ComponentProps<"li">) { return <li className={cn("shrink-0", className)} {...props} />; }
export function PaginationLink({ className, isActive, size = "icon", ...props }: React.ComponentProps<"button"> & { isActive?: boolean; size?: "default" | "sm" | "icon" }) {
  return <button type="button" aria-current={isActive ? "page" : undefined} className={cn(buttonVariants({ variant: isActive ? "default" : "outline", size }), className)} {...props} />;
}
export function PaginationFirst({ className, children = "首页", "aria-label": ariaLabel = "首页", ...props }: React.ComponentProps<typeof Button>) { return <Button type="button" aria-label={ariaLabel} variant="outline" size="default" className={cn("gap-1 px-2.5", className)} {...props}><ChevronsLeft aria-hidden="true" className="size-4" /><span className="hidden sm:inline">{children}</span></Button>; }
export function PaginationPrevious({ className, children = "上一页", "aria-label": ariaLabel = "上一页", ...props }: React.ComponentProps<typeof Button>) { return <Button type="button" aria-label={ariaLabel} variant="outline" size="default" className={cn("gap-1 px-2.5", className)} {...props}><ChevronLeft aria-hidden="true" className="size-4" /><span className="hidden sm:inline">{children}</span></Button>; }
export function PaginationNext({ className, children = "下一页", "aria-label": ariaLabel = "下一页", ...props }: React.ComponentProps<typeof Button>) { return <Button type="button" aria-label={ariaLabel} variant="outline" size="default" className={cn("gap-1 px-2.5", className)} {...props}><span className="hidden sm:inline">{children}</span><ChevronRight aria-hidden="true" className="size-4" /></Button>; }
export function PaginationLast({ className, children = "末页", "aria-label": ariaLabel = "末页", ...props }: React.ComponentProps<typeof Button>) { return <Button type="button" aria-label={ariaLabel} variant="outline" size="default" className={cn("gap-1 px-2.5", className)} {...props}><span className="hidden sm:inline">{children}</span><ChevronsRight aria-hidden="true" className="size-4" /></Button>; }
export function PaginationEllipsis({ className, ...props }: React.ComponentProps<"span">) { return <span aria-hidden="true" className={cn("flex size-9 items-center justify-center text-[color:var(--muted-foreground)]", className)} {...props}><MoreHorizontal className="size-4" /><span className="sr-only">更多页面</span></span>; }
