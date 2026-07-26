"use client";

import { useState } from "react";
import { PageHeader } from "@/components/layout/page-header";
import { PermissionGate } from "@/components/providers/permission-gate";
import { DataTable } from "@/components/patterns/data-table";
import { FilterBar } from "@/components/patterns/filter-bar";
import { Badge } from "@/components/ui/badge";
import { Select } from "@/components/ui/select";
import { usePermissionsQuery, useRolesQuery, useUsersQuery } from "@/features/iam/api";
import type { Permission, Role, User } from "@/types/api";

const emptyPage = { page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false };

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "加载数据失败，请稍后重试。";
}

export function UsersConsole() {
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [keyword, setKeyword] = useState("");
  const [active, setActive] = useState<"" | "true" | "false">("");
  const query = useUsersQuery({
    page,
    size,
    query: keyword,
    is_active: active === "" ? undefined : active === "true",
  });
  const result = query.data;

  return (
    <PermissionGate permission="users:read">
      <div className="space-y-6">
        <PageHeader title="用户" description="查询已注册账号及其当前启用状态。" />
        <FilterBar
          keyword={keyword}
          pageSize={size}
          keywordPlaceholder="按邮箱、用户名或显示名筛选"
          onKeywordChange={(value) => { setKeyword(value); setPage(1); }}
          onPageSizeChange={(value) => { setSize(value); setPage(1); }}
          onReset={() => { setKeyword(""); setActive(""); setPage(1); }}
        >
          <label className="grid gap-1.5">
            <span className="text-xs font-medium text-[color:var(--fg-muted)]">账号状态</span>
            <Select value={active} onChange={(event) => { setActive(event.target.value as "" | "true" | "false"); setPage(1); }}>
              <option value="">全部</option>
              <option value="true">启用</option>
              <option value="false">停用</option>
            </Select>
          </label>
        </FilterBar>
        <DataTable<User>
          columns={[
            { key: "identity", label: "用户", render: (user) => <div><p className="font-medium">{user.display_name || user.username}</p><p className="text-xs text-[color:var(--fg-muted)]">{user.username}</p></div> },
            { key: "email", label: "邮箱", render: (user) => user.email },
            { key: "active", label: "状态", render: (user) => <Badge variant={user.is_active ? "success" : "neutral"}>{user.is_active ? "启用" : "停用"}</Badge> },
            { key: "created", label: "创建时间", render: (user) => formatDate(user.created_at) },
          ]}
          rows={result?.items ?? []}
          rowKey={(user) => user.id}
          page={result?.meta ?? emptyPage}
          isLoading={query.isPending}
          error={query.isError ? errorMessage(query.error) : null}
          emptyTitle="暂无用户"
          emptyDescription="当前筛选条件没有匹配的账号。"
          onPageChange={setPage}
        />
      </div>
    </PermissionGate>
  );
}

export function RolesConsole() {
  const query = useRolesQuery();
  const roles = query.data ?? [];

  return (
    <PermissionGate permission="roles:read">
      <div className="space-y-6">
        <PageHeader title="角色" description="查看系统中已定义的角色及职责说明。" />
        <DataTable<Role>
          columns={[
            { key: "name", label: "角色名", render: (role) => <code className="text-sm">{role.name}</code> },
            { key: "description", label: "说明", render: (role) => role.description || "-" },
          ]}
          rows={roles}
          rowKey={(role) => role.id}
          page={{ ...emptyPage, total: roles.length, pages: roles.length ? 1 : 0 }}
          isLoading={query.isPending}
          error={query.isError ? errorMessage(query.error) : null}
          emptyTitle="暂无角色"
          emptyDescription="角色目录由后端授权配置维护。"
        />
      </div>
    </PermissionGate>
  );
}

export function PermissionsConsole() {
  const query = usePermissionsQuery();
  const permissions = query.data ?? [];

  return (
    <PermissionGate permission="roles:read">
      <div className="space-y-6">
        <PageHeader title="权限" description="查看可授予角色和用户的权限码。" />
        <DataTable<Permission>
          columns={[
            { key: "name", label: "权限码", render: (permission) => <code className="text-sm">{permission.name}</code> },
            { key: "description", label: "说明", render: (permission) => permission.description || "-" },
          ]}
          rows={permissions}
          rowKey={(permission) => permission.id}
          page={{ ...emptyPage, total: permissions.length, pages: permissions.length ? 1 : 0 }}
          isLoading={query.isPending}
          error={query.isError ? errorMessage(query.error) : null}
          emptyTitle="暂无权限"
          emptyDescription="权限目录由后端授权配置维护。"
        />
      </div>
    </PermissionGate>
  );
}

function formatDate(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(date);
}
