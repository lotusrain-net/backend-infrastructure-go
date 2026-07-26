import type { ReactNode } from "react";
import { AppShell } from "@/components/layout/app-shell";
import { AuthGate } from "@/components/providers/auth-gate";

export default function ConsoleLayout({ children }: { children: ReactNode }) {
  return <AuthGate><AppShell>{children}</AppShell></AuthGate>;
}
