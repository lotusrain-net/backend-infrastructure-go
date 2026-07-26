"use client";

import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { LogOut, Menu, UserCircle2 } from "lucide-react";
import { ThemeControls } from "@/components/layout/theme-controls";
import { Button } from "@/components/ui/button";
import { useLogoutMutation } from "@/features/auth/api";
import { useAuthStore } from "@/stores/auth-store";

const pageTitles: Array<[string, string]> = [
  ["/iam/users", "用户"],
  ["/iam/roles", "角色"],
  ["/iam/permissions", "权限"],
  ["/audit", "审计日志"],
  ["/tasks", "任务执行"],
  ["/profile", "个人资料"],
  ["/dashboard", "概览"],
];

export function Header({ onOpenMobileNav }: { onOpenMobileNav: () => void }) {
  const pathname = usePathname();
  const router = useRouter();
  const user = useAuthStore((state) => state.user);
  const logout = useLogoutMutation();
  const title = pageTitles.find(([route]) => pathname.startsWith(route))?.[1] ?? "平台控制台";

  async function handleLogout() {
    try {
      await logout.mutateAsync();
    } finally {
      router.replace("/login");
    }
  }

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b border-[color:var(--border-subtle)] bg-[color:var(--surface-raised)] px-4 lg:px-6">
      <div className="flex min-w-0 items-center gap-3">
        <Button variant="ghost" size="icon" className="lg:hidden" onClick={onOpenMobileNav} aria-label="打开导航" title="打开导航">
          <Menu aria-hidden="true" className="h-5 w-5" />
        </Button>
        <div className="min-w-0">
          <p className="truncate text-xs text-[color:var(--fg-muted)]">基础设施控制台</p>
          <p className="truncate text-sm font-semibold text-[color:var(--fg-default)]">{title}</p>
        </div>
      </div>

      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <Button variant="ghost" size="sm" className="max-w-[13rem]">
            <UserCircle2 aria-hidden="true" className="h-4 w-4 shrink-0" />
            <span className="truncate">{user?.display_name || user?.username || "会话"}</span>
          </Button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content align="end" sideOffset={8} className="z-50 w-72 border border-[color:var(--border-subtle)] bg-[color:var(--surface-raised)] p-3 shadow-[0_10px_24px_var(--shadow-color)]">
            <div className="border-b border-[color:var(--border-subtle)] pb-3">
              <p className="text-sm font-medium text-[color:var(--fg-default)]">{user?.display_name || user?.username || "当前会话"}</p>
              <p className="mt-1 truncate text-sm text-[color:var(--fg-muted)]">{user?.email}</p>
            </div>
            <div className="py-3">
              <ThemeControls />
            </div>
            <div className="border-t border-[color:var(--border-subtle)] pt-2">
              <DropdownMenu.Item asChild>
                <Link href="/profile" className="flex h-9 items-center gap-2 px-2 text-sm text-[color:var(--fg-default)] outline-none hover:bg-[color:var(--surface-hover)] focus:bg-[color:var(--surface-hover)]">
                  <UserCircle2 aria-hidden="true" className="h-4 w-4" />
                  个人资料
                </Link>
              </DropdownMenu.Item>
              <DropdownMenu.Item asChild>
                <button type="button" onClick={handleLogout} disabled={logout.isPending} className="flex h-9 w-full items-center gap-2 px-2 text-left text-sm text-[color:var(--danger)] outline-none hover:bg-[color:var(--danger-subtle)] focus:bg-[color:var(--danger-subtle)] disabled:opacity-50">
                  <LogOut aria-hidden="true" className="h-4 w-4" />
                  退出登录
                </button>
              </DropdownMenu.Item>
            </div>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </header>
  );
}
