"use client";

import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { Copy, Eye, KeyRound, Pencil, Plus, Power, Save, ShieldCheck, Trash2, UserRoundCog } from "lucide-react";
import { ConfirmationDialog } from "@/components/patterns/confirmation-dialog";
import { DataTable } from "@/components/patterns/data-table";
import { ErrorState } from "@/components/patterns/error-state";
import { FilterBar } from "@/components/patterns/filter-bar";
import { LoadingState } from "@/components/patterns/loading-state";
import { ManagementDialog } from "@/components/patterns/management-dialog";
import { useTableQueryState, type TableQueryCodec } from "@/components/patterns/use-table-query-state";
import { PageHeader } from "@/components/layout/page-header";
import { PermissionGate } from "@/components/providers/permission-gate";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { DialogFooter } from "@/components/ui/dialog";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "@/components/ui/toaster";
import {
  useCreateRoleMutation,
  useCreateUserMutation,
  useDeleteRoleMutation,
  usePermissionsQuery,
  useReplaceRolePermissionsMutation,
  useReplaceUserRolesMutation,
  useResetUserPasswordMutation,
  useRoleQuery,
  useRolesQuery,
  useSetUserActiveMutation,
  useUpdateRoleMutation,
  useUpdateUserMutation,
  useUsersQuery,
} from "@/features/iam/api";
import type { CreateUserRequest, Permission, Role, RoleDetail, RoleRequest, UpdateUserRequest, User } from "@/types/api";

const emptyPage = { page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false };

interface UserTableState {
  page: number;
  size: number;
  query: string;
  is_active: "" | "true" | "false";
}

interface UserFormValues {
  email: string;
  username: string;
  display_name: string;
  password: string;
}

interface PasswordResetValues {
  password: string;
}

interface RoleAssignmentValues {
  role_ids: string[];
}

interface RoleFormValues {
  name: string;
  description: string;
}

interface PermissionAssignmentValues {
  permission_ids: string[];
}

const usersTableCodec: TableQueryCodec<UserTableState> = {
  defaultState: { page: 1, size: 20, query: "", is_active: "" },
  keys: ["page", "size", "query", "is_active"],
  resetPageOnChangeKeys: ["size", "query", "is_active"],
  parse(params) {
    return {
      page: positiveInteger(params.get("page"), 1),
      size: positiveInteger(params.get("size"), 20),
      query: params.get("query") ?? "",
      is_active: parseActive(params.get("is_active")),
    };
  },
  serialize(state) {
    return {
      page: state.page === 1 ? undefined : String(state.page),
      size: state.size === 20 ? undefined : String(state.size),
      query: state.query.trim() || undefined,
      is_active: state.is_active || undefined,
    };
  },
};

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "请求失败，请稍后重试。";
}

function positiveInteger(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function parseActive(value: string | null): UserTableState["is_active"] {
  return value === "true" || value === "false" ? value : "";
}

export function UsersConsole() {
  const { state, setState, reset } = useTableQueryState(usersTableCodec);
  const [userForm, setUserForm] = useState<{ mode: "create" | "edit"; user?: User } | null>(null);
  const [passwordUser, setPasswordUser] = useState<User | null>(null);
  const [rolesUser, setRolesUser] = useState<User | null>(null);
  const [activeUser, setActiveUser] = useState<User | null>(null);
  const usersQuery = useUsersQuery({
    page: state.page,
    size: state.size,
    query: state.query,
    is_active: state.is_active === "" ? undefined : state.is_active === "true",
  });
  const rolesQuery = useRolesQuery();
  const createUser = useCreateUserMutation();
  const updateUser = useUpdateUserMutation();
  const resetPassword = useResetUserPasswordMutation();
  const replaceRoles = useReplaceUserRolesMutation();
  const setActive = useSetUserActiveMutation();

  async function saveUser(input: CreateUserRequest | UpdateUserRequest) {
    try {
      if (userForm?.mode === "edit" && userForm.user) {
        await updateUser.mutateAsync({ userID: userForm.user.id, input: input as UpdateUserRequest });
        toast.success("用户资料已更新。");
      } else {
        await createUser.mutateAsync(input as CreateUserRequest);
        toast.success("用户已创建。");
      }
      setUserForm(null);
    } catch (error) {
      toast.error(errorMessage(error));
      throw error;
    }
  }

  async function savePassword(newPassword: string) {
    if (!passwordUser) return;
    try {
      await resetPassword.mutateAsync({ userID: passwordUser.id, newPassword });
      setPasswordUser(null);
      toast.success("密码已重置，目标用户的会话已撤销。");
    } catch (error) {
      toast.error(errorMessage(error));
      throw error;
    }
  }

  async function saveRoles(roleIDs: string[]) {
    if (!rolesUser) return;
    try {
      await replaceRoles.mutateAsync({ userID: rolesUser.id, roleIDs });
      setRolesUser(null);
      toast.success("用户角色已更新。");
    } catch (error) {
      toast.error(errorMessage(error));
      throw error;
    }
  }

  async function changeActive() {
    if (!activeUser) return;
    try {
      await setActive.mutateAsync({ userID: activeUser.id, isActive: !activeUser.is_active });
      toast.success(activeUser.is_active ? "用户已停用。" : "用户已启用。");
    } catch (error) {
      toast.error(errorMessage(error));
      throw error;
    }
  }

  return (
    <PermissionGate permission="users:read">
      <div className="space-y-6">
        <PageHeader
          title="用户"
          description="创建、维护账号资料，并分配系统角色。"
          actions={
            <PermissionGate permission="users:write">
              <Button onClick={() => setUserForm({ mode: "create" })}>
                <Plus aria-hidden="true" className="size-4" />
                创建用户
              </Button>
            </PermissionGate>
          }
        />
        <FilterBar
          keyword={state.query}
          pageSize={state.size}
          keywordPlaceholder="按邮箱、用户名或显示名筛选"
          onKeywordChange={(query) => setState({ query })}
          onPageSizeChange={(size) => setState({ size })}
          onReset={reset}
        >
          <div className="grid gap-1.5">
            <Label htmlFor="users-active-filter">账号状态</Label>
            <Select
              value={state.is_active || "all"}
              onValueChange={(value) => setState({ is_active: value === "all" ? "" : value as UserTableState["is_active"] })}
            >
              <SelectTrigger id="users-active-filter" className="w-full sm:w-28">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部</SelectItem>
                <SelectItem value="true">启用</SelectItem>
                <SelectItem value="false">停用</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </FilterBar>
        <DataTable<User>
          columns={[
            {
              key: "identity",
              label: "用户",
              render: (user) => (
                <div>
                  <p className="font-medium">{user.display_name || user.username}</p>
                  <p className="text-[length:var(--text-caption)] text-[color:var(--muted-foreground)]">{user.username}</p>
                </div>
              ),
            },
            { key: "email", label: "邮箱", render: (user) => user.email },
            { key: "roles", label: "角色", render: (user) => <RoleLabels roles={user.roles ?? []} /> },
            { key: "active", label: "状态", render: (user) => <Badge variant={user.is_active ? "success" : "neutral"}>{user.is_active ? "启用" : "停用"}</Badge> },
            { key: "created", label: "创建时间", render: (user) => formatDate(user.created_at) },
            {
              key: "actions",
              label: "操作",
              className: "w-[12rem]",
              render: (user) => (
                <PermissionGate permission="users:write">
                  <div className="flex items-center gap-1">
                    <IconAction label={`编辑用户 ${displayUserName(user)}`} icon={<Pencil aria-hidden="true" className="size-4" />} onClick={() => setUserForm({ mode: "edit", user })} />
                    <IconAction label={`${user.is_active ? "停用" : "启用"}用户 ${displayUserName(user)}`} icon={<Power aria-hidden="true" className="size-4" />} onClick={() => setActiveUser(user)} />
                    <IconAction label={`重置 ${displayUserName(user)} 的密码`} icon={<KeyRound aria-hidden="true" className="size-4" />} onClick={() => setPasswordUser(user)} />
                    <IconAction label={`分配 ${displayUserName(user)} 的角色`} icon={<UserRoundCog aria-hidden="true" className="size-4" />} onClick={() => setRolesUser(user)} />
                  </div>
                </PermissionGate>
              ),
            },
          ]}
          rows={usersQuery.data?.items ?? []}
          rowKey={(user) => user.id}
          page={usersQuery.data?.meta ?? emptyPage}
          isLoading={usersQuery.isPending}
          error={usersQuery.isError ? errorMessage(usersQuery.error) : null}
          emptyTitle="暂无用户"
          emptyDescription="当前筛选条件没有匹配的账号。"
          onPageChange={(page) => setState({ page })}
        />
      </div>
      {userForm ? <UserFormDialog form={userForm} onOpenChange={(open) => !open && setUserForm(null)} onSubmit={saveUser} pending={createUser.isPending || updateUser.isPending} /> : null}
      {passwordUser ? <PasswordResetDialog user={passwordUser} onOpenChange={(open) => !open && setPasswordUser(null)} onSubmit={savePassword} pending={resetPassword.isPending} /> : null}
      {rolesUser ? <RoleAssignmentsDialog user={rolesUser} roles={rolesQuery.data ?? []} onOpenChange={(open) => !open && setRolesUser(null)} onSubmit={saveRoles} pending={replaceRoles.isPending} /> : null}
      <ConfirmationDialog
        open={Boolean(activeUser)}
        onOpenChange={(open) => !open && setActiveUser(null)}
        title={activeUser?.is_active ? "停用用户" : "启用用户"}
        description={activeUser ? `确认要${activeUser.is_active ? "停用" : "启用"} ${displayUserName(activeUser)} 吗？` : ""}
        onConfirm={changeActive}
        confirmLabel={activeUser?.is_active ? "停用" : "启用"}
        confirmVariant={activeUser?.is_active ? "destructive" : "default"}
        pending={setActive.isPending}
      />
    </PermissionGate>
  );
}

export function RolesConsole() {
  const rolesQuery = useRolesQuery();
  const permissionsQuery = usePermissionsQuery();
  const [roleForm, setRoleForm] = useState<{ mode: "create" | "edit"; role?: Role } | null>(null);
  const [roleID, setRoleID] = useState<string | null>(null);
  const [permissionsRole, setPermissionsRole] = useState<Role | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Role | null>(null);
  const roleDetailQuery = useRoleQuery(roleID ?? "");
  const permissionsDetailQuery = useRoleQuery(permissionsRole?.id ?? "");
  const createRole = useCreateRoleMutation();
  const updateRole = useUpdateRoleMutation();
  const deleteRole = useDeleteRoleMutation();
  const replacePermissions = useReplaceRolePermissionsMutation();
  const roles = rolesQuery.data ?? [];

  async function saveRole(input: RoleRequest) {
    try {
      if (roleForm?.mode === "edit" && roleForm.role) {
        await updateRole.mutateAsync({ roleID: roleForm.role.id, input });
        toast.success("角色已更新。");
      } else {
        await createRole.mutateAsync(input);
        toast.success("角色已创建。");
      }
      setRoleForm(null);
    } catch (error) {
      toast.error(errorMessage(error));
      throw error;
    }
  }

  async function savePermissions(permissionIDs: string[]) {
    if (!permissionsRole) return;
    try {
      await replacePermissions.mutateAsync({ roleID: permissionsRole.id, permissionIDs });
      setPermissionsRole(null);
      toast.success("角色权限已更新。");
    } catch (error) {
      toast.error(errorMessage(error));
      throw error;
    }
  }

  async function removeRole() {
    if (!deleteCandidate) return;
    try {
      await deleteRole.mutateAsync(deleteCandidate.id);
      toast.success("角色已删除。");
    } catch (error) {
      toast.error(errorMessage(error));
      throw error;
    }
  }

  return (
    <PermissionGate permission="roles:read">
      <div className="space-y-6">
        <PageHeader
          title="角色"
          description="维护自定义角色及其权限。系统角色的名称、权限和删除受到保护。"
          actions={
            <PermissionGate permission="roles:write">
              <Button onClick={() => setRoleForm({ mode: "create" })}>
                <Plus aria-hidden="true" className="size-4" />
                创建角色
              </Button>
            </PermissionGate>
          }
        />
        <DataTable<Role>
          columns={[
            { key: "name", label: "角色名", render: (role) => <div className="flex items-center gap-2"><code>{role.name}</code>{role.is_system ? <Badge variant="info">系统角色</Badge> : null}</div> },
            { key: "description", label: "说明", render: (role) => role.description || "-" },
            {
              key: "actions",
              label: "操作",
              className: "w-[9rem]",
              render: (role) => (
                <div className="flex items-center gap-1">
                  <IconAction label={`查看角色 ${role.name}`} icon={<Eye aria-hidden="true" className="size-4" />} onClick={() => setRoleID(role.id)} />
                  <PermissionGate permission="roles:write">
                    <IconAction label={`编辑角色 ${role.name}`} icon={<Pencil aria-hidden="true" className="size-4" />} onClick={() => setRoleForm({ mode: "edit", role })} />
                    {!role.is_system ? <IconAction label={`删除角色 ${role.name}`} icon={<Trash2 aria-hidden="true" className="size-4" />} onClick={() => setDeleteCandidate(role)} /> : null}
                  </PermissionGate>
                </div>
              ),
            },
          ]}
          rows={roles}
          rowKey={(role) => role.id}
          page={{ ...emptyPage, total: roles.length, pages: roles.length ? 1 : 0 }}
          isLoading={rolesQuery.isPending}
          error={rolesQuery.isError ? errorMessage(rolesQuery.error) : null}
          emptyTitle="暂无角色"
          emptyDescription="创建自定义角色后即可分配给用户。"
        />
      </div>
      {roleForm ? <RoleFormDialog form={roleForm} onOpenChange={(open) => !open && setRoleForm(null)} onSubmit={saveRole} pending={createRole.isPending || updateRole.isPending} /> : null}
      {roleID ? <RoleDetailDialog role={roleDetailQuery.data} loading={roleDetailQuery.isPending} onOpenChange={(open) => !open && setRoleID(null)} onAssignPermissions={(role) => { setRoleID(null); setPermissionsRole(role); }} /> : null}
      {permissionsRole ? <RolePermissionsDialog role={permissionsRole} detail={permissionsDetailQuery.data} permissions={permissionsQuery.data ?? []} onOpenChange={(open) => !open && setPermissionsRole(null)} onSubmit={savePermissions} pending={replacePermissions.isPending} /> : null}
      <ConfirmationDialog
        open={Boolean(deleteCandidate)}
        onOpenChange={(open) => !open && setDeleteCandidate(null)}
        title="删除角色"
        description={deleteCandidate ? `确认删除自定义角色 ${deleteCandidate.name} 吗？已分配的用户将失去该角色。` : ""}
        onConfirm={removeRole}
        confirmLabel="删除"
        pending={deleteRole.isPending}
      />
    </PermissionGate>
  );
}

export function PermissionsConsole() {
  const query = usePermissionsQuery();
  const [search, setSearch] = useState("");
  const permissions = useMemo(() => {
    const value = search.trim().toLowerCase();
    if (!value) return query.data ?? [];
    return (query.data ?? []).filter((permission) => `${permission.name} ${permission.description}`.toLowerCase().includes(value));
  }, [query.data, search]);

  async function copyPermission(name: string) {
    try {
      await copyToClipboard(name);
      toast.success(`已复制 ${name}。`);
    } catch {
      toast.error("无法复制权限码，请手动复制。");
    }
  }

  return (
    <PermissionGate permission="roles:read">
      <div className="space-y-6">
        <PageHeader title="权限" description="权限目录由服务端维护，只读提供给角色分配使用。" />
        <Card className="flex flex-wrap items-end gap-3 p-4 shadow-none sm:p-4">
          <div className="grid min-w-[15rem] flex-1 gap-1.5">
            <Label htmlFor="permissions-search">搜索权限</Label>
            <Input id="permissions-search" type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="按权限码或说明筛选" />
          </div>
        </Card>
        <DataTable<Permission>
          columns={[
            { key: "name", label: "权限码", render: (permission) => <code>{permission.name}</code> },
            { key: "description", label: "说明", render: (permission) => permission.description || "-" },
            { key: "copy", label: "复制", className: "w-[4rem]", render: (permission) => <IconAction label={`复制权限 ${permission.name}`} icon={<Copy aria-hidden="true" className="size-4" />} onClick={() => { void copyPermission(permission.name); }} /> },
          ]}
          rows={permissions}
          rowKey={(permission) => permission.id}
          page={{ ...emptyPage, total: permissions.length, pages: permissions.length ? 1 : 0 }}
          isLoading={query.isPending}
          error={query.isError ? errorMessage(query.error) : null}
          emptyTitle="暂无权限"
          emptyDescription="当前筛选条件没有匹配的权限码。"
        />
      </div>
    </PermissionGate>
  );
}

function UserFormDialog({ form, onOpenChange, onSubmit, pending }: { form: { mode: "create" | "edit"; user?: User }; onOpenChange: (open: boolean) => void; onSubmit: (input: CreateUserRequest | UpdateUserRequest) => Promise<void>; pending: boolean }) {
  const isCreate = form.mode === "create";
  const methods = useForm<UserFormValues>({
    defaultValues: {
      email: form.user?.email ?? "",
      username: form.user?.username ?? "",
      display_name: form.user?.display_name ?? "",
      password: "",
    },
  });

  async function submit(values: UserFormValues) {
    try {
      await onSubmit(isCreate
        ? { email: values.email, username: values.username, display_name: values.display_name, password: values.password }
        : { email: values.email, username: values.username, display_name: values.display_name });
    } catch (reason) {
      methods.setError("root", { message: errorMessage(reason) });
    }
  }

  return (
    <ManagementDialog open onOpenChange={onOpenChange} title={isCreate ? "创建用户" : "编辑用户"} description="账户资料会立即生效。">
      <Form {...methods}>
        <form className="space-y-4" noValidate onSubmit={methods.handleSubmit(submit)}>
          <FormField control={methods.control} name="email" rules={{ required: "请输入邮箱" }} render={({ field }) => (
            <FormItem><FormLabel>邮箱</FormLabel><FormControl><Input type="email" autoComplete="email" {...field} /></FormControl><FormMessage /></FormItem>
          )} />
          <FormField control={methods.control} name="username" rules={{ required: "请输入用户名" }} render={({ field }) => (
            <FormItem><FormLabel>用户名</FormLabel><FormControl><Input autoComplete="username" {...field} /></FormControl><FormMessage /></FormItem>
          )} />
          <FormField control={methods.control} name="display_name" render={({ field }) => (
            <FormItem><FormLabel>显示名称</FormLabel><FormControl><Input {...field} /></FormControl><FormMessage /></FormItem>
          )} />
          {isCreate ? <FormField control={methods.control} name="password" rules={{ required: "请输入初始密码", minLength: { value: 12, message: "密码至少需要 12 个字符" } }} render={({ field }) => (
            <FormItem><FormLabel>初始密码</FormLabel><FormControl><Input type="password" minLength={12} autoComplete="new-password" {...field} /></FormControl><FormMessage /></FormItem>
          )} /> : null}
          <FormFailure message={methods.formState.errors.root?.message} />
          <DialogActions><Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button><Button type="submit" disabled={pending}><Save aria-hidden="true" className="size-4" />{isCreate ? "创建" : "保存"}</Button></DialogActions>
        </form>
      </Form>
    </ManagementDialog>
  );
}

function PasswordResetDialog({ user, onOpenChange, onSubmit, pending }: { user: User; onOpenChange: (open: boolean) => void; onSubmit: (newPassword: string) => Promise<void>; pending: boolean }) {
  const methods = useForm<PasswordResetValues>({ defaultValues: { password: "" } });

  async function submit(values: PasswordResetValues) {
    try {
      await onSubmit(values.password);
    } catch (reason) {
      methods.setError("root", { message: errorMessage(reason) });
    }
  }

  return (
    <ManagementDialog open onOpenChange={onOpenChange} title="重置密码" description={`为 ${displayUserName(user)} 设置新密码，现有会话将被撤销。`}>
      <Form {...methods}>
        <form className="space-y-4" noValidate onSubmit={methods.handleSubmit(submit)}>
          <FormField control={methods.control} name="password" rules={{ required: "请输入新密码", minLength: { value: 12, message: "密码至少需要 12 个字符" } }} render={({ field }) => (
            <FormItem><FormLabel>新密码</FormLabel><FormControl><Input type="password" minLength={12} autoComplete="new-password" {...field} /></FormControl><FormMessage /></FormItem>
          )} />
          <FormFailure message={methods.formState.errors.root?.message} />
          <DialogActions><Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button><Button type="submit" disabled={pending}><KeyRound aria-hidden="true" className="size-4" />重置密码</Button></DialogActions>
        </form>
      </Form>
    </ManagementDialog>
  );
}

function RoleAssignmentsDialog({ user, roles, onOpenChange, onSubmit, pending }: { user: User; roles: Role[]; onOpenChange: (open: boolean) => void; onSubmit: (roleIDs: string[]) => Promise<void>; pending: boolean }) {
  const methods = useForm<RoleAssignmentValues>({ defaultValues: { role_ids: (user.roles ?? []).map((role) => role.id) } });

  async function submit(values: RoleAssignmentValues) {
    try {
      await onSubmit(values.role_ids);
    } catch (reason) {
      methods.setError("root", { message: errorMessage(reason) });
    }
  }

  return (
    <ManagementDialog open onOpenChange={onOpenChange} title="分配角色" description={`更新 ${displayUserName(user)} 的完整角色集合。`}>
      <Form {...methods}>
        <form className="space-y-4" onSubmit={methods.handleSubmit(submit)}>
          <FormField control={methods.control} name="role_ids" render={({ field }) => (
            <FormItem>
              <Label>角色</Label>
              <div className="space-y-2">
                {roles.map((role) => {
                  const controlID = `role-${role.id}`;
                  const selected = field.value.includes(role.id);
                  return (
                    <div key={role.id} className="flex items-start gap-3 border border-[color:var(--border)] p-3 text-[length:var(--text-body-sm)] [border-radius:var(--radius-md)]">
                      <Checkbox id={controlID} checked={selected} onCheckedChange={(checked) => field.onChange(updateSelection(field.value, role.id, checked === true))} />
                      <Label htmlFor={controlID} className="grid cursor-pointer gap-1 leading-normal">
                        <span className="flex items-center gap-2 font-medium"><code>{role.name}</code>{role.is_system ? <Badge variant="info">系统角色</Badge> : null}</span>
                        <span className="text-[color:var(--muted-foreground)]">{role.description}</span>
                      </Label>
                    </div>
                  );
                })}
              </div>
              <FormMessage />
            </FormItem>
          )} />
          <FormFailure message={methods.formState.errors.root?.message} />
          <DialogActions><Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button><Button type="submit" disabled={pending}><Save aria-hidden="true" className="size-4" />保存角色</Button></DialogActions>
        </form>
      </Form>
    </ManagementDialog>
  );
}

function RoleFormDialog({ form, onOpenChange, onSubmit, pending }: { form: { mode: "create" | "edit"; role?: Role }; onOpenChange: (open: boolean) => void; onSubmit: (input: RoleRequest) => Promise<void>; pending: boolean }) {
  const isCreate = form.mode === "create";
  const isSystem = Boolean(form.role?.is_system);
  const methods = useForm<RoleFormValues>({ defaultValues: { name: form.role?.name ?? "", description: form.role?.description ?? "" } });

  async function submit(values: RoleFormValues) {
    try {
      await onSubmit(values);
    } catch (reason) {
      methods.setError("root", { message: errorMessage(reason) });
    }
  }

  return (
    <ManagementDialog open onOpenChange={onOpenChange} title={isCreate ? "创建角色" : "编辑角色"} description={isSystem ? "系统角色仅允许更新说明。" : "名称和说明会用于角色分配。"}>
      <Form {...methods}>
        <form className="space-y-4" noValidate onSubmit={methods.handleSubmit(submit)}>
          <FormField control={methods.control} name="name" rules={{ required: "请输入角色名" }} render={({ field }) => (
            <FormItem><FormLabel>角色名</FormLabel><FormControl><Input disabled={isSystem} {...field} /></FormControl><FormMessage /></FormItem>
          )} />
          <FormField control={methods.control} name="description" render={({ field }) => (
            <FormItem><FormLabel>说明</FormLabel><FormControl><Input {...field} /></FormControl><FormMessage /></FormItem>
          )} />
          <FormFailure message={methods.formState.errors.root?.message} />
          <DialogActions><Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button><Button type="submit" disabled={pending}><Save aria-hidden="true" className="size-4" />{isCreate ? "创建" : "保存"}</Button></DialogActions>
        </form>
      </Form>
    </ManagementDialog>
  );
}

function RoleDetailDialog({ role, loading, onOpenChange, onAssignPermissions }: { role?: RoleDetail; loading: boolean; onOpenChange: (open: boolean) => void; onAssignPermissions: (role: Role) => void }) {
  return (
    <ManagementDialog open onOpenChange={onOpenChange} title="角色详情" description="查看当前角色的权限集合。">
      {loading ? <LoadingState label="正在加载角色详情..." /> : role ? (
        <div className="space-y-4">
          <div className="flex items-center gap-2"><code className="text-[length:var(--text-heading)] font-semibold">{role.name}</code>{role.is_system ? <Badge variant="info">系统角色</Badge> : null}</div>
          <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">{role.description || "未提供说明。"}</p>
          <section className="space-y-2">
            <h3 className="text-[length:var(--text-label)] font-medium">权限</h3>
            {role.permissions.length ? <ul className="space-y-1">{role.permissions.map((permission) => <li key={permission.id} className="border border-[color:var(--border)] px-3 py-2 text-[length:var(--text-body-sm)] [border-radius:var(--radius-sm)]"><code>{permission.name}</code><span className="ml-2 text-[color:var(--muted-foreground)]">{permission.description}</span></li>)}</ul> : <p className="text-[length:var(--text-body-sm)] text-[color:var(--muted-foreground)]">暂无权限。</p>}
          </section>
          {!role.is_system ? <div className="flex justify-end"><Button onClick={() => onAssignPermissions(role)}><ShieldCheck aria-hidden="true" className="size-4" />分配权限</Button></div> : null}
        </div>
      ) : <ErrorState message="无法加载角色详情。" />}
    </ManagementDialog>
  );
}

function RolePermissionsDialog({ role, detail, permissions, onOpenChange, onSubmit, pending }: { role: Role; detail?: RoleDetail; permissions: Permission[]; onOpenChange: (open: boolean) => void; onSubmit: (permissionIDs: string[]) => Promise<void>; pending: boolean }) {
  const methods = useForm<PermissionAssignmentValues>({ defaultValues: { permission_ids: [] } });
  const [hasEdited, setHasEdited] = useState(false);

  useEffect(() => {
    if (detail?.id === role.id && !hasEdited) {
      methods.reset({ permission_ids: detail.permissions.map((permission) => permission.id) });
    }
  }, [detail, hasEdited, methods, role.id]);

  async function submit(values: PermissionAssignmentValues) {
    try {
      await onSubmit(values.permission_ids);
    } catch (reason) {
      methods.setError("root", { message: errorMessage(reason) });
    }
  }

  return (
    <ManagementDialog open onOpenChange={onOpenChange} title="分配权限" description={`替换 ${role.name} 的完整权限集合。`}>
      <Form {...methods}>
        <form className="space-y-4" onSubmit={methods.handleSubmit(submit)}>
          {detail ? <FormField control={methods.control} name="permission_ids" render={({ field }) => (
            <FormItem>
              <Label>权限</Label>
              <div className="space-y-2">
                {permissions.map((permission) => {
                  const controlID = `permission-${permission.id}`;
                  const selected = field.value.includes(permission.id);
                  return (
                    <div key={permission.id} className="flex items-start gap-3 border border-[color:var(--border)] p-3 text-[length:var(--text-body-sm)] [border-radius:var(--radius-md)]">
                      <Checkbox id={controlID} checked={selected} onCheckedChange={(checked) => { setHasEdited(true); field.onChange(updateSelection(field.value, permission.id, checked === true)); }} />
                      <Label htmlFor={controlID} className="grid cursor-pointer gap-1 leading-normal"><code className="font-medium">{permission.name}</code><span className="text-[color:var(--muted-foreground)]">{permission.description}</span></Label>
                    </div>
                  );
                })}
              </div>
              <FormMessage />
            </FormItem>
          )} /> : <LoadingState label="正在加载权限..." />}
          <FormFailure message={methods.formState.errors.root?.message} />
          <DialogActions><Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button><Button type="submit" disabled={pending || !detail}><Save aria-hidden="true" className="size-4" />保存权限</Button></DialogActions>
        </form>
      </Form>
    </ManagementDialog>
  );
}

function DialogActions({ children }: { children: React.ReactNode }) {
  return <DialogFooter className="-mx-5 -mb-5 mt-5">{children}</DialogFooter>;
}

function FormFailure({ message }: { message?: string }) {
  return message ? <Alert variant="destructive"><AlertDescription>{message}</AlertDescription></Alert> : null;
}

function updateSelection(current: string[], value: string, checked: boolean) {
  return checked ? Array.from(new Set([...current, value])) : current.filter((item) => item !== value);
}

function IconAction({ label, icon, onClick }: { label: string; icon: React.ReactNode; onClick: () => void }) {
  return <Button type="button" variant="ghost" size="icon" aria-label={label} title={label} onClick={onClick}>{icon}</Button>;
}

function RoleLabels({ roles }: { roles: Role[] }) {
  return roles.length ? <div className="flex flex-wrap gap-1">{roles.map((role) => <Badge key={role.id} variant={role.is_system ? "info" : "neutral"}>{role.name}</Badge>)}</div> : <span className="text-[color:var(--muted-foreground)]">-</span>;
}

function displayUserName(user: User) {
  return user.display_name || user.username;
}

function formatDate(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(date);
}

async function copyToClipboard(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const element = document.createElement("textarea");
  element.value = value;
  element.style.position = "fixed";
  document.body.appendChild(element);
  element.select();
  const copied = document.execCommand("copy");
  element.remove();
  if (!copied) throw new Error("copy failed");
}
