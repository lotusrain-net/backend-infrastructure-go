"use client";

import { Toaster as SonnerToaster, toast } from "sonner";

export { toast };

export function Toaster() {
  return <SonnerToaster closeButton position="bottom-right" toastOptions={{ classNames: { toast: "!border !border-[color:var(--border)] !bg-[color:var(--popover)] !text-[color:var(--popover-foreground)] !shadow-[var(--shadow-popover)] ![border-radius:var(--radius-lg)]", title: "!text-[color:var(--popover-foreground)]", description: "!text-[color:var(--muted-foreground)]", success: "!border-[color:var(--success)]", error: "!border-[color:var(--destructive)]", actionButton: "!bg-[color:var(--primary)] !text-[color:var(--primary-foreground)]" } }} />;
}
