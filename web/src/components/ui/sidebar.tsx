"use client";

import * as React from "react";
import { PanelLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { cn } from "@/lib/utils";

interface SidebarContextValue {
  isMobile: boolean;
  openMobile: boolean;
  setOpenMobile: (open: boolean) => void;
  toggleSidebar: () => void;
}

const SidebarContext = React.createContext<SidebarContextValue | null>(null);

export function SidebarProvider({ children, className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  const [openMobile, setOpenMobile] = React.useState(false);
  const [isMobile, setIsMobile] = React.useState(false);

  React.useEffect(() => {
    const media = window.matchMedia("(max-width: 1023px)");
    const update = () => setIsMobile(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  const value = React.useMemo(() => ({ isMobile, openMobile, setOpenMobile, toggleSidebar: () => setOpenMobile((value) => !value) }), [isMobile, openMobile]);
  return <SidebarContext.Provider value={value}><div className={cn("flex min-h-screen w-full", className)} {...props}>{children}</div></SidebarContext.Provider>;
}

export function useSidebar() {
  const context = React.useContext(SidebarContext);
  if (!context) throw new Error("Sidebar components must be rendered inside SidebarProvider.");
  return context;
}

export function Sidebar({ children, className, ...props }: React.HTMLAttributes<HTMLElement>) {
  const { isMobile, openMobile, setOpenMobile } = useSidebar();
  const content = <div className="flex h-full w-full flex-col">{children}</div>;
  if (isMobile) {
    return <Sheet open={openMobile} onOpenChange={setOpenMobile}><SheetContent side="left" className={cn("w-[min(18rem,86vw)] border-r border-[color:var(--sidebar-border)] bg-[color:var(--sidebar)] p-0 text-[color:var(--sidebar-foreground)]", className)}><SheetHeader className="sr-only"><SheetTitle>主导航</SheetTitle><SheetDescription>控制台主导航</SheetDescription></SheetHeader>{content}</SheetContent></Sheet>;
  }
  return <aside className={cn("sticky top-14 hidden h-[calc(100vh-3.5rem)] w-64 shrink-0 border-r border-[color:var(--sidebar-border)] bg-[color:var(--sidebar)] text-[color:var(--sidebar-foreground)] lg:block", className)} {...props}>{content}</aside>;
}

export function SidebarInset({ className, ...props }: React.HTMLAttributes<HTMLElement>) { return <main className={cn("min-w-0 flex-1 bg-[color:var(--background)]", className)} {...props} />; }
export function SidebarContent({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) { return <div className={cn("flex min-h-0 flex-1 flex-col overflow-y-auto", className)} {...props} />; }
export function SidebarGroup({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) { return <div className={cn("px-3 py-2", className)} {...props} />; }
export function SidebarGroupLabel({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) { return <p className={cn("px-2 py-1 text-[length:var(--text-caption)] font-medium text-[color:var(--sidebar-muted)]", className)} {...props} />; }
export function SidebarMenu({ className, ...props }: React.HTMLAttributes<HTMLUListElement>) { return <ul className={cn("space-y-1", className)} {...props} />; }
export function SidebarMenuItem({ className, ...props }: React.LiHTMLAttributes<HTMLLIElement>) { return <li className={className} {...props} />; }
export function SidebarTrigger({ className, ...props }: React.ComponentProps<typeof Button>) {
  const { toggleSidebar } = useSidebar();
  return <Button variant="ghost" size="icon" className={cn("lg:hidden", className)} aria-label="打开导航" title="打开导航" onClick={toggleSidebar} {...props}><PanelLeft aria-hidden="true" className="size-5" /></Button>;
}
