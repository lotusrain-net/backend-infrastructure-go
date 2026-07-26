import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "@/components/ui/button";

describe("Button", () => {
  it("keeps semantic disabled behavior for an icon or text command", () => {
    render(
      <Button variant="secondary" disabled>
        保存
      </Button>,
    );

    const button = screen.getByRole("button", { name: "保存" });
    expect((button as HTMLButtonElement).disabled).toBe(true);
  });
});
