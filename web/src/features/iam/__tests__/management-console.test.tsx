import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PermissionsConsole, RolesConsole, UsersConsole } from "@/features/iam/console";

const hooks = vi.hoisted(() => ({
  usersQuery: vi.fn(),
  rolesQuery: vi.fn(),
  permissionsQuery: vi.fn(),
  roleQuery: vi.fn(),
  replace: vi.fn(),
  search: "",
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/iam/users",
  useRouter: () => ({ replace: hooks.replace }),
  useSearchParams: () => new URLSearchParams(hooks.search),
}));

vi.mock("@/components/providers/permission-gate", () => ({
  PermissionGate: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock("@/features/iam/api", () => ({
  useUsersQuery: (...args: unknown[]) => hooks.usersQuery(...args),
  useRolesQuery: () => hooks.rolesQuery(),
  usePermissionsQuery: () => hooks.permissionsQuery(),
  useRoleQuery: (...args: unknown[]) => hooks.roleQuery(...args),
  useCreateUserMutation: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useUpdateUserMutation: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useSetUserActiveMutation: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useResetUserPasswordMutation: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useReplaceUserRolesMutation: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useCreateRoleMutation: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useUpdateRoleMutation: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useDeleteRoleMutation: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useReplaceRolePermissionsMutation: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

const page = { page: 1, size: 20, total: 1, pages: 1, has_next: false, has_prev: false };

describe("IAM management consoles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    hooks.search = "";
    hooks.usersQuery.mockReturnValue({
      data: {
        items: [{ id: "u1", email: "ops@example.com", username: "ops", display_name: "Operations", is_active: true, roles: [{ id: "r2", name: "operator", description: "", is_system: false }] }],
        meta: page,
      },
      isPending: false,
      isError: false,
    });
    hooks.rolesQuery.mockReturnValue({ data: [{ id: "r1", name: "admin", description: "System administrator", is_system: true }, { id: "r2", name: "operator", description: "Operations", is_system: false }], isPending: false, isError: false });
    hooks.permissionsQuery.mockReturnValue({ data: [{ id: "p1", name: "users:read", description: "Read users" }], isPending: false, isError: false });
    hooks.roleQuery.mockReturnValue({ data: undefined, isPending: false, isError: false });
  });

  it("offers all user management operations and displays assigned role labels", () => {
    render(<UsersConsole />);

    expect(screen.getByRole("button", { name: "创建用户" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "编辑用户 Operations" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "停用用户 Operations" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "重置 Operations 的密码" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "分配 Operations 的角色" })).toBeTruthy();
    expect(screen.getByText("operator")).toBeTruthy();
    expect(screen.getByRole("button", { name: "首页" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "末页" })).toBeTruthy();
  });

  it("opens shared form controls and checkbox assignments for user management", async () => {
    const user = userEvent.setup();
    render(<UsersConsole />);

    await user.click(screen.getByRole("button", { name: "创建用户" }));
    expect(screen.getByRole("dialog", { name: "创建用户" })).toBeTruthy();
    expect(screen.getByLabelText("邮箱")).toBeTruthy();
    expect(screen.getByLabelText("初始密码")).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "关闭" }));
    await user.click(screen.getByRole("button", { name: "分配 Operations 的角色" }));
    expect(screen.getByRole("checkbox", { name: /operator/i }).getAttribute("aria-checked")).toBe("true");
  });

  it("keeps user filters in the URL and resets pagination after a filter changes", async () => {
    hooks.search = "page=3&query=legacy";
    render(<UsersConsole />);

    fireEvent.change(screen.getByRole("textbox", { name: "关键词" }), { target: { value: "ops" } });

    expect(hooks.replace).toHaveBeenLastCalledWith("/iam/users?query=ops", { scroll: false });
  });

  it("distinguishes protected system roles and supports custom role administration", () => {
    render(<RolesConsole />);

    expect(screen.getByRole("button", { name: "创建角色" })).toBeTruthy();
    expect(screen.getByText("系统角色")).toBeTruthy();
    expect(screen.getByRole("button", { name: "查看角色 operator" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "编辑角色 operator" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "删除角色 operator" })).toBeTruthy();
  });

  it("lets operators search and copy read-only permission codes", () => {
    render(<PermissionsConsole />);

    expect(screen.getByRole("searchbox", { name: "搜索权限" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "复制权限 users:read" })).toBeTruthy();
  });
});
