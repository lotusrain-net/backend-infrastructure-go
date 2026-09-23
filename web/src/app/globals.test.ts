import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const source = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), "globals.css"),
  "utf8",
);

describe("application stylesheet contract", () => {
  it("declares the Tailwind cascade before application base styles", () => {
    const order = "@layer theme, base, components, utilities;";
    expect(source).toContain(order);
    expect(source.indexOf(order)).toBeLessThan(source.indexOf("@layer base"));
  });
});
