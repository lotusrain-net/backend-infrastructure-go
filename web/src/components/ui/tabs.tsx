"use client";

import * as TabsPrimitive from "@radix-ui/react-tabs";
import * as React from "react";
import { cn } from "@/lib/utils";

export function Tabs({ className, ...props }: React.ComponentProps<typeof TabsPrimitive.Root>) {
  return <TabsPrimitive.Root className={cn("flex flex-col gap-5", className)} {...props} />;
}

export function TabsList({ className, ...props }: React.ComponentProps<typeof TabsPrimitive.List>) {
  return <TabsPrimitive.List className={cn("inline-flex min-h-10 w-fit max-w-full flex-wrap items-center gap-1 border border-[color:var(--border)] bg-[color:var(--muted)] p-1 [border-radius:var(--radius-lg)]", className)} {...props} />;
}

export function TabsTrigger({ className, ...props }: React.ComponentProps<typeof TabsPrimitive.Trigger>) {
  return <TabsPrimitive.Trigger className={cn("inline-flex h-8 min-w-0 items-center justify-center gap-2 whitespace-nowrap border border-transparent px-3 text-[length:var(--text-label)] font-medium text-[color:var(--muted-foreground)] [border-radius:var(--radius-md)] outline-none transition-colors data-[state=active]:border-[color:var(--border)] data-[state=active]:bg-[color:var(--card)] data-[state=active]:text-[color:var(--foreground)] data-[state=active]:shadow-sm focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] disabled:pointer-events-none disabled:opacity-50", className)} {...props} />;
}

export function TabsContent({ className, ...props }: React.ComponentProps<typeof TabsPrimitive.Content>) {
  return <TabsPrimitive.Content className={cn("outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]", className)} {...props} />;
}

// Legacy aliases keep the existing profile API stable while adopting Radix behavior.
export const TabList = TabsList;
export const TabTrigger = TabsTrigger;
export const TabPanel = TabsContent;
