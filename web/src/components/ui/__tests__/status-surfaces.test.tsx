import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";

describe("semantic status surfaces", () => {
  it("uses subtle foreground tokens instead of solid-action foreground tokens", () => {
    render(
      <>
        <Badge variant="danger">失败</Badge>
        <Badge variant="success">成功</Badge>
        <Alert variant="destructive">无法保存</Alert>
      </>,
    );

    expect(screen.getByText("失败").className).toContain("--destructive-subtle-foreground");
    expect(screen.getByText("成功").className).toContain("--success-subtle-foreground");
    expect(screen.getByRole("alert").className).toContain("--destructive-subtle-foreground");
  });
});
