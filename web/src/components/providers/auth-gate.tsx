"use client";

import { useEffect, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useCurrentUserQuery } from "@/features/auth/api";
import { AppearancePreferencesProvider } from "@/features/preferences/provider";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { ApiError } from "@/lib/api/client";

export function AuthGate({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { data: user, error, isPending } = useCurrentUserQuery();
  const status = error instanceof ApiError ? error.status : 0;
  // A token from an earlier database can still be syntactically valid while
  // its subject no longer exists. The current-user endpoint reports that as
  // 404, which is recoverable in the same way as an expired session.
  const hasInvalidSession = status === 401 || status === 404;

  useEffect(() => {
    if (!isPending && hasInvalidSession) {
      router.replace(`/login?next=${encodeURIComponent(pathname)}`);
    }
  }, [hasInvalidSession, isPending, pathname, router]);

  if (isPending) {
    return (
      <main role="status" className="grid min-h-screen place-items-center bg-[color:var(--background)] p-6 text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">
        正在恢复会话...
      </main>
    );
  }

  if (hasInvalidSession) {
    return null;
  }

  if (error) {
    return (
      <main className="grid min-h-screen place-items-center bg-[color:var(--background)] p-6">
        <Alert variant="destructive" className="max-w-md">
          <AlertDescription>当前无法恢复会话，请检查服务连接后重试。</AlertDescription>
        </Alert>
      </main>
    );
  }

  if (!user) {
    return null;
  }

  return <AppearancePreferencesProvider userID={user.id}>{children}</AppearancePreferencesProvider>;
}
