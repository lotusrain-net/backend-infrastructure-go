import { keepPreviousData, queryOptions, useQuery } from "@tanstack/react-query";
import { apiClient, withQuery } from "@/lib/api/client";
import type { ListUsersParams, PaginatedResponse, Permission, Role, User } from "@/types/api";

export const iamKeys = {
  users: (params: Required<Pick<ListUsersParams, "page" | "size">> & Omit<ListUsersParams, "page" | "size">) =>
    ["iam", "users", params] as const,
  roles: () => ["iam", "roles"] as const,
  permissions: () => ["iam", "permissions"] as const,
};

function normalizeUsersParams(params: ListUsersParams = {}) {
  return {
    page: params.page ?? 1,
    size: params.size ?? 20,
    query: params.query?.trim() || undefined,
    is_active: params.is_active,
  };
}

export async function listUsers(params: ListUsersParams = {}): Promise<PaginatedResponse<User>> {
  const normalized = normalizeUsersParams(params);
  return apiClient.request<PaginatedResponse<User>>(withQuery("/api/v1/users", normalized));
}

export const usersQueryOptions = (params: ListUsersParams = {}) => {
  const normalized = normalizeUsersParams(params);
  return queryOptions({
    queryKey: iamKeys.users(normalized),
    queryFn: () => listUsers(normalized),
    placeholderData: keepPreviousData,
  });
};

export function useUsersQuery(params: ListUsersParams = {}) {
  return useQuery(usersQueryOptions(params));
}

export async function listRoles(): Promise<Role[]> {
  return apiClient.request<Role[]>("/api/v1/roles");
}

export async function listPermissions(): Promise<Permission[]> {
  return apiClient.request<Permission[]>("/api/v1/permissions");
}

export const rolesQueryOptions = () =>
  queryOptions({
    queryKey: iamKeys.roles(),
    queryFn: listRoles,
  });

export const permissionsQueryOptions = () =>
  queryOptions({
    queryKey: iamKeys.permissions(),
    queryFn: listPermissions,
  });

export function useRolesQuery() {
  return useQuery(rolesQueryOptions());
}

export function usePermissionsQuery() {
  return useQuery(permissionsQueryOptions());
}
