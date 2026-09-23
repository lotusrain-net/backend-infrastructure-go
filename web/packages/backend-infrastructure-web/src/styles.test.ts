import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const source = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), "styles.css"),
  "utf8",
);

describe("package stylesheet contract", () => {
  it("defines the layout tokens required by precompiled utilities", () => {
    expect(source).toContain("@layer theme, base, components, utilities;");
    expect(source).toContain("@theme inline");
    expect(source).toContain("--spacing: 0.25rem");
    expect(source).toContain("--breakpoint-sm: 40rem");
    expect(source).toContain("--breakpoint-lg: 64rem");
  });
});
