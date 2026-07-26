import * as React from "react";
import { cn } from "@/lib/utils";

export function Card({ className, ...props }: React.HTMLAttributes<HTMLElement>) {
  return (
    <section
      className={cn("border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-5", className)}
      {...props}
    />
  );
}
