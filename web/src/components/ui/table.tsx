import * as React from "react";
import { cn } from "@/lib/utils";

export function Table({ className, ...props }: React.ComponentProps<"table">) {
  return <div className="w-full overflow-x-auto"><table className={cn("w-full caption-bottom text-left text-[length:var(--text-body-sm)]", className)} {...props} /></div>;
}

export function TableHeader({ className, ...props }: React.ComponentProps<"thead">) { return <thead className={cn("border-b border-[color:var(--border)] bg-[color:var(--muted)] text-[color:var(--muted-foreground)]", className)} {...props} />; }
export function TableBody({ className, ...props }: React.ComponentProps<"tbody">) { return <tbody className={cn("[&_tr:last-child]:border-0", className)} {...props} />; }
export function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) { return <tfoot className={cn("border-t border-[color:var(--border)] bg-[color:var(--muted)] font-medium", className)} {...props} />; }
export function TableRow({ className, ...props }: React.ComponentProps<"tr">) { return <tr className={cn("border-b border-[color:var(--border)] transition-colors hover:bg-[color:var(--accent)]", className)} {...props} />; }
export function TableHead({ className, ...props }: React.ComponentProps<"th">) { return <th className={cn("h-11 whitespace-nowrap px-4 text-[length:var(--text-caption)] font-semibold", className)} {...props} />; }
export function TableCell({ className, ...props }: React.ComponentProps<"td">) { return <td className={cn("px-4 py-3 align-middle text-[color:var(--foreground)]", className)} {...props} />; }
export function TableCaption({ className, ...props }: React.ComponentProps<"caption">) { return <caption className={cn("mt-4 text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]", className)} {...props} />; }
