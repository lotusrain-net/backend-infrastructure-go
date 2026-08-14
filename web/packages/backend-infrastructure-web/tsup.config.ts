import { defineConfig } from "tsup";

export default defineConfig({
  entry: {
    index: "src/index.ts",
    "api/index": "src/api/index.ts",
    "theme/index": "src/theme/index.ts",
    "ui/index": "src/ui/index.ts",
    "ui/client": "src/ui/client.tsx",
    "patterns/index": "src/patterns/index.tsx",
  },
  format: ["esm"],
  target: "es2022",
  dts: true,
  splitting: true,
  sourcemap: true,
  clean: true,
  external: ["react", "react-dom", "react/jsx-runtime"],
});
