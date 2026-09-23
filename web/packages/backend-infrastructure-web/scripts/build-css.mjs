import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import postcss from "postcss";
import tailwindcss from "@tailwindcss/postcss";

const source = resolve("src/styles.css");
const destination = resolve("dist/styles.css");
const input = await readFile(source, "utf8");
const result = await postcss([tailwindcss()]).process(input, { from: source, to: destination });

if (/(^|[,{]\s*)(?:html|body)(?:\s|,|\{|:)/m.test(result.css)) {
  throw new Error("Public package CSS must not contain html/body application styles.");
}
if (/box-sizing\s*:|@import\s+|@tailwind\s+/m.test(result.css)) {
  throw new Error("Public package CSS must be precompiled and must not contain a reset.");
}

// Spacing utilities depend on the package's own --spacing and breakpoint
// tokens. Keep this check close to the precompile step so a published package
// cannot silently regress to unpadded cards, controls, and responsive layouts.
const requiredUtilities = [
  ".p-5",
  ".sm\\:p-6",
  ".gap-2",
  ".h-10",
  ".size-4",
];
for (const utility of requiredUtilities) {
  if (!result.css.includes(utility)) {
    throw new Error(`Public package CSS is missing required utility: ${utility}`);
  }
}

if (!result.css.includes("--spacing: 0.25rem")) {
  throw new Error("Public package CSS is missing its spacing scale.");
}
if (!result.css.includes("@layer theme, base, components, utilities;")) {
  throw new Error("Public package CSS is missing the Tailwind cascade order.");
}

await writeFile(destination, result.css);
