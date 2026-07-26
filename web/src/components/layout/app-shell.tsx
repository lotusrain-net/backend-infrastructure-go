"use client";

import { useState, type ReactNode } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { Header } from "@/components/layout/header";
import { Sidebar } from "@/components/layout/sidebar";

export function AppShell({ children }: { children: ReactNode }) {
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <div className="min-h-screen bg-[color:var(--canvas)] text-[color:var(--fg-default)]">
      <Header onOpenMobileNav={() => setMobileOpen(true)} />
      <div className="mx-auto flex min-h-[calc(100vh-3.5rem)] max-w-[1600px]">
        <aside className="sticky top-14 hidden h-[calc(100vh-3.5rem)] w-64 shrink-0 border-r border-[color:var(--border-subtle)] bg-[color:var(--surface-raised)] lg:block">
          <Sidebar />
        </aside>
        <main id="main-content" className="min-w-0 flex-1 p-4 sm:p-6 lg:p-8">
          {children}
        </main>
      </div>

      <Dialog.Root open={mobileOpen} onOpenChange={setMobileOpen}>
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 z-40 bg-black/45 lg:hidden" />
          <Dialog.Content className="fixed inset-y-0 left-0 z-50 w-[min(18rem,86vw)] border-r border-[color:var(--border-subtle)] bg-[color:var(--surface-raised)] outline-none lg:hidden">
            <Dialog.Title className="sr-only">主导航</Dialog.Title>
            <Sidebar onNavigate={() => setMobileOpen(false)} />
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  );
}
