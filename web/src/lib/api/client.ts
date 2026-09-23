import { ApiClient, zhCNApiMessages } from "@lotusrain-net/backend-infrastructure-web/api";
import { queryClient } from "@/lib/query/client";
import { clearAuthState } from "@/stores/auth-store";
import { clearActiveThemeState } from "@/stores/theme-store";

export { ApiClient, ApiError, withQuery } from "@lotusrain-net/backend-infrastructure-web/api";
export type { ApiClientOptions, ApiRequestInit } from "@lotusrain-net/backend-infrastructure-web/api";

export const apiClient = new ApiClient({
  messages: zhCNApiMessages,
  refresh: async () => {
    const response = await fetch("/api/v1/auth/refresh", {
      method: "POST",
      credentials: "include",
    });

    if (!response.ok) {
      throw new Error("session refresh failed");
    }
  },
  onSessionExpired: () => {
    clearAuthState();
    clearActiveThemeState();
    queryClient.clear();

    if (typeof window !== "undefined") {
      const next = `${window.location.pathname}${window.location.search}`;
      window.location.assign(new URL(`/login?next=${encodeURIComponent(next)}`, window.location.origin).toString());
    }
  },
});
