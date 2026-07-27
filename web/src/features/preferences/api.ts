import { queryOptions, useMutation, useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api/client";
import type { AccountPreferences } from "@/types/api";

export const preferenceKeys = {
  currentUser: (userID: string) => ["preferences", "current-user", userID] as const,
};

export async function getCurrentUserPreferences(): Promise<AccountPreferences> {
  return apiClient.request<AccountPreferences>("/api/v1/users/me/preferences");
}

export async function putCurrentUserPreferences(input: AccountPreferences): Promise<AccountPreferences> {
  return apiClient.request<AccountPreferences>("/api/v1/users/me/preferences", {
    method: "PUT",
    body: input,
  });
}

export const currentUserPreferencesQueryOptions = (userID: string) =>
  queryOptions({
    queryKey: preferenceKeys.currentUser(userID),
    queryFn: getCurrentUserPreferences,
    enabled: userID.length > 0,
    retry: false,
  });

export function useCurrentUserPreferencesQuery(userID: string) {
  return useQuery(currentUserPreferencesQueryOptions(userID));
}

export function usePutCurrentUserPreferencesMutation() {
  return useMutation({
    mutationFn: putCurrentUserPreferences,
  });
}
