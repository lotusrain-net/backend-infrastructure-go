import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Sidebar } from "../sidebar";
import {
  SidebarProvider,
  Sidebar as SidebarPanel,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { useAuthStore } from "@/stores/auth-store";
vi.mock("next/navigation", () => ({
  usePathname: () => "/system-settings/basic-auth",
}));
function setup(permissions: string[]) {
  useAuthStore.setState({
    user: {
      id: "u",
      email: "a@b.com",
      username: "u",
      display_name: "U",
      is_active: true,
      permissions,
    },
  });
  const navigate = vi.fn();
  render(
    <SidebarProvider>
      <Sidebar onNavigate={navigate} />
    </SidebarProvider>,
  );
  return navigate;
}
describe("system settings navigation", () => {
  it("follows profile, collapses and navigates with the mobile callback", async () => {
    const user = userEvent.setup();
    const navigate = setup(["*"]);
    const nav = screen.getByRole("navigation", { name: "主导航" });
    const labels = within(nav)
      .getAllByRole("link")
      .map((a) => a.textContent);
    expect(labels).toEqual([
      "概览",
      "审计日志",
      "任务执行",
      "个人资料",
      "用户",
      "角色",
      "权限",
      "基本身份验证",
    ]);
    const group = screen.getByRole("button", { name: "系统设置" });
    await user.click(group);
    expect(screen.queryByRole("link", { name: "基本身份验证" })).toBeNull();
    await user.click(group);
    const link = screen.getByRole("link", { name: "基本身份验证" });
    link.addEventListener("click", (event) => event.preventDefault());
    await user.click(link);
    expect(navigate).toHaveBeenCalledOnce();
  });
  it("filters children by permissions", () => {
    setup(["system-settings:read"]);
    expect(screen.getByRole("link", { name: "基本身份验证" })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "用户" })).toBeNull();
    expect(screen.queryByRole("link", { name: "角色" })).toBeNull();
  });
});

it("opens the navigation as a mobile sheet and closes it after selecting a permitted child", async () => {
  vi.stubGlobal(
    "matchMedia",
    vi
      .fn()
      .mockReturnValue({
        matches: true,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
  );
  useAuthStore.setState({
    user: {
      id: "u",
      email: "a@b.com",
      username: "u",
      display_name: "U",
      is_active: true,
      permissions: ["system-settings:read"],
    },
  });
  const user = userEvent.setup();
  render(
    <SidebarProvider>
      <SidebarTrigger />
      <SidebarPanel>
        <Sidebar />
      </SidebarPanel>
    </SidebarProvider>,
  );
  await user.click(screen.getByRole("button", { name: "打开导航" }));
  const dialog = await screen.findByRole("dialog", { name: "主导航" });
  const link = within(dialog).getByRole("link", { name: "基本身份验证" });
  link.addEventListener("click", (event) => event.preventDefault());
  expect(within(dialog).queryByRole("link", { name: "用户" })).toBeNull();
  await user.click(link);
  await waitFor(() =>
    expect(screen.queryByRole("dialog", { name: "主导航" })).toBeNull(),
  );
});
