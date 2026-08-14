import { render, screen } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  PaginationEllipsis,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./client";

beforeAll(() => {
  if (!HTMLElement.prototype.scrollIntoView) {
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      value: vi.fn(),
    });
  }
});

describe("public client UI", () => {
  it("uses popover foreground tokens for portaled surfaces", () => {
    const { unmount } = render(
      <AlertDialog open onOpenChange={vi.fn()}>
        <AlertDialogContent data-testid="alert-dialog-content">
          <AlertDialogTitle>Confirm</AlertDialogTitle>
          <AlertDialogDescription>Continue?</AlertDialogDescription>
        </AlertDialogContent>
      </AlertDialog>,
    );

    expect(screen.getByTestId("alert-dialog-content").className).toContain(
      "text-[color:var(--popover-foreground)]",
    );

    unmount();
    render(
      <Select open value="one" onValueChange={vi.fn()}>
        <SelectTrigger aria-label="Choice">
          <SelectValue />
        </SelectTrigger>
        <SelectContent data-testid="select-content">
          <SelectItem value="one">One</SelectItem>
        </SelectContent>
      </Select>,
    );

    expect(screen.getByRole("combobox", { name: "Choice", hidden: true }).className).toContain(
      "text-[color:var(--foreground)]",
    );
    expect(screen.getByTestId("select-content").className).toContain(
      "text-[color:var(--popover-foreground)]",
    );
    expect(screen.getByRole("option", { name: "One" }).className).toContain(
      "text-[color:var(--popover-foreground)]",
    );
  });

  it("keeps the pagination ellipsis label in the accessibility tree", () => {
    render(<PaginationEllipsis label="More results" />);

    const label = screen.getByText("More results");
    expect(label.closest('[aria-hidden="true"]')).toBeNull();
    expect(label.previousElementSibling?.getAttribute("aria-hidden")).toBe("true");
  });
});
