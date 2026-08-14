import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ConfirmationDialog, DataTable, ManagementDialog } from "./index";

describe("public patterns", () => {
  it("keeps confirmation open and reports rejected actions", async () => {
    const error = new Error("request failed");
    const onOpenChange = vi.fn();
    const onConfirmError = vi.fn();

    render(
      <ConfirmationDialog
        open
        onOpenChange={onOpenChange}
        onConfirm={() => Promise.reject(error)}
        onConfirmError={onConfirmError}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));

    await waitFor(() => expect(onConfirmError).toHaveBeenCalledWith(error));
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(screen.getByRole("alertdialog")).toBeTruthy();
  });

  it("accepts a localized management dialog close label", () => {
    render(
      <ManagementDialog
        open
        onOpenChange={vi.fn()}
        title="Settings"
        description="Update settings"
        closeLabel="关闭"
      >
        Content
      </ManagementDialog>,
    );

    expect(screen.getByRole("button", { name: "关闭" })).toBeTruthy();
  });

  it("announces an empty table result", () => {
    render(
      <DataTable
        columns={[{ key: "name", label: "Name", render: (row: { name: string }) => row.name }]}
        rows={[]}
        rowKey={(row) => row.name}
        page={{ page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false }}
      />,
    );

    expect(screen.getByRole("status").textContent).toContain("No records");
  });
});
