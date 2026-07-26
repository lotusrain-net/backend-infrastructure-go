import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DataTable } from "@/components/patterns/data-table";

describe("DataTable", () => {
  it("renders paginated rows and an accessible empty state", () => {
    render(
      <DataTable
        columns={[{ key: "email", label: "Email", render: (row: { email: string }) => row.email }]}
        rows={[{ email: "operator@example.com" }]}
        rowKey={(row) => row.email}
        page={{ page: 1, size: 20, total: 1, pages: 1, has_next: false, has_prev: false }}
      />,
    );

    expect(screen.getByRole("columnheader", { name: "Email" })).toBeTruthy();
    expect(screen.getByText("operator@example.com")).toBeTruthy();
    expect(screen.getByText("共 1 条记录")).toBeTruthy();
  });

  it("announces an empty result", () => {
    render(
      <DataTable
        columns={[{ key: "email", label: "Email", render: (row: { email: string }) => row.email }]}
        rows={[]}
        rowKey={(row) => row.email}
        page={{ page: 1, size: 20, total: 0, pages: 0, has_next: false, has_prev: false }}
        emptyTitle="No users"
      />,
    );

    expect(screen.getByRole("status").textContent).toContain("No users");
  });
});
