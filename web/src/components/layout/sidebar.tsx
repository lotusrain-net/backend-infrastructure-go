"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { appNavigation, type NavigationItem } from "@/config/navigation";
import { filterNavigation } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth-store";
import { SidebarContent, useSidebar } from "@/components/ui/sidebar";

export function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname();
  const permissions = useAuthStore((state) => state.user?.permissions ?? []);
  const navigation = filterNavigation(appNavigation, { permissions });
  const { setOpenMobile } = useSidebar();

  const handleNavigate = onNavigate ?? (() => setOpenMobile(false));

  return (
    <SidebarContent className="p-3">
      <nav aria-label="主导航" className="space-y-1">
        {navigation.map((item) => (
          <NavigationGroup key={item.href} item={item} pathname={pathname} onNavigate={handleNavigate} />
        ))}
      </nav>
    </SidebarContent>
  );
}

function NavigationGroup({ item, pathname, onNavigate }: { item: NavigationItem; pathname: string; onNavigate?: () => void }) {
  if (item.children?.length) {
    return (
      <div className="py-2">
        <p className="flex items-center gap-2 px-2 text-[length:var(--text-caption)] font-medium text-[color:var(--sidebar-muted)]">
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
      className={`flex h-10 items-center gap-2 px-2 text-[length:var(--text-body-sm)] transition-colors [border-radius:var(--radius-md)] ${
        active
          ? "bg-[color:var(--sidebar-primary)] text-[color:var(--sidebar-primary-foreground)]"
          : "text-[color:var(--sidebar-foreground)] hover:bg-[color:var(--sidebar-accent)] hover:text-[color:var(--sidebar-accent-foreground)]"
      } ${nested ? "ml-2" : ""}`}
    >
      <item.icon aria-hidden="true" className="h-4 w-4 shrink-0" />
      <span>{item.label}</span>
    </Link>
  );
}
