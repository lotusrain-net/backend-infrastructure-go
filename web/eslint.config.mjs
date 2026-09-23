import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";

export default defineConfig([
  ...nextVitals,
  globalIgnores([
    ".next/**",
    "coverage/**",
    "node_modules_stale_*/**",
    ".node_modules_stale_*/**",
    "**/._*",
    "**/.__*",
  ]),
]);
