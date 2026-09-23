import { queryOptions, useMutation, useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api/client";
import { queryClient } from "@/lib/query/client";
import { clearAuthState, setCurrentUser } from "@/stores/auth-store";
import { clearActiveThemeState } from "@/stores/theme-store";
import type {
  AuthenticatedUser,
  LoginRequest,
  LoginResponse,
  LoginResult,
} from "@/types/api";

export const authKeys = {
  currentUser: () => ["auth", "current-user"] as const,
};

export async function getCurrentUser(): Promise<AuthenticatedUser> {
  const user = await apiClient.request<AuthenticatedUser>("/api/v1/users/me");
  setCurrentUser(user);
  return user;
}

export const currentUserQueryOptions = () =>
  queryOptions({
    queryKey: authKeys.currentUser(),
    queryFn: getCurrentUser,
  });

export async function login(
  input: LoginRequest,
): Promise<LoginResult> {
  const result = await apiClient.request<LoginResponse>(
    "/api/v1/auth/login",
    {
      method: "POST",
      body: input,
      requiresAuth: false,
    },
  );

  if ("status" in result && result.status === "totp_required") return result;

  const user = await getCurrentUser();
  queryClient.setQueryData(authKeys.currentUser(), user);
  return user;
}

export async function logout(): Promise<void> {
  try {
    await apiClient.request<Record<string, boolean>>("/api/v1/auth/logout", {
      method: "POST",
      requiresAuth: false,
    });
  } finally {
    clearAuthState();
    clearActiveThemeState();
    queryClient.clear();
  }
}

export function useCurrentUserQuery() {
  return useQuery(currentUserQueryOptions());
}

export function useLoginMutation() {
  return useMutation({
    mutationFn: login,
    gcTime: 0,
  });
}

export function useLogoutMutation() {
  return useMutation({
    mutationFn: logout,
  });
}
