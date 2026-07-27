"use client";

import { useEffect, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { hasPermission } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth-store";

export function PermissionGate({ children, permission }: { children: ReactNode; permission: string }) {
  const router = useRouter();
  const permissions = useAuthStore((state) => state.user?.permissions ?? []);
  const allowed = hasPermission({ permissions }, permission);

  useEffect(() => {
    if (!allowed) {
      router.replace("/403");
    }
  }, [allowed, router]);

  if (!allowed) {
    return (
      <div role="status" className="grid min-h-[16rem] place-items-center text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">
        正在检查访问权限...
      </div>
    );
  }

  return <>{children}</>;
}
