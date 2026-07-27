"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { LogOut, ServerCog, UserCircle2 } from "lucide-react";
import { ThemeControls } from "@/components/layout/theme-controls";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { appNavigation, type NavigationItem } from "@/config/navigation";
import { useLogoutMutation } from "@/features/auth/api";
import { filterNavigation } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth-store";

export function Header() {
  const pathname = usePathname();
  const router = useRouter();
  const user = useAuthStore((state) => state.user);
  const logout = useLogoutMutation();
  const navigation = flattenNavigation(filterNavigation(appNavigation, { permissions: user?.permissions ?? [] }));
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
      <div className="grid h-full grid-cols-[minmax(0,1fr)_minmax(0,1fr)] items-center gap-3 xl:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]">
        <div className="flex min-w-0 items-center gap-2.5">
          <SidebarTrigger />
          <Link href="/dashboard" className="flex min-w-0 items-center gap-2.5 rounded-[var(--radius-md)] outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]">
            <span aria-hidden="true" className="flex size-7 shrink-0 items-center justify-center bg-[color:var(--primary-subtle)] text-[color:var(--primary)] [border-radius:var(--radius-md)]">
              <ServerCog className="size-4" />
            </span>
            <span className="truncate text-[length:var(--text-body-sm)] font-semibold text-[color:var(--foreground)]">基础设施控制台</span>
          </Link>
        </div>

        <nav aria-label="快捷导航" className="hidden items-center justify-center gap-1 xl:flex">
          {navigation.map((item) => {
            const active = isActiveRoute(pathname, item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={`border-b-2 px-2.5 py-2 text-[length:var(--text-label)] font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] ${
                  active
                    ? "border-[color:var(--primary)] text-[color:var(--foreground)]"
                    : "border-transparent text-[color:var(--muted-foreground)] hover:text-[color:var(--foreground)]"
                }`}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>

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

function flattenNavigation(items: NavigationItem[]) {
  return items
    .flatMap((item) => item.children?.length ? item.children : [item])
    .filter((item) => item.href !== "/profile");
}

function isActiveRoute(pathname: string, href: string) {
  return pathname === href || (href !== "/dashboard" && pathname.startsWith(`${href}/`));
}
