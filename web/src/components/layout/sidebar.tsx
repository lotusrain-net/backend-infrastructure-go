"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { appNavigation, type NavigationItem } from "@/config/navigation";
import { filterNavigation } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth-store";

export function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname();
  const permissions = useAuthStore((state) => state.user?.permissions ?? []);
  const navigation = filterNavigation(appNavigation, { permissions });

  return (
    <div className="flex h-full flex-col p-4">
      <div className="border-b border-[color:var(--border-subtle)] pb-4">
        <p className="text-sm font-semibold text-[color:var(--fg-default)]">基础设施</p>
        <p className="mt-1 text-xs text-[color:var(--fg-muted)]">平台控制台</p>
      </div>
      <nav aria-label="主导航" className="mt-4 space-y-1">
        {navigation.map((item) => (
          <NavigationGroup key={item.href} item={item} pathname={pathname} onNavigate={onNavigate} />
        ))}
      </nav>
    </div>
  );
}

function NavigationGroup({ item, pathname, onNavigate }: { item: NavigationItem; pathname: string; onNavigate?: () => void }) {
  if (item.children?.length) {
    return (
      <div className="py-2">
        <p className="flex items-center gap-2 px-2 text-xs font-medium text-[color:var(--fg-muted)]">
          <item.icon aria-hidden="true" className="h-4 w-4" />
          {item.label}
        </p>
        <div className="mt-1 space-y-1">
          {item.children.map((child) => (
            <NavigationLink key={child.href} item={child} pathname={pathname} onNavigate={onNavigate} nested />
          ))}
        </div>
      </div>
    );
  }

  return <NavigationLink item={item} pathname={pathname} onNavigate={onNavigate} />;
}

function NavigationLink({ item, pathname, onNavigate, nested = false }: { item: NavigationItem; pathname: string; onNavigate?: () => void; nested?: boolean }) {
  const active = pathname === item.href || (item.href !== "/dashboard" && pathname.startsWith(`${item.href}/`));

  return (
    <Link
      href={item.href}
      onClick={onNavigate}
      aria-current={active ? "page" : undefined}
      className={`flex h-10 items-center gap-2 px-2 text-sm transition-colors ${
        active
          ? "bg-[color:var(--accent-primary)] text-[color:var(--accent-contrast)]"
          : "text-[color:var(--fg-muted)] hover:bg-[color:var(--surface-hover)] hover:text-[color:var(--fg-default)]"
      } ${nested ? "ml-2" : ""}`}
    >
      <item.icon aria-hidden="true" className="h-4 w-4 shrink-0" />
      <span>{item.label}</span>
    </Link>
  );
}
