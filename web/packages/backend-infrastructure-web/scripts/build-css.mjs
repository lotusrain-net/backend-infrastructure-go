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

await writeFile(destination, result.css);
