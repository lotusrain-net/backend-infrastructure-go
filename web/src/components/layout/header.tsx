"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { LogOut, ServerCog, UserCircle2 } from "lucide-react";
import { ThemeControls } from "@/components/layout/theme-controls";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { useLogoutMutation } from "@/features/auth/api";
import { useAuthStore } from "@/stores/auth-store";

export function Header() {
  const router = useRouter();
  const user = useAuthStore((state) => state.user);
  const logout = useLogoutMutation();
  const displayName = user?.display_name || user?.username || "当前会话";
  const avatarInitial = displayName.trim().slice(0, 1).toLocaleUpperCase() || "会";

  async function handleLogout() {
    try {
      await logout.mutateAsync();
    } finally {
      router.replace("/login");
    }
  }

  return (
    <header className="sticky top-0 z-30 h-14 border-b border-[color:var(--border)] bg-[color:var(--popover)] px-4 lg:px-6">
      <div className="flex h-full items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2.5">
          <SidebarTrigger />
          <Link href="/dashboard" className="flex min-w-0 items-center gap-2.5 rounded-[var(--radius-md)] outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]">
            <span aria-hidden="true" className="flex size-7 shrink-0 items-center justify-center bg-[color:var(--primary-subtle)] text-[color:var(--primary)] [border-radius:var(--radius-md)]">
              <ServerCog className="size-4" />
            </span>
            <span className="truncate text-[length:var(--text-body-sm)] font-semibold text-[color:var(--foreground)]">基础设施控制台</span>
          </Link>
        </div>

        <div className="flex min-w-0 items-center justify-end">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="[border-radius:var(--radius-full)]"
                aria-label={`打开${displayName}的账户菜单`}
                title={`打开${displayName}的账户菜单`}
              >
                <span aria-hidden="true" className="flex size-7 items-center justify-center bg-[color:var(--primary)] text-[length:var(--text-caption)] font-semibold text-[color:var(--primary-foreground)] [border-radius:var(--radius-full)]">
                  {avatarInitial}
                </span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-72 p-3">
              <div className="border-b border-[color:var(--border)] pb-3">
                <p className="text-[length:var(--text-body-sm)] font-medium text-[color:var(--foreground)]">{displayName}</p>
                <p className="mt-1 truncate text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">{user?.email}</p>
              </div>
              <div className="py-3">
                <ThemeControls />
              </div>
              <DropdownMenuSeparator />
              <DropdownMenuItem asChild>
                <Link href="/profile" className="flex items-center gap-2">
                  <UserCircle2 aria-hidden="true" className="h-4 w-4" />
                  个人资料
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={handleLogout} disabled={logout.isPending} className="text-[color:var(--destructive)] data-[highlighted]:bg-[color:var(--destructive-subtle)] data-[highlighted]:text-[color:var(--destructive-subtle-foreground)]">
                <LogOut aria-hidden="true" className="h-4 w-4" />
                退出登录
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </header>
  );
}
