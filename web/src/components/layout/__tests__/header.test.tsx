import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Header } from "@/components/layout/header";
import { SidebarProvider } from "@/components/ui/sidebar";
import { useAuthStore } from "@/stores/auth-store";

const replace = vi.fn();
const mutateAsync = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

vi.mock("@/features/auth/api", () => ({
  useLogoutMutation: () => ({ isPending: false, mutateAsync }),
}));

vi.mock("@/components/layout/theme-controls", () => ({
  ThemeControls: () => <div>主题设置</div>,
}));

describe("Header", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mutateAsync.mockResolvedValue(undefined);
  });

  it("keeps the header focused on the brand and account actions", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: {
        id: "user-1",
        email: "operator@example.com",
        username: "operator",
        display_name: "运营",
        is_active: true,
        permissions: ["users:read", "audit:read"],
      },
    });

    render(<SidebarProvider><Header /></SidebarProvider>);

    expect(screen.getByRole("link", { name: "基础设施控制台" })).toBeTruthy();
    expect(screen.queryByRole("navigation", { name: "快捷导航" })).toBeNull();
    expect(screen.queryByRole("link", { name: "概览" })).toBeNull();
    expect(screen.queryByRole("link", { name: "用户" })).toBeNull();
    expect(screen.queryByRole("link", { name: "审计日志" })).toBeNull();
    expect(screen.queryByRole("link", { name: "任务执行" })).toBeNull();
    expect(screen.queryByRole("link", { name: "个人资料" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "打开运营的账户菜单" }));

    expect(screen.getByText("主题设置")).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "个人资料" })).toBeTruthy();
    await user.click(screen.getByRole("menuitem", { name: "退出登录" }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalledOnce());
    expect(replace).toHaveBeenCalledWith("/login");
  });
});
