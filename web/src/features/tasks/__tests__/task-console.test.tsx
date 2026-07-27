import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TaskConsole } from "@/features/tasks/task-console";
import { useAuthStore } from "@/stores/auth-store";

const useTaskExecutionsQuery = vi.fn();
const useTaskExecutionQuery = vi.fn();
const useTaskTypesQuery = vi.fn();
const mutateAsync = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/tasks",
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams("page=2&size=10&task_type=system.test&status=failed"),
}));

vi.mock("@/components/layout/page-header", () => ({
  PageHeader: ({ title, description }: { title: string; description: string }) => (
    <div>
      <h1>{title}</h1>
      <p>{description}</p>
    </div>
  ),
}));

vi.mock("@/components/providers/permission-gate", () => ({
  PermissionGate: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock("@/features/tasks/api", () => ({
  useTaskExecutionsQuery: (...args: unknown[]) => useTaskExecutionsQuery(...args),
  useTaskExecutionQuery: (...args: unknown[]) => useTaskExecutionQuery(...args),
  useTaskTypesQuery: () => useTaskTypesQuery(),
  useSubmitTaskMutation: () => ({ isPending: false, mutateAsync }),
}));

describe("TaskConsole", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAuthStore.setState({
      user: {
        id: "u1",
        email: "ops@example.com",
        username: "ops",
        display_name: "Ops",
        is_active: true,
        permissions: ["tasks:read", "tasks:write"],
      },
    });
    useTaskExecutionsQuery.mockReturnValue({
      data: {
        items: [{
          id: "execution-1",
          task_type: "system.test",
          payload: { dry_run: true },
          status: "failed",
          attempt: 2,
          processed_rows: 0,
          error_summary: "timed out",
        }],
        meta: { page: 2, size: 10, total: 1, pages: 2, has_next: false, has_prev: true },
      },
      isPending: false,
      isError: false,
      error: null,
    });
    useTaskExecutionQuery.mockReturnValue({
      data: {
        id: "execution-1",
        task_type: "system.test",
        payload: { dry_run: true },
        status: "failed",
        attempt: 2,
        processed_rows: 0,
        error_summary: "timed out",
      },
      isPending: false,
      isError: false,
      error: null,
    });
    useTaskTypesQuery.mockReturnValue({
      data: [{ task_type: "system.test" }, { task_type: "report.generate" }],
      isPending: false,
      isError: false,
    });
    mutateAsync.mockResolvedValue({ id: "execution-1" });
  });

  it("uses the first available task type when the user has not picked one", async () => {
    const user = userEvent.setup();
    render(<TaskConsole />);

    expect((screen.getByRole("combobox", { name: "任务类型" }) as HTMLSelectElement).value).toBe("system.test");

    await user.click(screen.getByRole("button", { name: "提交任务" }));

    expect(mutateAsync).toHaveBeenCalledWith({ task_type: "system.test", payload: {} });
    expect(await screen.findByText("任务 execution-1 已提交。")).toBeTruthy();
  });

  it("keeps URL-backed filters and opens a detail view through the existing task API", async () => {
    const user = userEvent.setup();
    render(<TaskConsole />);

    expect(useTaskExecutionsQuery).toHaveBeenCalledWith({
      page: 2,
      size: 10,
      task_type: "system.test",
      status: "failed",
    }, true);

    await user.click(screen.getByRole("button", { name: "查看任务执行详情" }));

    expect(useTaskExecutionQuery).toHaveBeenLastCalledWith("execution-1");
    expect(await screen.findByRole("dialog", { name: "任务执行详情" })).toBeTruthy();
    expect(screen.getByText((_, element) => element?.tagName === "PRE" && element.textContent?.includes('"dry_run": true') === true)).toBeTruthy();
  });
});
