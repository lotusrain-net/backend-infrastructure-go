"use client";

import { useEffect } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <main className="grid min-h-screen place-items-center bg-[color:var(--canvas)] p-6">
      <section className="max-w-md border border-[color:var(--border-subtle)] bg-[color:var(--surface)] p-6 text-center shadow-[0_12px_30px_var(--shadow-color)]">
        <AlertTriangle aria-hidden="true" className="mx-auto h-8 w-8 text-[color:var(--warning)]" />
        <h1 className="mt-4 text-xl font-semibold text-[color:var(--fg-default)]">页面暂时不可用</h1>
        <p className="mt-2 text-sm text-[color:var(--fg-muted)]">请重试，或稍后返回控制台。</p>
        <Button className="mt-5" onClick={reset}>重试</Button>
      </section>
    </main>
  );
}
