import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api/client";
import { AuthGate } from "@/components/providers/auth-gate";

const replace = vi.fn();
const useCurrentUserQuery = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/tasks",
  useRouter: () => ({ replace }),
}));

vi.mock("@/features/auth/api", () => ({
  useCurrentUserQuery: () => useCurrentUserQuery(),
}));

describe("AuthGate", () => {
  beforeEach(() => {
    replace.mockReset();
    useCurrentUserQuery.mockReset();
  });

  it("redirects an expired anonymous session to login with its return path", async () => {
    useCurrentUserQuery.mockReturnValue({
      data: undefined,
      error: new ApiError("登录状态已失效，请重新登录", 401, 401),
      isPending: false,
    });

    render(
      <AuthGate>
        <p>受保护内容</p>
      </AuthGate>,
    );

    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith("/login?next=%2Ftasks");
    });
    expect(screen.queryByText("受保护内容")).toBeNull();
  });

  it("renders children after the current user resolves", () => {
    useCurrentUserQuery.mockReturnValue({
      data: {
        id: "u1",
        email: "operator@example.com",
        username: "operator",
        display_name: "运维人员",
        is_active: true,
        permissions: [],
      },
      error: null,
      isPending: false,
    });

    render(
      <AuthGate>
        <p>受保护内容</p>
      </AuthGate>,
    );

    expect(screen.getByText("受保护内容")).toBeTruthy();
  });

  it("shows a recoverable service error without redirecting for non-auth failures", () => {
    useCurrentUserQuery.mockReturnValue({
      data: undefined,
      error: new ApiError("服务暂时不可用", 503, 503),
      isPending: false,
    });

    render(
      <AuthGate>
        <p>受保护内容</p>
      </AuthGate>,
    );

    const alert = screen.getByRole("alert");
    expect(alert.textContent).toContain("当前无法恢复会话");
    expect(alert.className.includes("border-l-2")).toBe(false);
    expect(replace).not.toHaveBeenCalled();
  });
});
