import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuditConsole, auditEventsToCSV } from "@/features/audit/audit-console";

const useAuditLogsQuery = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/audit",
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams("page=2&size=10&actor_id=actor-1&action=task.submit&result=failure&resource_type=task_execution&resource_id=execution-1&from=2026-07-25T00%3A00%3A00.000Z&to=2026-07-26T00%3A00%3A00.000Z"),
}));

vi.mock("@/components/layout/page-header", () => ({
  PageHeader: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

vi.mock("@/components/providers/permission-gate", () => ({
  PermissionGate: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock("@/features/audit/api", () => ({
  useAuditLogsQuery: (...args: unknown[]) => useAuditLogsQuery(...args),
}));

describe("AuditConsole", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAuditLogsQuery.mockReturnValue({
      data: {
        items: [{
          id: "audit-1",
          request_id: "request-1",
          actor_id: "actor-1",
          action: "task.submit",
          result: "failure",
          resource_type: "task_execution",
          resource_id: "execution-1",
          ip_address: "203.0.113.4",
          user_agent: "console-test",
          metadata: { retry: 2, reason: "timeout" },
          created_at: "2026-07-25T12:00:00.000Z",
        }],
        meta: { page: 2, size: 10, total: 11, pages: 2, has_next: false, has_prev: true },
      },
      isPending: false,
      isError: false,
      error: null,
    });
  });

  it("loads every documented audit filter from the URL and exposes event metadata in details", async () => {
    const user = userEvent.setup();
    render(<AuditConsole />);

    expect(useAuditLogsQuery).toHaveBeenCalledWith({
      page: 2,
      size: 10,
      request_id: undefined,
      actor_id: "actor-1",
      action: "task.submit",
      result: "failure",
      resource_type: "task_execution",
      resource_id: "execution-1",
      from: "2026-07-25T00:00:00.000Z",
      to: "2026-07-26T00:00:00.000Z",
    });
    expect(screen.getByRole("combobox", { name: "结果" }).textContent).toContain("失败");
    expect(screen.getByRole("button", { name: "首页" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "末页" })).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "查看审计事件详情" }));

    expect(await screen.findByRole("dialog", { name: "审计事件详情" })).toBeTruthy();
    expect(screen.getByText((_, element) => element?.tagName === "PRE" && element.textContent?.includes('"reason": "timeout"') === true)).toBeTruthy();
  });

  it("exports only the loaded rows with CSV escaping and spreadsheet formula neutralization", () => {
    const csv = auditEventsToCSV([{
      id: "audit-1",
      request_id: "=SUM(A1:A2)",
      action: "line\nbreak",
      result: "success",
      resource_type: "task,execution",
      resource_id: "quoted \"value\"",
      metadata: { note: "@danger" },
      created_at: "2026-07-25T12:00:00.000Z",
    }]);

    expect(csv).toContain("'=SUM(A1:A2)");
    expect(csv).toContain('"line\nbreak"');
    expect(csv).toContain('"task,execution"');
    expect(csv).toContain('"quoted ""value"""');
    expect(csv).toContain('"{\"\"note\"\":\"\"@danger\"\"}"');
  });
});
