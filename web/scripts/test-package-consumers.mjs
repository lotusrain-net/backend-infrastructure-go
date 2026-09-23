import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { mkdir } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const packageDirectory = join(
  webRoot,
  "packages",
  "backend-infrastructure-web",
);
const workspace = mkdtempSync(
  join(process.env.TMPDIR ?? tmpdir(), "backend-infrastructure-web-consumers-"),
);

run(
  "npm",
  ["run", "build", "--workspace", "@lotusrain-net/backend-infrastructure-web"],
  webRoot,
);
const packOutput = run(
  "npm",
  ["pack", "--json", "--ignore-scripts", "--pack-destination", workspace],
  packageDirectory,
);
const [{ filename }] = JSON.parse(packOutput);
const tarball = join(workspace, filename);

await verifyViteConsumer(tarball);
await verifyNextConsumer(tarball);

async function verifyViteConsumer(packageTarball) {
  const directory = join(workspace, "vite-consumer");
  await mkdir(join(directory, "src"), { recursive: true });
  writeJSON(join(directory, "package.json"), {
    private: true,
    type: "module",
    scripts: { build: "tsc --noEmit && vite build" },
    dependencies: {
      "@lotusrain-net/backend-infrastructure-web": `file:${packageTarball}`,
      "@vitejs/plugin-react": "5.2.0",
      react: "18.3.1",
      "react-dom": "18.3.1",
      vite: "7.3.6",
    },
    devDependencies: {
      "@types/react": "18.3.27",
      "@types/react-dom": "18.3.7",
      typescript: "5.9.2",
    },
  });
  writeJSON(join(directory, "tsconfig.json"), {
    compilerOptions: {
      target: "ES2022",
      lib: ["DOM", "DOM.Iterable", "ES2022"],
      module: "ESNext",
      moduleResolution: "Bundler",
      jsx: "react-jsx",
      strict: true,
      skipLibCheck: false,
      noEmit: true,
    },
    include: ["src"],
  });
  writeFileSync(
    join(directory, "index.html"),
    '<div id="root"></div><script type="module" src="/src/main.tsx"></script>\n',
  );
  writeFileSync(
    join(directory, "src", "main.tsx"),
    `
import React from "react";
import { createRoot } from "react-dom/client";
import { ApiClient, deriveThemeTokens } from "@lotusrain-net/backend-infrastructure-web";
import { withQuery } from "@lotusrain-net/backend-infrastructure-web/api";
import { Button, Card, cn } from "@lotusrain-net/backend-infrastructure-web/ui";
import { Checkbox, Tabs, TabsContent, TabsList, TabsTrigger } from "@lotusrain-net/backend-infrastructure-web/ui/client";
import { ErrorState, PaginationControls } from "@lotusrain-net/backend-infrastructure-web/patterns";
import "@lotusrain-net/backend-infrastructure-web/styles.css";

const client = new ApiClient({ fetch: globalThis.fetch });
const tokens = deriveThemeTokens("#147D6B", "enterprise", "light");
void client;
void tokens;
void withQuery("/records", { page: 1 });

function App() {
  return <Card className={cn("fixture")}><Button>Save</Button><Checkbox aria-label="Select" /><Tabs defaultValue="one"><TabsList><TabsTrigger value="one">One</TabsTrigger></TabsList><TabsContent value="one"><ErrorState message="Fixture" /></TabsContent></Tabs><PaginationControls page={{ page: 1, size: 20, total: 1, pages: 1, has_next: false, has_prev: false }} onPageChange={() => {}} /></Card>;
}

createRoot(document.getElementById("root")!).render(<App />);
`,
  );
  run(
    "npm",
    ["install", "--ignore-scripts", "--no-audit", "--no-fund"],
    directory,
  );
  run("npm", ["run", "build"], directory);
}

async function verifyNextConsumer(packageTarball) {
  const directory = join(workspace, "next-consumer");
  await mkdir(join(directory, "app"), { recursive: true });
  writeJSON(join(directory, "package.json"), {
    private: true,
    scripts: { build: "next build" },
    dependencies: {
      "@lotusrain-net/backend-infrastructure-web": `file:${packageTarball}`,
      next: "16.3.0",
      react: "19.1.1",
      "react-dom": "19.1.1",
    },
    devDependencies: {
      "@types/node": "24.4.0",
      "@types/react": "19.1.13",
      "@types/react-dom": "19.1.9",
      typescript: "5.9.2",
    },
  });
  writeFileSync(
    join(directory, "next.config.mjs"),
    "export default { transpilePackages: ['@lotusrain-net/backend-infrastructure-web'] };\n",
  );
  writeFileSync(
    join(directory, "app", "layout.tsx"),
    `
import type { ReactNode } from "react";
import "@lotusrain-net/backend-infrastructure-web/styles.css";
export default function Layout({ children }: { children: ReactNode }) { return <html lang="en"><body>{children}</body></html>; }
`,
  );
  writeFileSync(
    join(directory, "app", "client.tsx"),
    `
"use client";
import { useState } from "react";
import { Button } from "@lotusrain-net/backend-infrastructure-web/ui";
import { Checkbox } from "@lotusrain-net/backend-infrastructure-web/ui/client";
import { ConfirmationDialog } from "@lotusrain-net/backend-infrastructure-web/patterns";
export function ClientBoundary() { const [open, setOpen] = useState(false); return <><Button onClick={() => setOpen(true)}>Open</Button><Checkbox aria-label="Select" /><ConfirmationDialog open={open} onOpenChange={setOpen} onConfirm={() => {}} /></>; }
`,
  );
  writeFileSync(
    join(directory, "app", "page.tsx"),
    `
import { deriveThemeTokens } from "@lotusrain-net/backend-infrastructure-web/theme";
import { Alert, Card } from "@lotusrain-net/backend-infrastructure-web/ui";
import { ClientBoundary } from "./client";
export default function Page() { const tokens = deriveThemeTokens("#147D6B", "enterprise", "light"); return <main data-primary={tokens.primary}><Card><Alert>SSR fixture</Alert><ClientBoundary /></Card></main>; }
`,
  );
  run(
    "npm",
    ["install", "--ignore-scripts", "--no-audit", "--no-fund"],
    directory,
  );
  run("npm", ["run", "build"], directory);
}

function writeJSON(path, value) {
  writeFileSync(path, `${JSON.stringify(value, null, 2)}\n`);
}

function run(command, args, cwd) {
  return execFileSync(command, args, {
    cwd,
    encoding: "utf8",
    env: { ...process.env, CI: "1", NEXT_TELEMETRY_DISABLED: "1" },
    stdio: ["ignore", "pipe", "inherit"],
  });
}
