import { keepPreviousData, queryOptions, useMutation, useQuery } from "@tanstack/react-query";
import { apiClient, withQuery } from "@/lib/api/client";
import { queryClient } from "@/lib/query/client";
import type {
  CreateUserRequest,
  ListUsersParams,
  PaginatedResponse,
  Permission,
  Role,
  RoleDetail,
  RoleRequest,
  UpdateUserRequest,
  User,
} from "@/types/api";

export const iamKeys = {
  users: (params: Required<Pick<ListUsersParams, "page" | "size">> & Omit<ListUsersParams, "page" | "size">) =>
    ["iam", "users", params] as const,
  roles: () => ["iam", "roles"] as const,
  role: (roleID: string) => ["iam", "roles", roleID] as const,
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

export async function createUser(input: CreateUserRequest): Promise<User> {
  return apiClient.request<User>("/api/v1/users", { method: "POST", body: input });
}

export async function updateUser(userID: string, input: UpdateUserRequest): Promise<User> {
  return apiClient.request<User>(`/api/v1/users/${encodeURIComponent(userID)}`, { method: "PATCH", body: input });
}

export async function setUserActive(userID: string, isActive: boolean): Promise<Record<string, boolean>> {
  return apiClient.request<Record<string, boolean>>(`/api/v1/users/${encodeURIComponent(userID)}/active`, {
    method: "PATCH",
    body: { is_active: isActive },
  });
}

export async function resetUserPassword(userID: string, newPassword: string): Promise<Record<string, boolean>> {
  return apiClient.request<Record<string, boolean>>(`/api/v1/users/${encodeURIComponent(userID)}/password/reset`, {
    method: "POST",
    body: { new_password: newPassword },
  });
}

export async function replaceUserRoles(userID: string, roleIDs: string[]): Promise<Record<string, boolean>> {
  return apiClient.request<Record<string, boolean>>(`/api/v1/users/${encodeURIComponent(userID)}/roles`, {
    method: "PUT",
    body: { role_ids: roleIDs },
  });
}

export async function getRole(roleID: string): Promise<RoleDetail> {
  return apiClient.request<RoleDetail>(`/api/v1/roles/${encodeURIComponent(roleID)}`);
}

export async function createRole(input: RoleRequest): Promise<Role> {
  return apiClient.request<Role>("/api/v1/roles", { method: "POST", body: input });
}

export async function updateRole(roleID: string, input: RoleRequest): Promise<Role> {
  return apiClient.request<Role>(`/api/v1/roles/${encodeURIComponent(roleID)}`, { method: "PATCH", body: input });
}

export async function deleteRole(roleID: string): Promise<Record<string, boolean>> {
  return apiClient.request<Record<string, boolean>>(`/api/v1/roles/${encodeURIComponent(roleID)}`, { method: "DELETE" });
}

export async function replaceRolePermissions(roleID: string, permissionIDs: string[]): Promise<Record<string, boolean>> {
  return apiClient.request<Record<string, boolean>>(`/api/v1/roles/${encodeURIComponent(roleID)}/permissions`, {
    method: "PUT",
    body: { permission_ids: permissionIDs },
  });
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

export const roleQueryOptions = (roleID: string) =>
  queryOptions({
    queryKey: iamKeys.role(roleID),
    queryFn: () => getRole(roleID),
    enabled: Boolean(roleID),
  });

export function useRoleQuery(roleID: string) {
  return useQuery(roleQueryOptions(roleID));
}

function invalidateUsers() {
  return queryClient.invalidateQueries({ queryKey: ["iam", "users"] });
}

function invalidateRoles(roleID?: string) {
  void queryClient.invalidateQueries({ queryKey: iamKeys.roles() });
  if (roleID) {
    return queryClient.invalidateQueries({ queryKey: iamKeys.role(roleID) });
  }
  return Promise.resolve();
}

export function useCreateUserMutation() {
  return useMutation({ mutationFn: createUser, onSuccess: invalidateUsers });
}

export function useUpdateUserMutation() {
  return useMutation({
    mutationFn: ({ userID, input }: { userID: string; input: UpdateUserRequest }) => updateUser(userID, input),
    onSuccess: invalidateUsers,
  });
}

export function useSetUserActiveMutation() {
  return useMutation({
    mutationFn: ({ userID, isActive }: { userID: string; isActive: boolean }) => setUserActive(userID, isActive),
    onSuccess: invalidateUsers,
  });
}

export function useResetUserPasswordMutation() {
  return useMutation({ mutationFn: ({ userID, newPassword }: { userID: string; newPassword: string }) => resetUserPassword(userID, newPassword) });
}

export function useReplaceUserRolesMutation() {
  return useMutation({
    mutationFn: ({ userID, roleIDs }: { userID: string; roleIDs: string[] }) => replaceUserRoles(userID, roleIDs),
    onSuccess: invalidateUsers,
  });
}

export function useCreateRoleMutation() {
  return useMutation({ mutationFn: createRole, onSuccess: () => invalidateRoles() });
}

export function useUpdateRoleMutation() {
  return useMutation({
    mutationFn: ({ roleID, input }: { roleID: string; input: RoleRequest }) => updateRole(roleID, input),
    onSuccess: (_, variables) => invalidateRoles(variables.roleID),
  });
}

export function useDeleteRoleMutation() {
  return useMutation({ mutationFn: deleteRole, onSuccess: (_, roleID) => invalidateRoles(roleID) });
}

export function useReplaceRolePermissionsMutation() {
  return useMutation({
    mutationFn: ({ roleID, permissionIDs }: { roleID: string; permissionIDs: string[] }) => replaceRolePermissions(roleID, permissionIDs),
    onSuccess: (_, variables) => invalidateRoles(variables.roleID),
  });
}
