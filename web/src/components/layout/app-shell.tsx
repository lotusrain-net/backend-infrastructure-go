"use client";

import type { ReactNode } from "react";
import { Header } from "@/components/layout/header";
import { Sidebar as AppSidebar } from "@/components/layout/sidebar";
import { Sidebar, SidebarInset, SidebarProvider } from "@/components/ui/sidebar";

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <SidebarProvider className="min-h-screen flex-col bg-[color:var(--background)] text-[color:var(--foreground)]">
      <Header />
      <div className="flex min-h-[calc(100vh-3.5rem)]">
        <Sidebar>
          <AppSidebar />
        </Sidebar>
        <SidebarInset id="main-content">
          <div className="mx-auto w-full max-w-[1440px] p-4 sm:p-6 lg:p-8">
            {children}
          </div>
        </SidebarInset>
      </div>
    </SidebarProvider>
  );
}
