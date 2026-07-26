import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PermissionGate } from "@/components/providers/permission-gate";
import { setCurrentUser } from "@/stores/auth-store";

const replace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

describe("PermissionGate", () => {
  beforeEach(() => {
    replace.mockReset();
  });

  it("redirects a signed-in user without the required permission to the forbidden route", async () => {
    setCurrentUser({
      id: "u1",
      email: "operator@example.com",
      username: "operator",
      display_name: "运维人员",
      is_active: true,
      permissions: ["users:read"],
    });

    render(
      <PermissionGate permission="tasks:read">
        <p>任务内容</p>
      </PermissionGate>,
    );

    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith("/403");
    });
    expect(screen.queryByText("任务内容")).toBeNull();
  });

  it("renders protected children when the user has the required permission", () => {
    setCurrentUser({
      id: "u1",
      email: "operator@example.com",
      username: "operator",
      display_name: "运维人员",
      is_active: true,
      permissions: ["tasks:read"],
    });

    render(
      <PermissionGate permission="tasks:read">
        <p>任务内容</p>
      </PermissionGate>,
    );

    expect(screen.getByText("任务内容")).toBeTruthy();
  });
});
