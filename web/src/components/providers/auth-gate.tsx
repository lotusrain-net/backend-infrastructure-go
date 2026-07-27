"use client";

import { useEffect, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useCurrentUserQuery } from "@/features/auth/api";
import { AppearancePreferencesProvider } from "@/features/preferences/provider";
import { ApiError } from "@/lib/api/client";

export function AuthGate({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { data: user, error, isPending } = useCurrentUserQuery();
  const status = error instanceof ApiError ? error.status : 0;

  useEffect(() => {
    if (!isPending && status === 401) {
      router.replace(`/login?next=${encodeURIComponent(pathname)}`);
    }
  }, [isPending, pathname, router, status]);

  if (isPending) {
    return (
      <main role="status" className="grid min-h-screen place-items-center bg-[color:var(--canvas)] p-6 text-sm text-[color:var(--fg-muted)]">
        正在恢复会话...
      </main>
    );
  }

  if (status === 401) {
    return null;
  }

  if (error) {
    return (
      <main className="grid min-h-screen place-items-center bg-[color:var(--canvas)] p-6">
        <div role="alert" className="max-w-md border border-[color:var(--danger)] bg-[color:var(--danger-subtle)] p-4 text-sm text-[color:var(--danger)]">
          当前无法恢复会话，请检查服务连接后重试。
        </div>
      </main>
    );
  }

  if (!user) {
    return null;
  }

  return <AppearancePreferencesProvider userID={user.id}>{children}</AppearancePreferencesProvider>;
}
