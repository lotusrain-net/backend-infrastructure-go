"use client";

import type { ReactNode } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "@/lib/query/client";
import { ThemeInit } from "@/components/providers/theme-init";
import { Toaster } from "@/components/ui/toaster";

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeInit />
      {children}
      <Toaster />
    </QueryClientProvider>
  );
}
